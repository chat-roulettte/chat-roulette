package bot

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
)

const (
	notifyMemberTemplateFilename = "notify_member.json.tmpl"
)

// notifyMemberTemplate is used with templates/notify_member.json.tmpl
type notifyMemberTemplate struct {
	ChannelID string
	UserID    string
	NextRound time.Time
}

// NotifyMemberParams are the parameters for the NOTIFY_MEMBER job.
type NotifyMemberParams struct {
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
}

// NotifyMember sends an apologetic message to a participant for not being able to be matched in this round of chat roulette.
func NotifyMember(ctx context.Context, db *gorm.DB, client *slack.Client, p *NotifyMemberParams) error {

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
	t := notifyMemberTemplate{
		ChannelID: p.ChannelID,
		UserID:    p.UserID,
		NextRound: channel.NextRound,
	}

	content, err := renderTemplate(notifyMemberTemplateFilename, t)
	if err != nil {
		return errors.Wrap(err, "failed to render template")
	}

	logger.Info("notifying Slack member with a message")

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
				p.UserID,
			},
		})

	if err != nil {
		logger.Error("failed to open Slack DM", "error", err)
		return err
	}

	// Send the Slack DM to the user
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

// QueueNotifyMemberJob adds a new NOTIFY_MEMBER job to the queue.
func QueueNotifyMemberJob(ctx context.Context, db *gorm.DB, p *NotifyMemberParams) error {
	job := models.GenericJob[*NotifyMemberParams]{
		JobType:  models.JobTypeNotifyMember,
		Priority: models.JobPriorityStandard,
		Params:   p,
	}

	return QueueJob(ctx, db, job)
}
