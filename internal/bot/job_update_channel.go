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
	"github.com/chat-roulettte/chat-roulette/internal/timex"
)

// UpdateChannelParams are the parameters the UPDATE_CHANNEL job.
type UpdateChannelParams struct {
	ChannelID      string    `json:"channel_id"`
	Interval       string    `json:"interval"`
	ConnectionMode string    `json:"connection_mode"`
	Weekday        string    `json:"weekday"`
	Hour           int       `json:"hour"`
	NextRound      time.Time `json:"next_round"`
}

// UpdateChannel updates the settings for a chat-roulette enabled Slack channel.
func UpdateChannel(ctx context.Context, db *gorm.DB, client *slack.Client, p *UpdateChannelParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
	)

	logger.Info("updating Slack channel")

	// Validate changes
	weekday, err := timex.ParseWeekday(p.Weekday)
	if err != nil {
		logger.Error("failed to parse weekday", "error", err)
		return err
	}

	interval, err := models.ParseInterval(p.Interval)
	if err != nil {
		logger.Error("failed to parse interval", "error", err)
		return err
	}

	connectionMode, err := models.ParseConnectionMode(p.ConnectionMode)
	if err != nil {
		logger.Error("failed to parse connection mode", "error", err)
		return err
	}

	// Update the chat-roulette settings for the Slack channel
	updatedChannel := &models.Channel{
		ChannelID:      p.ChannelID,
		Interval:       interval,
		ConnectionMode: connectionMode,
		Weekday:        weekday,
		Hour:           p.Hour,
		NextRound:      p.NextRound,
	}

	dbCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	result := db.WithContext(dbCtx).
		Model(&models.Channel{}).
		Where("channel_id = ?", p.ChannelID).
		Updates(updatedChannel)

	if result.Error != nil {
		message := "failed to update database row channel"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	logger.Info("updated database row for the channel")

	// Cancel any pending CREATE_ROUND jobs for this Slack channel
	dbCtx, cancel = context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	result = db.WithContext(dbCtx).
		Model(&models.Job{}).
		Where("data->>'channel_id' = ?", p.ChannelID).
		Where("is_completed = false").
		Where("job_type = ?", models.JobTypeCreateRound.String()).
		Updates(&models.Job{IsCompleted: true, Status: models.JobStatusCanceled})

	if result.Error != nil {
		message := "failed to cancel any pending CREATE_ROUND jobs"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	// Queue a new CREATE_ROUND job using the updated channel settings
	createRoundParams := &CreateRoundParams{
		ChannelID: p.ChannelID,
		Interval:  p.Interval,
		NextRound: p.NextRound,
	}

	if err := QueueCreateRoundJob(ctx, db, createRoundParams); err != nil {
		message := "failed to add CREATE_ROUND job to the queue"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	return nil
}

// UpdateChannelJob adds a new UPDATE_CHANNEL job to the queue.
func QueueUpdateChannelJob(ctx context.Context, db *gorm.DB, p *UpdateChannelParams) error {
	job := models.GenericJob[*UpdateChannelParams]{
		JobType:  models.JobTypeUpdateChannel,
		Priority: models.JobPriorityHigh,
		Params:   p,
	}

	return QueueJob(ctx, db, job)
}
