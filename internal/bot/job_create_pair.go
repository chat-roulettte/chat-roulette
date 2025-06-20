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

// CreatePairParams are the parameters for the CREATE_PAIR job.
type CreatePairParams struct {
	ChannelID   string `json:"channel_id"`
	MatchID     int32  `json:"match_id"`
	Participant string `json:"participant"`
	Partner     string `json:"partner"`
}

// CreatePair creates a pairing between 2 matched participants.
func CreatePair(ctx context.Context, db *gorm.DB, client *slack.Client, p *CreatePairParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
		attributes.MatchID, p.MatchID,
	)

	logger.Info("creating chat-roulette pair")

	// Look up the member IDs for the pair in the database
	var memberIDs []int32

	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	result := db.WithContext(dbCtx).
		Model(&models.Member{}).
		Select("id").
		Where("channel_id = ?", p.ChannelID).
		Where(
			db.Where("user_id = ?", p.Participant).
				Or("user_id = ?", p.Partner)).
		Find(&memberIDs)

	if result.Error != nil || result.RowsAffected != 2 {
		message := "failed to retrieved member IDs"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	// Create new records in the pairings table for the pair
	for _, id := range memberIDs {
		newPair := &models.Pairing{
			MatchID:  p.MatchID,
			MemberID: id,
		}

		dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()

		if err := db.WithContext(dbCtx).Create(newPair).Error; err != nil {
			message := "failed to add a new pair record to the database"
			logger.Error(message, "error", result.Error)
			return errors.Wrap(result.Error, message)
		}
	}

	logger.Debug("created chat-roulette pair")

	// Queue a NOTIFY_PAIR job
	params := &NotifyPairParams{
		ChannelID:   p.ChannelID,
		MatchID:     p.MatchID,
		Participant: p.Participant,
		Partner:     p.Partner,
	}

	dbCtx, cancel = context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	if err := QueueNotifyPairJob(dbCtx, db, params); err != nil {
		message := "failed to add NOTIFY_PAIR job to the queue"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	logger.Info("queued NOTIFY_PAIR job for this match")

	return nil
}

// QueueCreatePairJob adds a new CREATE_PAIR job to the queue.
func QueueCreatePairJob(ctx context.Context, db *gorm.DB, p *CreatePairParams) error {
	job := models.GenericJob[*CreatePairParams]{
		JobType:  models.JobTypeCreatePair,
		Priority: models.JobPriorityStandard,
		Params:   p,
	}

	return QueueJob(ctx, db, job)
}
