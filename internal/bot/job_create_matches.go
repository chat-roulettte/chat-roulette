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

// CreateMatchesParams are the parameters for the CREATE_MATCHES job.
type CreateMatchesParams struct {
	ChannelID string `json:"channel_id"`
	RoundID   int32  `json:"round_id"`
}

// CreateMatches creates matches between active participants for a round of chat-roulette.
func CreateMatches(ctx context.Context, db *gorm.DB, client *slack.Client, p *CreateMatchesParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
		"round_id", p.RoundID,
	)

	// Wait for member jobs to completed before retrieving participants
	logger.Info("waiting up to 30 seconds for in-flight member jobs to complete")
	if err := waitOnMemberJobs(ctx, db, p.ChannelID); err != nil {
		return err
	}

	// Retrieve possible matches
	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	logger.Info("retrieving possible matches for this round of chat-roulette")
	var matches []ChatRoulettePair

	result := db.WithContext(dbCtx).
		Raw("SELECT * FROM GetRandomMatches(?)", p.ChannelID).
		Scan(&matches)

	if result.Error != nil {
		message := "failed to retrieve possible matches for chat-roulette"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}
	logger.Debug("retrieved possible matches for chat-roulette", "matches", result.RowsAffected)

	// Pair the active participants
	logger.Info("pairing active participants for chat-roulette")

	pairings := PairParticipants(matches)

	for participant, partner := range pairings {
		// Create a database record in the matches table for this pair
		newMatch := &models.Match{
			RoundID: p.RoundID,
		}

		dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()

		if err := db.WithContext(dbCtx).Create(newMatch).Error; err != nil {
			logger.Error("failed to create new match record in the database", "error", err)
			return err
		}

		logger.Info("added new match to the database", "match_id", newMatch.ID)

		// Queue a CREATE_PAIR job for this pair
		params := &CreatePairParams{
			MatchID:     newMatch.ID,
			ChannelID:   p.ChannelID,
			Participant: participant,
			Partner:     partner,
		}

		dbCtx, cancel = context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()

		if err := QueueCreatePairJob(dbCtx, db, params); err != nil {
			message := "failed to add CREATE_PAIR job to the queue"
			logger.Error(message, "error", result.Error)
			return errors.Wrap(result.Error, message)
		}

		logger.Info("queued CREATE_PAIR job for this match", "match_id", newMatch.ID)
	}

	logger.Info("paired active participants for chat-roulette", "participants", len(pairings)*2, "pairings", len(pairings))

	return nil
}

// QueueCreateMatchesJob adds a new CREATE_MATCHES job to the queue.
func QueueCreateMatchesJob(ctx context.Context, db *gorm.DB, p *CreateMatchesParams) error {
	job := models.GenericJob[*CreateMatchesParams]{
		JobType:  models.JobTypeCreateMatches,
		Priority: models.JobPriorityLow,
		Params:   p,
	}

	return QueueJob(ctx, db, job)
}
