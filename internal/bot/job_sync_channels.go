package bot

import (
	"context"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
)

// SyncChannelsParams are the parameters for SYNC_CHANNEL job.
type SyncChannelsParams struct {
	BotUserID string `json:"bot_user_id"`
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
		logger = logger.With(
			attributes.SlackChannelID, channel.ChannelID,
		)

		switch {
		case channel.Create:
			// Skip scheduling GREET_ADMIN job if it has already been done
			dbCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			defer cancel()

			var count int64
			if err := db.WithContext(dbCtx).Model(&models.Job{}).Where("job_type = ?", models.JobTypeGreetAdmin).
				Where(datatypes.JSONQuery("data").Equals(channel.ChannelID, "channel_id")).
				Count(&count).Error; err != nil {
				logger.Error("failed to check if GREET_ADMIN job has already been scheduled for this channel", "error", err)
			}

			if count == 1 {
				logger.Info("Skipping as GREET_ADMIN job has already been scheduled for this channel")
				continue
			}

			// Queue a GREET_ADMIN job for the new Slack channel to start onboarding
			p := &GreetAdminParams{
				ChannelID: channel.ChannelID,
				Inviter:   channel.Inviter,
			}

			if err := QueueGreetAdminJob(ctx, db, p); err != nil {
				message := "failed to add job to the queue"
				logger.Error(message, "error", err, "job", models.JobTypeGreetAdmin.String())
				return errors.Wrap(err, message)
			}

		case channel.Delete:
			// Queue a DELETE_CHANNEL job for the Slack channel.
			p := &DeleteChannelParams{
				ChannelID: channel.ChannelID,
			}

			if err := QueueDeleteChannelJob(ctx, db, p); err != nil {
				message := "failed to add job to the queue"
				logger.Error(message, "error", err, "job", models.JobTypeDeleteChannel.String())
				return errors.Wrap(err, message)
			}

		case !channel.Create && !channel.Delete:
			// Queue a SYNC_MEMBERS job if the channel already exists.
			syncMembersParams := &SyncMembersParams{
				ChannelID: channel.ChannelID,
			}

			if err := QueueSyncMembersJob(ctx, db, syncMembersParams); err != nil {
				// Don't fail the entire job if this errors. SYNC_MEMBERS will be scheduled before the start of each round.
				logger.Error("failed to add job to the queue", "error", err, "job", models.JobTypeSyncMembers.String())
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
