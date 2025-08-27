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

// CreateRoundParams are the parameters for the CREATE_ROUND job.
type CreateRoundParams struct {
	ChannelID string    `json:"channel_id"`
	NextRound time.Time `json:"next_round"`
	Interval  string
}

// CreateRound adds a new chat roulette round for a Slack channel to the database.
func CreateRound(ctx context.Context, db *gorm.DB, client *slack.Client, p *CreateRoundParams) error {

	logger := hclog.FromContext(ctx).With(attributes.SlackChannelID, p.ChannelID)

	// Do not start a new chat roulette round, if the bot is not in the Slack channel.
	// This is necessary to check because a bot user cannot receive "member_left_channel"
	// events when the bot is removed from a Slack channel.
	botUserID, err := GetBotUserID(ctx, client)
	if err != nil {
		message := "failed to retrieve bot user ID"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	isMember, err := isBotAChannelMember(ctx, client, botUserID, p.ChannelID)
	if err != nil {
		return err
	}

	if !isMember {
		// Queue a DELETE_CHANNEL job to offboard the Slack channel
		p := &DeleteChannelParams{
			ChannelID: p.ChannelID,
		}

		if err := QueueDeleteChannelJob(ctx, db, p); err != nil {
			message := "failed to add DELETE_CHANNEL job to the queue"
			logger.Error(message, "error", err)
			return errors.Wrap(err, message)
		}

		return nil
	}

	logger.Info("starting new chat-roulette round")

	// Start a new chat-roulette round only if the last round has ended
	newRound := &models.Round{
		ChannelID: p.ChannelID,
	}

	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	result := db.WithContext(dbCtx).
		Where("channel_id = ? AND has_ended = false", newRound.ChannelID).
		FirstOrCreate(newRound)

	if result.Error != nil {
		message := "failed to start new chat-roulette round"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	// Check if a new chat-roulette round was started
	if time.Since(newRound.CreatedAt) >= 1*time.Second {
		logger.Info("a chat-roulette round is already in progress")
		return nil
	}

	logger.Info("started new chat-roulette round")

	// Update the next_round column for the channel
	interval, err := models.IntervalEnumString(p.Interval)
	if err != nil {
		return err
	}

	nextRound := NextChatRouletteRound(p.NextRound, interval)

	dbCtx, cancel = context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	result = db.WithContext(dbCtx).
		Model(&models.Channel{}).
		Where("channel_id = ?", p.ChannelID).
		Update("next_round", nextRound)

	if result.Error != nil {
		message := "failed to update next_round for Slack channel"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	// Queue an END_ROUND job for this round of chat-roulette
	endRoundParams := &EndRoundParams{
		ChannelID: p.ChannelID,
		NextRound: nextRound,
	}

	if err := QueueEndRoundJob(ctx, db, endRoundParams); err != nil {
		message := "failed to add END_ROUND job to the queue"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	// Queue a REPORT_STATS job for this round of chat-roulette
	reportStatsParams := &ReportStatsParams{
		ChannelID: p.ChannelID,
		RoundID:   newRound.ID,
		NextRound: nextRound,
	}

	if err := QueueReportStatsJob(ctx, db, reportStatsParams); err != nil {
		message := "failed to add REPORT_STATS job to the queue"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	// Queue a CREATE_ROUND job for the next round of chat-roulette
	createRoundParams := &CreateRoundParams{
		ChannelID: p.ChannelID,
		NextRound: nextRound,
		Interval:  p.Interval,
	}

	if err := QueueCreateRoundJob(ctx, db, createRoundParams); err != nil {
		message := "failed to add CREATE_ROUND job to the queue"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	// Queue a SYNC_MEMBERS job before matching participants
	syncMembersParams := &SyncMembersParams{
		ChannelID: p.ChannelID,
	}

	if err := QueueSyncMembersJob(ctx, db, syncMembersParams); err != nil {
		message := "failed to add SYNC_MEMBERS job to the queue"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	// Queue a CREATE_MATCHES job for the current round of chat roulette
	createMatchesParams := &CreateMatchesParams{
		ChannelID: p.ChannelID,
		RoundID:   newRound.ID,
	}

	if err := QueueCreateMatchesJob(ctx, db, createMatchesParams); err != nil {
		message := "failed to add CREATE_MATCHES job to the queue"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	return nil
}

// QueueCreateRoundJob adds a new CREATE_ROUND job to the queue.
func QueueCreateRoundJob(ctx context.Context, db *gorm.DB, p *CreateRoundParams) error {
	job := models.GenericJob[*CreateRoundParams]{
		JobType:  models.JobTypeCreateRound,
		Priority: models.JobPriorityStandard,
		Params:   p,
		ExecAt:   p.NextRound,
	}

	return QueueJob(ctx, db, job)
}
