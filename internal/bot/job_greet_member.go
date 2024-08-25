package bot

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/bincyber/go-sqlcrypter"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-playground/tz"
	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"go.opentelemetry.io/otel"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/isx"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
	"github.com/chat-roulettte/chat-roulette/internal/templatex"
	"github.com/chat-roulettte/chat-roulette/internal/tzx"
)

const (
	greetMemberTemplateFilename = "greet_member.json.tmpl"

	onboardingModalTemplateFilename    = "onboarding_modal.json.tmpl"
	onboardingLocationTemplateFilename = "onboarding_location.json.tmpl"
	onboardingTimezoneTemplateFilename = "onboarding_timezone.json.tmpl"
	onboardingGenderTemplateFilename   = "onboarding_gender.json.tmpl"
	onboardingProfileTemplateFilename  = "onboarding_profile.json.tmpl"
	onboardingCalendlyTemplateFilename = "onboarding_calendly.json.tmpl"
)

// greetMemberTemplate is used with templates/greet_member.json.tmpl
type greetMemberTemplate struct {
	ChannelID      string
	Invitor        string
	UserID         string
	NextRound      time.Time
	When           string
	ConnectionMode string
}

type privateMetadata struct {
	ChannelID   string       `json:"channel_id,omitempty"`
	ResponseURL string       `json:"response_url,omitempty"`
	Blocks      slack.Blocks `json:"blocks,omitempty"`
}

// Encode encodes privateMetadata from struct to json to base64
func (p *privateMetadata) Encode() (string, error) {
	var b bytes.Buffer

	encoder := base64.NewEncoder(base64.StdEncoding, &b)
	if err := json.NewEncoder(encoder).Encode(p); err != nil {
		return "", err
	}
	encoder.Close()

	return b.String(), nil
}

// Decode decodes privateMetadata from base64 to json to struct
func (p *privateMetadata) Decode(s string) error {
	decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(s))
	return json.NewDecoder(decoder).Decode(p)
}

// onboardingMemberTemplate is used with templates/onboarding_*.json.tmpl templates
type onboardingMemberTemplate struct {
	UserID          string
	PrivateMetadata string
	ImageURL        string
	Zones           []tz.Zone
}

// GreetMemberParams are the parameters for the GREET_MEMBER job.
type GreetMemberParams struct {
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
}

// GreetMember greets a new member of a Slack channel with a welcome message.
func GreetMember(ctx context.Context, db *gorm.DB, client *slack.Client, p *GreetMemberParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
		attributes.SlackUserID, p.UserID,
	)

	// Retrieve channel metadata from the database
	dbCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	var channel models.Channel

	if err := db.WithContext(dbCtx).Where("channel_id = ?", p.ChannelID).First(&channel).Error; err != nil {
		message := "failed to retrieve metadata for the Slack channel"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	// Render template
	t := greetMemberTemplate{
		ChannelID:      p.ChannelID,
		Invitor:        channel.Inviter,
		UserID:         p.UserID,
		NextRound:      channel.NextRound,
		When:           formatSchedule(channel.Interval, channel.NextRound),
		ConnectionMode: channel.ConnectionMode.String(),
	}

	content, err := renderTemplate(greetMemberTemplateFilename, t)
	if err != nil {
		return errors.Wrap(err, "failed to render template")
	}

	logger.Info("greeting Slack member with an intro message")

	// We can marshal the json template into View as it contains Blocks
	var view slack.View
	if err := json.Unmarshal([]byte(content), &view); err != nil {
		return errors.Wrap(err, "failed to unmarshal JSON")
	}

	// Open a Slack DM with the user
	childCtx, cancel := context.WithTimeout(ctx, 3000*time.Millisecond)
	defer cancel()

	response, _, _, err := client.OpenConversationContext(
		childCtx,
		&slack.OpenConversationParameters{
			ReturnIM: false,
			Users: []string{
				p.UserID,
			},
		})

	if err != nil {
		logger.Error("failed to open Slack DM", "error", err)
		return err
	}

	// Send the Slack direct message to the user
	if _, _, err = client.PostMessageContext(
		ctx,
		response.Conversation.ID,
		slack.MsgOptionBlocks(view.Blocks.BlockSet...),
	); err != nil {
		logger.Error("failed to send Slack direct message", "error", err)
		return err
	}

	return nil
}

// QueueGreetMemberJob adds a new GREET_MEMBER job to the queue.
func QueueGreetMemberJob(ctx context.Context, db *gorm.DB, p *GreetMemberParams) error {
	job := models.GenericJob[*GreetMemberParams]{
		JobType:  models.JobTypeGreetMember,
		Priority: models.JobPriorityStandard,
		Params:   p,
	}

	return QueueJob(ctx, db, job)
}

// HandleGreetMemberButton processes the webhook sent by Slack when a user clicks
// on the button in the GREET_MESSAGE job confirming that they wish to participate
// in chat roulette. A modal is opened to collect onboarding information and upon
// submission, a response is sent to Slack overwriting the button in the original message,
// so that it cannot be clicked multiple times. Since this interaction only contains
// a single button, we do not need to parse the action.
func HandleGreetMemberButton(ctx context.Context, client *slack.Client, interaction *slack.InteractionCallback) error {
	// Start new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "button.GREET_MEMBER")
	defer span.End()

	if interaction.Type == slack.InteractionTypeBlockActions {
		// ChannelID and ResponseURL will be stored in the private_metadata field
		pm := &privateMetadata{
			ChannelID:   interaction.ActionCallback.BlockActions[0].Value,
			ResponseURL: interaction.ResponseURL,
			Blocks:      interaction.Message.Blocks,
		}

		s, err := pm.Encode()
		if err != nil {
			return errors.Wrap(err, "failed to encode privateMetadata to base64")
		}

		// Render the template
		t := onboardingMemberTemplate{
			UserID:          interaction.User.ID,
			PrivateMetadata: s,
		}

		content, err := renderTemplate(onboardingModalTemplateFilename, t)
		if err != nil {
			return errors.Wrap(err, "failed to render template")
		}

		// Marshal the template
		var view slack.ModalViewRequest
		if err := json.Unmarshal([]byte(content), &view); err != nil {
			return errors.Wrap(err, "failed to unmarshal JSON")
		}

		// Use the trigger ID to open the initial view for the modal
		if _, err = client.OpenViewContext(ctx, interaction.TriggerID, view); err != nil {
			return errors.Wrap(err, "failed to push view context")
		}
	}

	return nil
}

// RenderOnboardingLocationView renders the view template for collecting
// a new member's location data.
func RenderOnboardingLocationView(ctx context.Context, interaction *slack.InteractionCallback, baseURL string) ([]byte, error) {
	// Start new span
	tracer := otel.Tracer("")
	_, span := tracer.Start(ctx, "render.location")
	defer span.End()

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse base URL")
	}
	u.Path = path.Join(u.Path, "static/img/globe.jpg")

	// Render the template
	t := onboardingMemberTemplate{
		PrivateMetadata: interaction.View.PrivateMetadata,
		ImageURL:        u.String(),
	}

	content, err := renderTemplate(onboardingLocationTemplateFilename, t)
	if err != nil {
		return nil, errors.Wrap(err, "failed to render template")
	}

	return []byte(content), nil
}

// UpsertMemberLocationInfo collects a new member's location info during
// the onboarding flow and updates it in the database.
func UpsertMemberLocationInfo(ctx context.Context, db *gorm.DB, interaction *slack.InteractionCallback) error {
	// Start new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "upsert.location")
	defer span.End()

	// Extract the ChannelID from the private_metadata field
	var pm privateMetadata
	if err := pm.Decode(interaction.View.PrivateMetadata); err != nil {
		return errors.Wrap(err, "failed to decode base64 string to privateMetadata")
	}

	// Extract the values from the view state
	country := interaction.View.State.Values["onboarding-country"]["onboarding-location-country"].SelectedOption.Value
	city := templatex.Capitalize(interaction.View.State.Values["onboarding-city"]["onboarding-location-city"].Value)

	// Schedule an UPDATE_MEMBER job to update the member's location.
	// UpdateMember() could be called directly here, however
	// scheduling a background job will ensure it is reliably executed.
	p := &UpdateMemberParams{
		UserID:    interaction.User.ID,
		ChannelID: pm.ChannelID,
		Country:   sqlcrypter.NewEncryptedBytes(country),
		City:      sqlcrypter.NewEncryptedBytes(city),
	}

	if err := QueueUpdateMemberJob(ctx, db, p); err != nil {
		return errors.Wrap(err, "failed to add UPDATE_MEMBER job to the queue")
	}

	return nil
}

// RenderOnboardingTimezoneView renders the view template for collecting
// a new member's timezone data.
func RenderOnboardingTimezoneView(ctx context.Context, interaction *slack.InteractionCallback, baseURL string) ([]byte, error) {
	// Start new span
	tracer := otel.Tracer("")
	_, span := tracer.Start(ctx, "render.timezone")
	defer span.End()

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse base URL")
	}
	u.Path = path.Join(u.Path, "static/img/globe.jpg")

	// Lookup the timezones for the specified country
	value := interaction.View.State.Values["onboarding-country"]["onboarding-location-country"].SelectedOption.Value

	country, ok := tzx.GetCountryByName(value)
	if !ok {
		return nil, fmt.Errorf("invalid country provided")
	}

	t := onboardingMemberTemplate{
		PrivateMetadata: interaction.View.PrivateMetadata,
		ImageURL:        u.String(),
		Zones:           country.Zones,
	}

	content, err := renderTemplate(onboardingTimezoneTemplateFilename, t)
	if err != nil {
		return nil, errors.Wrap(err, "failed to render template")
	}

	return []byte(content), nil
}

// UpsertMemberTimezoneInfo collects a new member's timezone info during
// the onboarding flow and updates it in the database.
func UpsertMemberTimezoneInfo(ctx context.Context, db *gorm.DB, interaction *slack.InteractionCallback) error {
	// Start new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "upsert.timezone")
	defer span.End()

	// Extract the ChannelID from the private_metadata field
	var pm privateMetadata
	if err := pm.Decode(interaction.View.PrivateMetadata); err != nil {
		return errors.Wrap(err, "failed to decode base64 string to privateMetadata")
	}

	// Extract the timezone from the view state
	timezone := interaction.View.State.Values["onboarding-timezone"]["onboarding-timezone"].SelectedOption.Value

	// Schedule an UPDATE_MEMBER job to update the member's timezone.
	// UpdateMember() could be called directly here, however
	// scheduling a background job will ensure it is reliably executed.
	p := &UpdateMemberParams{
		UserID:    interaction.User.ID,
		ChannelID: pm.ChannelID,
		Timezone:  sqlcrypter.NewEncryptedBytes(timezone),
	}

	if err := QueueUpdateMemberJob(ctx, db, p); err != nil {
		return errors.Wrap(err, "failed to add UPDATE_MEMBER job to the queue")
	}

	return nil
}

// RenderOnboardingGenderView renders the view template for collecting
// a new member's gender info.
func RenderOnboardingGenderView(ctx context.Context, interaction *slack.InteractionCallback, baseURL string) ([]byte, error) {
	// Start new span
	tracer := otel.Tracer("")
	_, span := tracer.Start(ctx, "render.gender")
	defer span.End()

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse base URL")
	}
	u.Path = path.Join(u.Path, "static/img/social-icons.png")

	// Render the template
	t := onboardingMemberTemplate{
		UserID:          interaction.User.ID,
		PrivateMetadata: interaction.View.PrivateMetadata,
		ImageURL:        u.String(),
	}

	content, err := renderTemplate(onboardingGenderTemplateFilename, t)
	if err != nil {
		return nil, errors.Wrap(err, "failed to render template")
	}

	return []byte(content), nil
}

// UpsertMemberGenderInfo collects a new member's gender info during
// the onboarding flow and updates it in the database.
func UpsertMemberGenderInfo(ctx context.Context, db *gorm.DB, interaction *slack.InteractionCallback) error {
	// Start new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "upsert.gender")
	defer span.End()

	// Extract the ChannelID from the private_metadata field
	var pm privateMetadata
	if err := pm.Decode(interaction.View.PrivateMetadata); err != nil {
		return errors.Wrap(err, "failed to decode base64 string to privateMetadata")
	}

	// Extract the values from the view state
	gender := interaction.View.State.Values["onboarding-gender-select"]["onboarding-gender-select"].SelectedOption.Value

	hasGenderPreference := false
	if len(interaction.View.State.Values["onboarding-gender-checkbox"]["onboarding-gender-checkbox"].SelectedOptions) > 0 {
		hasGenderPreference = true
	}

	// Schedule an UPDATE_MEMBER job to update the member's gender.
	// UpdateMember() could be called directly here, however
	// scheduling a background job will ensure it is reliably executed.
	p := &UpdateMemberParams{
		UserID:              interaction.User.ID,
		ChannelID:           pm.ChannelID,
		Gender:              gender,
		HasGenderPreference: hasGenderPreference,
	}

	if err := QueueUpdateMemberJob(ctx, db, p); err != nil {
		return errors.Wrap(err, "failed to add UPDATE_MEMBER job to the queue")
	}

	return nil
}

// RenderOnboardingProfileView renders the view template for collecting
// a new member's profile info.
func RenderOnboardingProfileView(ctx context.Context, interaction *slack.InteractionCallback, baseURL string) ([]byte, error) {
	// Start new span
	tracer := otel.Tracer("")
	_, span := tracer.Start(ctx, "render.profile")
	defer span.End()

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse base URL")
	}
	u.Path = path.Join(u.Path, "static/img/social-icons.png")

	// Render the template
	t := onboardingMemberTemplate{
		UserID:          interaction.User.ID,
		PrivateMetadata: interaction.View.PrivateMetadata,
		ImageURL:        u.String(),
	}

	content, err := renderTemplate(onboardingProfileTemplateFilename, t)
	if err != nil {
		return nil, errors.Wrap(err, "failed to render template")
	}

	return []byte(content), nil
}

// ValidateMemberProfileInfo validates that the user provided social profile link
// is a valid URL for the supported social profile types.
func ValidateMemberProfileInfo(ctx context.Context, interaction *slack.InteractionCallback) error {
	// Start new span
	tracer := otel.Tracer("")
	_, span := tracer.Start(ctx, "validate.profile")
	defer span.End()

	// Extract the values from the view state
	profileType := strings.ToLower(interaction.View.State.Values["onboarding-profile-type"]["onboarding-profile-type"].SelectedOption.Value)
	profileLink := strings.ToLower(interaction.View.State.Values["onboarding-profile-link"]["onboarding-profile-link"].Value)

	// Validate
	if err := validation.Validate(profileType,
		validation.Required,
		validation.By(isx.ProfileType),
	); err != nil {
		return err
	}

	if err := isx.ValidProfileLink(profileType, profileLink); err != nil {
		return err
	}

	return nil
}

// UpsertMemberProfileInfo collects a new member's profile info during
// the onboarding flow and updates it in the database.
func UpsertMemberProfileInfo(ctx context.Context, db *gorm.DB, interaction *slack.InteractionCallback) error {
	// Start new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "upsert.profile")
	defer span.End()

	// Extract the ChannelID from the private_metadata field
	var pm privateMetadata
	if err := pm.Decode(interaction.View.PrivateMetadata); err != nil {
		return errors.Wrap(err, "failed to decode base64 string to privateMetadata")
	}

	// Extract the values from the view state
	profileType := interaction.View.State.Values["onboarding-profile-type"]["onboarding-profile-type"].SelectedOption.Value
	profileLink := interaction.View.State.Values["onboarding-profile-link"]["onboarding-profile-link"].Value

	// Schedule an UPDATE_MEMBER job to update the member's location.
	// UpdateMember() could be called directly here, however
	// scheduling a background job will ensure it is reliably executed.
	p := &UpdateMemberParams{
		UserID:      interaction.User.ID,
		ChannelID:   pm.ChannelID,
		ProfileType: sqlcrypter.NewEncryptedBytes(profileType),
		ProfileLink: sqlcrypter.NewEncryptedBytes(profileLink),
		IsActive:    true,
	}

	if err := QueueUpdateMemberJob(ctx, db, p); err != nil {
		return errors.Wrap(err, "failed to add UPDATE_MEMBER job to the queue")
	}

	return nil
}

// RenderOnboardingCalendlyView renders the view template for collecting
// a new member's calendly link.
func RenderOnboardingCalendlyView(ctx context.Context, interaction *slack.InteractionCallback, baseURL string) ([]byte, error) {
	// Start new span
	tracer := otel.Tracer("")
	_, span := tracer.Start(ctx, "render.calendly")
	defer span.End()

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse base URL")
	}
	u.Path = path.Join(u.Path, "static/img/calendly.jpg")

	// Render the template
	t := onboardingMemberTemplate{
		UserID:          interaction.User.ID,
		PrivateMetadata: interaction.View.PrivateMetadata,
		ImageURL:        u.String(),
	}

	content, err := renderTemplate(onboardingCalendlyTemplateFilename, t)
	if err != nil {
		return nil, errors.Wrap(err, "failed to render template")
	}

	return []byte(content), nil
}

// ValidateMemberCalendlyLink validates that the user-provided Calendly link
// is a valid.
//
// Note: providing a Calendly link is optional.
func ValidateMemberCalendlyLink(ctx context.Context, link string) error {
	// Start new span
	tracer := otel.Tracer("")
	_, span := tracer.Start(ctx, "validate.calendly")
	defer span.End()

	// Validate the Calendly link if provided
	if link != "" {
		if err := validation.Validate(link, validation.By(isx.CalendlyLink)); err != nil {
			return err
		}
	}

	return nil
}

// UpsertMemberCalendlyLink collects a new member's Calendly link during
// the onboarding flow and updates it in the database.
func UpsertMemberCalendlyLink(ctx context.Context, db *gorm.DB, interaction *slack.InteractionCallback) error {
	// Start new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "upsert.calendly")
	defer span.End()

	// Extract the ChannelID from the private_metadata field
	var pm privateMetadata
	if err := pm.Decode(interaction.View.PrivateMetadata); err != nil {
		return errors.Wrap(err, "failed to decode base64 string to privateMetadata")
	}

	// Extract the Calendly link from the view state
	calendlyLink := strings.ToLower(interaction.View.State.Values["onboarding-calendly"]["onboarding-calendly"].Value)

	// Schedule an UPDATE_MEMBER job to update the member's location.
	// UpdateMember() could be called directly here, however
	// scheduling a background job will ensure it is reliably executed.
	p := &UpdateMemberParams{
		UserID:       interaction.User.ID,
		ChannelID:    pm.ChannelID,
		CalendlyLink: sqlcrypter.NewEncryptedBytes(calendlyLink),
		IsActive:     true,
	}

	if err := QueueUpdateMemberJob(ctx, db, p); err != nil {
		return errors.Wrap(err, "failed to add UPDATE_MEMBER job to the queue")
	}

	return nil
}

// RespondGreetMemberWebhook responds to the Slack webhook received when the
// "Opt In" button in the GREET_MEMBER message is clicked. The original message
// is updated to overwrite the button, so that it cannot be clicked multiple times.
func RespondGreetMemberWebhook(ctx context.Context, client *http.Client, interaction *slack.InteractionCallback) error {
	// Start new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "webhook.GREET_MEMBER")
	defer span.End()

	// Extract the original slack.Message and ResponseURL from the private_metadata field
	var pm privateMetadata
	if err := pm.Decode(interaction.View.PrivateMetadata); err != nil {
		return errors.Wrap(err, "failed to decode base64 string to privateMetadata")
	}

	confirmationText := `*Thank you for choosing to participate in Chat Roulette!*`

	text := slack.NewTextBlockObject("mrkdwn", confirmationText, false, false)
	section := slack.NewSectionBlock(text, nil, nil)

	var message slack.Message
	message.Msg.Blocks = pm.Blocks

	message = transformMessage(message, 6, section)

	slack.AddBlockMessage(message, section)

	webhookMessage := &slack.WebhookMessage{
		Blocks:          &message.Blocks,
		ReplaceOriginal: true,
	}

	// Send HTTP response for the webhook
	if err := slack.PostWebhookCustomHTTPContext(ctx, pm.ResponseURL, client, webhookMessage); err != nil {
		return errors.Wrap(err, "failed to send Slack webhook")
	}

	return nil
}
