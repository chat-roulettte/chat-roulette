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

// AddMemberParams are the parameters for the ADD_MEMBER job.
type AddMemberParams struct {
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
}

// AddMember adds a new member of a Slack channel to the database
// and begins the onboarding process for them.
func AddMember(ctx context.Context, db *gorm.DB, client *slack.Client, p *AddMemberParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
		attributes.SlackUserID, p.UserID,
	)

	// Skip bot users as they cannot participate in chat-roulette
	// This checks for all bot users in the channel and not only the chat-roulette bot
	if isBot, err := isUserASlackBot(ctx, client, p.UserID); err != nil {
		message := "failed to check if this Slack user is a bot"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	} else if isBot {
		logger.Debug("skipping because this Slack user is a bot")
		return nil
	}

	// Add the Slack user to the database
	logger.Info("adding Slack user to the database")

	isActive := false

	newMember := &models.Member{
		ChannelID:           p.ChannelID,
		UserID:              p.UserID,
		ConnectionMode:      models.ConnectionModeHybrid,
		Gender:              models.Male,
		IsActive:            &isActive,
		HasGenderPreference: new(bool),
	}

	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	result := db.WithContext(dbCtx).
		Where("user_id = ? AND channel_id = ?", newMember.UserID, newMember.ChannelID).
		FirstOrCreate(newMember)

	if result.Error != nil {
		err := result.Error
		message := "failed to add new Slack user to the database"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	// We cannot depend on results.RowsAffected == 1 here because the
	// database query always returns 1 row
	if time.Since(newMember.CreatedAt) >= 1*time.Second {
		logger.Debug("Slack user already exists in the database")
	} else {
		logger.Info("added Slack user to the database")

		// Queue a GREET_MEMBER job
		params := &GreetMemberParams{
			ChannelID: p.ChannelID,
			UserID:    p.UserID,
		}

		dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()

		if err := QueueGreetMemberJob(dbCtx, db, params); err != nil {
			message := "failed to add GREET_MEMBER job to the queue"
			logger.Error(message, "error", result.Error)
			return errors.Wrap(result.Error, message)
		}

		logger.Info("queued GREET_MEMBER job for this user")
	}

	return nil
}

// QueueAddChannelJob adds a new ADD_MEMBER job to the queue.
func QueueAddMemberJob(ctx context.Context, db *gorm.DB, p *AddMemberParams) error {
	job := models.GenericJob[*AddMemberParams]{
		JobType:  models.JobTypeAddMember,
		Priority: models.JobPriorityHigh,
		Params:   p,
	}

	return QueueJob(ctx, db, job)
}
