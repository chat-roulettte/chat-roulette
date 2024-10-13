package bot

import (
	"context"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
	"github.com/chat-roulettte/chat-roulette/internal/timex"
)

// CreateMatchParams are the parameters for the CREATE_MATCH job.
type CreateMatchParams struct {
	ChannelID   string `json:"channel_id"`
	Participant string `json:"participant"`
}

// CreateMatch creates a match for a participant who has joined late to a round of chat-roulette.
func CreateMatch(ctx context.Context, db *gorm.DB, client *slack.Client, p *CreateMatchParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
		attributes.SlackUserID, p.Participant,
	)

	// Check if an active round of Chat Roulette is in progress
	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	var roundID int32
	result := db.WithContext(dbCtx).
		Model(&models.Round{}).
		Select("id").
		Where("channel_id = ?", p.ChannelID).
		Where("has_ended = false").
		Order("id DESC").
		First(&roundID)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			logger.Warn("unable to match participant: no active Chat Roulette round found")
			return nil
		}

		message := "failed to check if an active Chat Roulette round is in progress"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	// Check if the current round has enough time remaining
	// There must be more than half of the time remaining in the round
	dbCtx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	var nextRound time.Time
	result = db.WithContext(dbCtx).
		Model(&models.Channel{}).
		Select("next_round").
		Where("channel_id = ?", p.ChannelID).
		First(&nextRound)

	if result.Error != nil {
		message := "failed to retrieve timestamp of next Chat Roulette round"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	dbCtx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	var currentRound time.Time
	result = db.WithContext(dbCtx).
		Model(&models.Job{}).
		Select("exec_at").
		Where("status = ?", models.JobStatusSucceeded).
		Where("is_completed = true").
		Where("job_type = ?", models.JobTypeCreateRound.String()).
		Where(datatypes.JSONQuery("data").Equals(p.ChannelID, "channel_id")).
		First(&currentRound)

	if result.Error != nil {
		message := "failed to retrieve timestamp of current Chat Roulette round"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	t, err := timex.MidPoint(currentRound, nextRound)
	if err != nil {
		message := "failed to determine mid point between current Chat Roulette round and next round"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	if t.Before(time.Now().UTC()) {
		logger.Warn("unable to match participant: not enough time remaining in current Chat Roulette round")
		return nil
	}

	// Retrieve preferences of this participant
	dbCtx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	var participant models.Member
	result = db.WithContext(dbCtx).
		Model(&models.Member{}).
		Where("channel_id = ?", p.ChannelID).
		First(&participant)

	if result.Error != nil {
		message := "failed to retrieve the participant from the database"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	// Find a suitable partner for this participant
	// This will try to match them with another active user who did not get matched during this round
	dbCtx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	var partner string

	subQuery := db.Table("pairings").
		Select("pairings.member_id").
		Joins("JOIN matches matches ON pairings.match_id = matches.id").
		Where("matches.round_id = ?", roundID)

	query := db.WithContext(dbCtx).
		Model(&models.Member{}).
		Select("user_id").
		Where("channel_id = ?", p.ChannelID).
		Where("is_active = true").
		Not("id IN (?)", subQuery).
		Not("user_id = ?", p.Participant)

	if *participant.HasGenderPreference {
		query = query.Where("gender = ?", participant.Gender).
			Order(clause.OrderByColumn{
				Column: clause.Column{Name: "has_gender_preference"},
				Desc:   true,
			})
	}

	if err := query.First(&partner).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warn("unable to match participant: no suitable partner found")
			return nil
		}

		message := "failed to lookup a suitable partner for this participant"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	// Verify that the selected partner has not already been matched
	dbCtx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	var count int64
	result = db.WithContext(dbCtx).
		Model(&models.Job{}).
		Where(
			db.Where(
				datatypes.JSONQuery("data").Equals(p.Participant, "participant")).
				Or(datatypes.JSONQuery("data").Equals(partner, "participant")).
				Or(datatypes.JSONQuery("data").Equals(p.Participant, "partner")).
				Or(datatypes.JSONQuery("data").Equals(partner, "partner"))).
		Where("status = ?", models.JobStatusPending).
		Where("is_completed = false").
		Where("job_type = ?", models.JobTypeCreatePair.String()).Count(&count)

	if result.Error != nil {
		message := "failed to check if partner has already been matched"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, "failed to check if partner has already been matched")
	}

	if count > 0 {
		logger.Warn("unable to match participant: partner has already been matched")
		return nil
	}

	// Create a new match for this pair
	newMatch := &models.Match{
		RoundID: roundID,
	}

	dbCtx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	if err := db.WithContext(dbCtx).Create(newMatch).Error; err != nil {
		message := "failed to create new match record in the database"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	logger.Debug("added new match to the database", "match_id", newMatch.ID)

	// Queue a CREATE_PAIR job for this pair
	params := &CreatePairParams{
		MatchID:     newMatch.ID,
		ChannelID:   p.ChannelID,
		Participant: p.Participant,
		Partner:     partner,
	}

	dbCtx, cancel = context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	if err := QueueCreatePairJob(dbCtx, db, params); err != nil {
		message := "failed to add CREATE_PAIR job to the queue"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	logger.Info("queued CREATE_PAIR job for this match", "match_id", newMatch.ID)

	return nil
}

// QueueCreateMatchJob adds a new CREATE_MATCH job to the queue.
func QueueCreateMatchJob(ctx context.Context, db *gorm.DB, p *CreateMatchParams) error {
	job := models.GenericJob[*CreateMatchParams]{
		JobType:  models.JobTypeCreateMatch,
		Priority: models.JobPriorityLow,
		Params:   p,
		ExecAt:   time.Now().UTC().Add(10 * time.Minute), // 10 minute delay on execution
	}

	return QueueJob(ctx, db, job)
}
