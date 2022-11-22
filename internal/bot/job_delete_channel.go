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

// DeleteChannelParams are the parameters for the DELETE_CHANNEL job.
type DeleteChannelParams struct {
	ChannelID string `json:"channel_id"`
}

// DeleteChannel deletes a Slack channel from the database.
func DeleteChannel(ctx context.Context, db *gorm.DB, client *slack.Client, p *DeleteChannelParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
	)

	logger.Info("offboarding Slack channel")

	// Mark all pending jobs for this Slack channel as completed
	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	result := db.WithContext(dbCtx).
		Model(&models.Job{}).
		Where("data->>'channel_id' = ? AND is_completed = false", p.ChannelID).
		Updates(&models.Job{IsCompleted: true, Status: models.JobStatusCanceled})

	if result.Error != nil {
		message := "failed to cancel pending jobs"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	dbCtx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	result = db.WithContext(dbCtx).
		Where("channel_id = ?", p.ChannelID).
		Delete(&models.Channel{})

	if result.Error != nil {
		err := result.Error
		message := "failed to delete Slack channel from the database"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	if result.RowsAffected == 1 {
		logger.Info("deleted Slack channel from the database")
	}

	return nil
}

// DeleteChannelJob adds a new DELETE_CHANNEL job to the queue.
func QueueDeleteChannelJob(ctx context.Context, db *gorm.DB, p *DeleteChannelParams) error {
	job := models.GenericJob[*DeleteChannelParams]{
		JobType:  models.JobTypeDeleteChannel,
		Priority: models.JobPriorityHighest,
		Params:   p,
	}

	return QueueJob(ctx, db, job)
}
