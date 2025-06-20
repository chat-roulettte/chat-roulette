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

// UpdateMatchParams are the parameters for the UPDATE_MATCH job.
type UpdateMatchParams struct {
	MatchID int32 `json:"match_id"`
	HasMet  bool  `json:"has_met"`
}

// UpdateMatch updates the has_met status for a match at the end of a chat-roulette round.
func UpdateMatch(ctx context.Context, db *gorm.DB, client *slack.Client, p *UpdateMatchParams) error {

	logger := hclog.FromContext(ctx).With(attributes.MatchID, p.MatchID)

	logger.Info("updating match")

	// Update the has_met column for the match in the matches table
	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	result := db.WithContext(dbCtx).
		Model(&models.Match{}).
		Where("id = ?", p.MatchID).
		Update("has_met", p.HasMet)

	if result.Error != nil {
		message := "failed to update met status for the match"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	logger.Info("updated met status for the match")

	return nil
}

// QueueUpdateMatchJob adds a new UPDATE_MATCH job to the queue.
func QueueUpdateMatchJob(ctx context.Context, db *gorm.DB, p *UpdateMatchParams) error {
	job := models.GenericJob[*UpdateMatchParams]{
		JobType:  models.JobTypeUpdateMatch,
		Priority: models.JobPriorityHigh,
		Params:   p,
	}

	return QueueJob(ctx, db, job)
}
