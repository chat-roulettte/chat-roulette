package bot

import (
	"context"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/config"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
)

// SyncChannelsParams are the parameters for SYNC_CHANNEL job.
type SyncChannelsParams struct {
	BotUserID          string                    `json:"bot_user_id"`
	ChatRouletteConfig config.ChatRouletteConfig `json:"config"`
}

// SyncChannels ensures that there is no discrepancy between the Slack channels in
// the database and the Slack channels that the bot is a member of.
func SyncChannels(ctx context.Context, db *gorm.DB, client *slack.Client, p *SyncChannelsParams) error {

	logger := hclog.FromContext(ctx)

	logger.Info("syncing Slack channels")

	// Get the list of Slack channels that the bot is a member of from Slack
	slackChannels, err := getChannels(ctx, client, p.BotUserID)
	if err != nil {
		message := "failed to retrieve Slack channel membership"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	logger.Debug("retrieved the list of Slack channels from Slack", "channels_count", len(slackChannels))

	// Get the list of Slack channels that the bot is a member of from the database
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	var dbChannels []chatRouletteChannel
	result := db.WithContext(ctx).Model(&models.Channel{}).Select("channel_id", "inviter").Find(&dbChannels)
	if result.Error != nil {
		message := "failed to retrieve Slack channels from the database"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	logger.Debug("retrieved the list of Slack channels from the database", "channels_count", len(slackChannels))

	// Reconcile between the two lists
	channels := reconcileChannels(ctx, slackChannels, dbChannels)

	for _, channel := range channels {
		switch {
		case channel.Create:
			// Get the timestamp for the first chat roulette round
			firstRound := FirstChatRouletteRound(time.Now().UTC(), p.ChatRouletteConfig.Weekday, p.ChatRouletteConfig.Hour)

			// Queue an ADD_CHANNEL job for the new Slack channel.
			p := &AddChannelParams{
				ChannelID: channel.ChannelID,
				Invitor:   channel.Invitor,
				Interval:  p.ChatRouletteConfig.Interval,
				Weekday:   p.ChatRouletteConfig.Weekday,
				Hour:      p.ChatRouletteConfig.Hour,
				NextRound: firstRound,
			}

			if err := QueueAddChannelJob(ctx, db, p); err != nil {
				message := "failed to add job to the queue"
				logger.Error(message, "error", err, "job", "ADD_CHANNEL")
				return errors.Wrap(err, message)
			}

		case channel.Delete:
			// Queue a DELETE_CHANNEL job for the Slack channel.
			p := &DeleteChannelParams{
				ChannelID: channel.ChannelID,
			}

			if err := QueueDeleteChannelJob(ctx, db, p); err != nil {
				message := "failed to add job to the queue"
				logger.Error(message, "error", err, "job", "DELETE_CHANNEL")
				return errors.Wrap(err, message)
			}

		case !channel.Create && !channel.Delete:
			// Queue a SYNC_MEMBERS job if the channel already exists.
			syncMembersParams := &SyncMembersParams{
				ChannelID: channel.ChannelID,
			}

			if err := QueueSyncMembersJob(ctx, db, syncMembersParams); err != nil {
				// Don't fail the entire job if this errors. SYNC_MEMBERS will be scheduled before the start of each round.
				logger.Error("failed to add job to the queue", "error", err, "job", "SYNC_MEMBERS")
			}
		}
	}

	return nil
}

// QueueSyncChannelsJob adds a new SYNC_CHANNELS job to the queue.
func QueueSyncChannelsJob(ctx context.Context, db *gorm.DB, p *SyncChannelsParams) error {
	job := models.GenericJob[*SyncChannelsParams]{
		JobType:  models.JobTypeSyncChannels,
		Priority: models.JobPriorityHighest,
		Params:   p,
	}

	return QueueJob(ctx, db, job)
}
