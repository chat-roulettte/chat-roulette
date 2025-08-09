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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
)

const (
	blockMemberTemplateFilename = "block_member_modal.json.tmpl"
)

// blockMemberTemplate is used with blockMemberTemplateFilename
type blockMemberTemplate struct {
	UserID          string
	ImageURL        string
	PrivateMetadata string
}

// BlockMemberParams are the parameters for the BLOCK_MEMBER job.
type BlockMemberParams struct {
	UserID   string `json:"user_id"`
	MemberID string `json:"member_id"`
}

func (p *BlockMemberParams) Validate() error {
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

// BlockMember blocks the user p.MemberID from being matched with the user p.User.
func BlockMember(ctx context.Context, db *gorm.DB, client *slack.Client, p *BlockMemberParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackUserID, p.UserID,
	)

	// Validate job parameters
	if err := p.Validate(); err != nil {
		logger.Error("failed to validate job parameters", "error", err)
		return models.ErrJobParamsFailedValidation
	}

	// Skip bot users as they cannot participate in chat-roulette
	// This checks for all bot users in the channel and not only the chat-roulette bot
	if isBot, err := isUserASlackBot(ctx, client, p.MemberID); err != nil {
		message := "failed to check if this Slack user is a bot"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	} else if isBot {
		logger.Debug("skipping because this Slack user is a bot")
		return nil
	}

	// Retrieve the channelIDs that this user is a member of
	var channels []string

	dbCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()

	err := db.WithContext(dbCtx).Model(&models.Member{}).Select("channel_id").Where("user_id = ?", p.UserID).Find(&channels).Error
	if err != nil {
		return errors.Wrap(err, "failed to retrieve chat roulette channels for this user")
	}
	if len(channels) == 0 {
		return nil // noop
	}

	for _, channel := range channels {
		l := logger.With(
			attributes.SlackChannelID, channel,
			"blocked_member_id", p.MemberID,
		)

		// Add the blocked Slack member to the database
		l.Info("adding blocked Slack member to the database")

		blockedMember := &models.BlockedMember{
			ChannelID: channel,
			UserID:    p.UserID,
			MemberID:  p.MemberID,
		}

		dbCtx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()

		result := db.WithContext(dbCtx).Create(blockedMember)

		if result.Error != nil {
			// Dont error if the member is already blocked by this user
			var pgErr *pgconn.PgError
			if errors.Is(result.Error, gorm.ErrDuplicatedKey) || (errors.As(result.Error, &pgErr) && pgErr.Code == "23505") {
				l.Debug("Slack member is already blocked by this user")
				return nil
			}

			message := "failed to add new blocked member to the database"
			l.Error(message, "error", result.Error)
			return errors.Wrap(result.Error, message)
		}

		l.Info("successfully blocked Slack member from matching with this user")
	}

	return nil
}

// QueueBlockMemberJob adds a new BLOCK_MEMBER job to the queue.
func QueueBlockMemberJob(ctx context.Context, db *gorm.DB, p *BlockMemberParams) error {
	job := models.GenericJob[*BlockMemberParams]{
		JobType:  models.JobTypeBlockMember,
		Priority: models.JobPriorityHigh,
		Params:   p,
	}

	return QueueJob(ctx, db, job)
}

// HandleBlockMemberButton ...
func HandleBlockMemberButton(ctx context.Context, baseURL string, client *slack.Client, interaction *slack.InteractionCallback) error {
	// Start new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "handle.button.BLOCK_MEMBER")
	defer span.End()

	if interaction.Type == slack.InteractionTypeBlockActions {
		span.SetAttributes(
			attribute.String(attributes.SlackUserID, interaction.User.ID),
			attribute.String(attributes.SlackInteraction, string(interaction.Type)),
			attribute.String(attributes.SlackActionID, string(interaction.ActionCallback.BlockActions[0].Type)),
		)

		// Render the template
		u, err := url.Parse(baseURL)
		if err != nil {
			return errors.Wrap(err, "failed to parse base URL")
		}
		u.Path = path.Join(u.Path, "static/img/do-not-match.png")

		t := blockMemberTemplate{
			UserID:          interaction.User.ID,
			PrivateMetadata: interaction.View.PrivateMetadata,
			ImageURL:        u.String(),
		}

		content, err := renderTemplate(blockMemberTemplateFilename, t)
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
// that a user does not want to be matched with and
// queues BLOCK_MEMBER job for each member.
func UpsertMemberBlockList(ctx context.Context, db *gorm.DB, interaction *slack.InteractionCallback) error {
	// Start new span
	tracer := otel.Tracer("")
	ctx, span := tracer.Start(ctx, "upsert.block_member")
	defer span.End()

	span.SetAttributes(
		attribute.String(attributes.SlackUserID, interaction.User.ID),
		attribute.String(attributes.SlackInteraction, string(interaction.Type)),
	)

	// Extract the list of users from the view state
	users := interaction.View.State.Values["block-members"]["placeholder"].SelectedUsers

	for _, user := range users {
		// Schedule an BLOCK_MEMBER job to ...
		// BlockMember() could be called directly here, however
		// scheduling a background job will ensure it is reliably executed.
		p := &BlockMemberParams{
			UserID:   interaction.User.ID,
			MemberID: user,
		}

		if err := QueueBlockMemberJob(ctx, db, p); err != nil {
			return errors.Wrap(err, "failed to add BLOCK_MEMBER job to the queue")
		}
	}

	return nil
}
