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

// EndRoundParams are the parameters for the END_ROUND job.
type EndRoundParams struct {
	ChannelID string    `json:"channel_id"`
	NextRound time.Time `json:"next_round"`
}

// EndRound concludes a running chat-roulette round for a Slack channel.
func EndRound(ctx context.Context, db *gorm.DB, client *slack.Client, p *EndRoundParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
	)

	logger.Info("ending chat-roulette round if one is in progress")

	// End the current chat-roulette round for this Slack channel
	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	result := db.WithContext(dbCtx).
		Model(&models.Round{}).
		Where("channel_id = ? ", p.ChannelID).
		Where("has_ended = false").
		Update("has_ended", true)

	if result.Error != nil {
		message := "failed to end current chat-roulette round"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	logger.Info("ended the current chat-roulette round")

	return nil
}

// QueueEndRoundJob adds a new END_ROUND job to the queue.
func QueueEndRoundJob(ctx context.Context, db *gorm.DB, p *EndRoundParams) error {
	job := models.GenericJob[*EndRoundParams]{
		JobType:  models.JobTypeEndRound,
		Priority: models.JobPriorityStandard,
		Params:   p,
		ExecAt:   p.NextRound.Add(-(4 * time.Hour)), // 4 hours before the start of the next round
	}

	return QueueJob(ctx, db, job)
}
