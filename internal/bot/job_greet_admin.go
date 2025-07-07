package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
)

const (
	greetAdminTemplateFilename = "greet_admin.json.tmpl"

	onboardingModalTemplateFilename   = "onboarding_modal.json.tmpl"
	onboardingChannelTemplateFilename = "onboarding_channel.json.tmpl"
)

// greetAdminTemplate is used with templates/greet_admin.json.tmpl
type greetAdminTemplate struct {
	ChannelID string
	UserID    string
}

// GreetAdminParams are the parameters for the GREET_ADMIN job.
type GreetAdminParams struct {
	ChannelID string `json:"channel_id"`
	Inviter   string `json:"user_id"`
}

// GreetAdmin greets the admin of new Slack channel with a welcome message.
func GreetAdmin(ctx context.Context, db *gorm.DB, client *slack.Client, p *GreetAdminParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
		attributes.SlackUserID, p.Inviter,
	)

	// Render template
	t := greetAdminTemplate{
		ChannelID: p.ChannelID,
		UserID:    p.Inviter,
	}

	content, err := renderTemplate(greetAdminTemplateFilename, t)
	if err != nil {
		return errors.Wrap(err, "failed to render template")
	}

	logger.Info("greeting Slack channel admin with an intro message")

	// We can marshal the json template into View as it contains Blocks
	var view slack.View
	if err := json.Unmarshal([]byte(content), &view); err != nil {
		return errors.Wrap(err, "failed to unmarshal JSON")
	}

	// Open a Slack DM with the user
	childCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	response, _, _, err := client.OpenConversationContext(
		childCtx,
		&slack.OpenConversationParameters{
			ReturnIM: false,
			Users: []string{
				p.Inviter,
			},
		})

	if err != nil {
		logger.Error("failed to open Slack DM", "error", err)
		return err
	}

	// Send the Slack direct message to the user
	if _, _, err = client.PostMessageContext(
		ctx,
		response.ID,
		slack.MsgOptionBlocks(view.Blocks.BlockSet...),
	); err != nil {
		logger.Error("failed to send Slack direct message", "error", err)
		return err
	}

	return nil
}

// QueueGreetAdminJob adds a new GREET_ADMIN job to the queue.
func QueueGreetAdminJob(ctx context.Context, db *gorm.DB, p *GreetAdminParams) error {
	job := models.GenericJob[*GreetAdminParams]{
		JobType:  models.JobTypeGreetAdmin,
		Priority: models.JobPriorityHigh,
		Params:   p,
	}

	return QueueJob(ctx, db, job)
}

// HandleGreetAdminButton processes the webhook sent by Slack when a user clicks
// on the button in the GREET_ADMIN job starting the flow for channel onboarding
// in chat roulette. A modal is opened to collect onboarding information and upon
// submission, a response is sent to Slack overwriting the button in the original message,
// so that it cannot be clicked multiple times. Since this interaction only contains
// a single button, we do not need to parse the action.
func HandleGreetAdminButton(ctx context.Context, client *slack.Client, interaction *slack.InteractionCallback) error {
	// Start new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "handle.button.GREET_ADMIN")
	defer span.End()

	if interaction.Type == slack.InteractionTypeBlockActions {
		span.SetAttributes(
			attribute.String(attributes.SlackInteraction, string(interaction.Type)),
			attribute.String(attributes.SlackAction, string(interaction.ActionCallback.BlockActions[0].Type)),
		)

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
		t := onboardingTemplate{
			UserID:          interaction.User.ID,
			PrivateMetadata: s,
			IsAdmin:         true,
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

// RenderOnboardingChannelView renders the view template for collecting settings
// to enable a new chat-roulette channel.
func RenderOnboardingChannelView(ctx context.Context, interaction *slack.InteractionCallback, baseURL string) ([]byte, error) {
	// Start new span
	tracer := otel.Tracer("")
	_, span := tracer.Start(ctx, "render.channel")
	defer span.End()

	// Extract the channel ID from the private_metadata field
	var pm privateMetadata
	if err := pm.Decode(interaction.View.PrivateMetadata); err != nil {
		return nil, errors.Wrap(err, "failed to decode base64 string to privateMetadata")
	}

	// Render the template
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse base URL")
	}
	u.Path = path.Join(u.Path, "static/img/coffee-machine.jpg")

	t := onboardingTemplate{
		ChannelID:       pm.ChannelID,
		PrivateMetadata: interaction.View.PrivateMetadata,
		ImageURL:        u.String(),
	}

	content, err := renderTemplate(onboardingChannelTemplateFilename, t)
	if err != nil {
		return nil, errors.Wrap(err, "failed to render template")
	}

	return []byte(content), nil
}

// UpsertChannelSettings ...
func UpsertChannelSettings(ctx context.Context, db *gorm.DB, interaction *slack.InteractionCallback) error {
	// Start new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "upsert.channel")
	defer span.End()

	// Extract the ChannelID from the private_metadata field
	var pm privateMetadata
	if err := pm.Decode(interaction.View.PrivateMetadata); err != nil {
		return errors.Wrap(err, "failed to decode base64 string to privateMetadata")
	}

	// Extract the channel settings from the view state
	connectionMode := interaction.View.State.Values["onboarding-channel-connection-mode"]["onboarding-channel-connection-mode"].SelectedOption.Value
	interval := interaction.View.State.Values["onboarding-channel-interval"]["onboarding-channel-interval"].SelectedOption.Value

	datetime := interaction.View.State.Values["onboarding-channel-datetime"]["onboarding-channel-datetime"].SelectedDateTime
	firstRound := time.Unix(datetime, 0).UTC()

	// Schedule an ADD_CHANNEL job to onboard the new Slack channel
	p := &AddChannelParams{
		ChannelID:      pm.ChannelID,
		Inviter:        interaction.User.ID,
		ConnectionMode: connectionMode,
		Interval:       interval,
		Weekday:        firstRound.Weekday().String(),
		Hour:           firstRound.Hour(),
		NextRound:      firstRound,
	}

	if err := QueueAddChannelJob(ctx, db, p); err != nil {
		return errors.Wrap(err, "failed to add ADD_CHANNEL job to the queue")
	}

	return nil
}

// RespondGreetAdminWebhook responds to the Slack webhook received when the
// button in the GREET_ADMIN message is clicked. The original message
// is updated to overwrite the button, so that it cannot be clicked multiple times.
func RespondGreetAdminWebhook(ctx context.Context, client *http.Client, interaction *slack.InteractionCallback) error {
	// Start new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "webhook.GREET_ADMIN")
	defer span.End()

	// Extract the original slack.Message and ResponseURL from the private_metadata field
	var pm privateMetadata
	if err := pm.Decode(interaction.View.PrivateMetadata); err != nil {
		return errors.Wrap(err, "failed to decode base64 string to privateMetadata")
	}

	confirmationText := `*Chat Roulette is now enabled! I hope you enjoy using this app* :grin:`

	text := slack.NewTextBlockObject("mrkdwn", confirmationText, false, false)
	section := slack.NewSectionBlock(text, nil, nil)

	deepLink := generateAppHomeDeepLink(interaction.Team.ID, interaction.APIAppID)

	visitAppHomeText := fmt.Sprintf(":pushpin:  You can always visit me in <%s|App Home>", deepLink)

	element := slack.NewTextBlockObject("mrkdwn", visitAppHomeText, false, false)
	contextBlock := slack.NewContextBlock("AppHome", element)

	var message slack.Message
	message.Blocks = pm.Blocks

	message = transformMessage(message, 5, section, contextBlock)

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
