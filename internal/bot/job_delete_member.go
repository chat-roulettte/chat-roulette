package bot

import (
	"context"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
)

// DeleteMemberParams are the parameters for the DELETE_MEMBER job.
type DeleteMemberParams struct {
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
}

// DeleteMember deletes a member who has left a Slack channel from the database.
func DeleteMember(ctx context.Context, db *gorm.DB, client *slack.Client, p *DeleteMemberParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
		attributes.SlackUserID, p.UserID,
	)

	logger.Info("deleting Slack user from the database")

	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	result := db.WithContext(dbCtx).
		Where("user_id = ? AND channel_id = ?", p.UserID, p.ChannelID).
		Delete(&models.Member{})

	if result.Error != nil {
		err := result.Error
		message := "failed to delete Slack user from the database"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	return nil
}

// QueueDeleteMemberJob adds a new DELETE_MEMBER job to the queue.
func QueueDeleteMemberJob(ctx context.Context, db *gorm.DB, p *DeleteMemberParams) error {
	job := models.GenericJob[*DeleteMemberParams]{
		JobType:  models.JobTypeDeleteMember,
		Priority: models.JobPriorityHigh,
		Params:   p,
	}

	return QueueJob(ctx, db, job)
}
