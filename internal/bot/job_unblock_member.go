package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
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
	unblockMemberTemplateFilename = "unblock_member_modal.json.tmpl"
)

// unblockMemberTemplate is used with unblockMemberTemplateFilename
type unblockMemberTemplate struct {
	UserID          string
	ImageURL        string
	PrivateMetadata string
	BlockedMembers  string
}

// UnblockMemberParams are the parameters for the UNBLOCK_MEMBER job.
type UnblockMemberParams struct {
	UserID   string `json:"user_id"`
	MemberID string `json:"member_id"`
}

func (p *UnblockMemberParams) Validate() error {
	return validation.ValidateStruct(p,
		validation.Field(&p.UserID, validation.Required, is.Alphanumeric),
		validation.Field(&p.MemberID, validation.Required, is.Alphanumeric, validation.By(func(value interface{}) error {
			if p.UserID == p.MemberID {
				return fmt.Errorf("user_id cannot be same as member_id")
			}

			return nil
		})),
	)
}

// UnblockMember unblocks the user p.MemberID from not being matched with the user p.User.
func UnblockMember(ctx context.Context, db *gorm.DB, client *slack.Client, p *UnblockMemberParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackUserID, p.UserID,
	)

	// Validate job parameters
	if err := p.Validate(); err != nil {
		logger.Error("failed to validate job parameters", "error", err)
		return models.ErrJobParamsFailedValidation
	}

	// Delete the user from the blocklist
	dbCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	result := db.WithContext(dbCtx).Model(models.BlockedMember{}).Where("user_id = ?", p.UserID).Where("member_id = ?", p.MemberID).Delete(models.BlockedMember{})
	if result.Error != nil {
		message := "failed to unblock member for this user"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	if result.RowsAffected != 1 {
		logger.Debug("no action taken: Slack member was not blocked for this user")
		return nil // noop
	}

	logger.Info("successfully unblocked Slack member for this user")

	return nil
}

// QueueUnblockMemberJob adds a new BLOCK_MEMBER job to the queue.
func QueueUnblockMemberJob(ctx context.Context, db *gorm.DB, p *UnblockMemberParams) error {
	job := models.GenericJob[*UnblockMemberParams]{
		JobType:  models.JobTypeUnblockMember,
		Priority: models.JobPriorityHigh,
		Params:   p,
	}

	return QueueJob(ctx, db, job)
}

// HandleUnblockMemberButton ...
func HandleUnblockMemberButton(ctx context.Context, baseURL string, db *gorm.DB, client *slack.Client, interaction *slack.InteractionCallback) error {
	// Start new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "handle.button.UNBLOCK_MEMBER")
	defer span.End()

	if interaction.Type == slack.InteractionTypeBlockActions {
		span.SetAttributes(
			attribute.String(attributes.SlackUserID, interaction.User.ID),
			attribute.String(attributes.SlackInteraction, string(interaction.Type)),
			attribute.String(attributes.SlackActionID, string(interaction.ActionCallback.BlockActions[0].Type)),
		)

		// Retrieve upto 3 members that were previously blocked by this user
		var blockedMembers []string

		dbCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
		defer cancel()

		err := db.WithContext(dbCtx).
			Model(models.BlockedMember{}).
			Select("member_id").
			Where("user_id = ?", interaction.User.ID).
			Limit(3).
			Find(&blockedMembers).Error
		if err != nil {
			// Ignore any errors and return unpopulated list
			hclog.FromContext(ctx).With(
				attributes.SlackUserID, interaction.User.ID,
				attributes.SlackInteraction, interaction.Type,
			).Warn("failed to retrieve list of blocked members for this user", "error", err)
		}

		// Convert blockedMembers to json
		blockedMembersList := `[]`
		data, err := json.Marshal(blockedMembers)
		if err == nil {
			blockedMembersList = string(data)
		}

		// Render the template
		u, err := url.Parse(baseURL)
		if err != nil {
			return errors.Wrap(err, "failed to parse base URL")
		}
		u.Path = path.Join(u.Path, "static/img/allow-match.png")

		t := unblockMemberTemplate{
			UserID:          interaction.User.ID,
			PrivateMetadata: interaction.View.PrivateMetadata,
			ImageURL:        u.String(),
			BlockedMembers:  blockedMembersList,
		}

		content, err := renderTemplate(unblockMemberTemplateFilename, t)
		if err != nil {
			return errors.Wrap(err, "failed to render template")
		}

		// Marshal the template
		var view slack.ModalViewRequest
		if err := json.Unmarshal([]byte(content), &view); err != nil {
			return errors.Wrap(err, "failed to unmarshal JSON to view")
		}

		// Use the trigger ID to open the view for the modal
		if _, err = client.OpenViewContext(ctx, interaction.TriggerID, view); err != nil {
			return errors.Wrap(err, "failed to push view context")
		}
	}

	return nil
}

// UpsertMemberBlockList collects the list of members
// that a user wishes to unblock  and
// queues UNBLOCK_MEMBER job for each member.
func UpsertMemberUnblockList(ctx context.Context, db *gorm.DB, interaction *slack.InteractionCallback) error {
	// Start new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "upsert.unblock_member")
	defer span.End()

	span.SetAttributes(
		attribute.String(attributes.SlackUserID, interaction.User.ID),
		attribute.String(attributes.SlackInteraction, string(interaction.Type)),
	)

	// Extract the list of users from the view state
	users := interaction.View.State.Values["unblock-members"]["placeholder"].SelectedUsers

	for _, user := range users {
		// Schedule an UNBLOCK_MEMBER job to unblock this member for this user
		// BlockMember() could be called directly here, however
		// scheduling a background job will ensure it is reliably executed.
		p := &UnblockMemberParams{
			UserID:   interaction.User.ID,
			MemberID: user,
		}

		if err := QueueUnblockMemberJob(ctx, db, p); err != nil {
			return errors.Wrap(err, "failed to add BLOCK_MEMBER job to the queue")
		}
	}

	return nil
}
