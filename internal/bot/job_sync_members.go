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

// SyncMembersParams are the parameters for the SYNC_MEMBERS job.
type SyncMembersParams struct {
	ChannelID string `json:"channel_id"`
}

// SyncMembers ensures that there is no discrepancy between the members
// of Slack channels in the database and in Slack.
func SyncMembers(ctx context.Context, db *gorm.DB, client *slack.Client, p *SyncMembersParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
	)

	logger.Info("syncing Slack members")

	// Get the list of channel members from Slack
	slackMembers, err := getChannelMembers(ctx, client, p.ChannelID, 100)
	if err != nil {
		logger.Error("failed to retrieve channel membership from Slack", "error", err)
		return err
	}

	logger.Debug("retrieved the list of members from Slack", "members_count", len(slackMembers))

	// Get the list of channel members from the database
	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	var dbMembers []chatRouletteMember

	result := db.WithContext(dbCtx).
		Model(&models.Member{}).
		Select("user_id").
		Where("channel_id = ?", p.ChannelID).
		Find(&dbMembers)

	if result.Error != nil {
		message := "failed to retrieve channel membership from the database"
		logger.Error(message, "error", err)
		return errors.Wrap(result.Error, message)
	}

	logger.Debug("retrieved the list of members from the database", "members_count", len(dbMembers))

	// Reconcile between the two lists
	members := reconcileMembers(ctx, slackMembers, dbMembers)

	for _, member := range members {
		switch {
		case member.Create:
			// Queue an ADD_MEMBER job for the new member of the Slack channel.
			p := &AddMemberParams{
				ChannelID: p.ChannelID,
				UserID:    member.UserID,
			}

			if err := QueueAddMemberJob(ctx, db, p); err != nil {
				message := "failed to add job to the queue"
				logger.Error(message, "error", err, "job", "ADD_MEMBER")
				return errors.Wrap(err, message)
			}

		case member.Delete:
			// Queue a DELETE_MEMBER job for the user that is no longer a member of the Slack channel.
			p := &DeleteMemberParams{
				ChannelID: p.ChannelID,
				UserID:    member.UserID,
			}

			// Queue an DELETE_MEMBER job for each member of the Slack channel.
			if err := QueueDeleteMemberJob(ctx, db, p); err != nil {
				message := "failed to add job to the queue"
				logger.Error(message, "error", err, "job", "DELETE_MEMBER")
				return errors.Wrap(err, message)
			}
		}
	}

	return nil
}

// QueueSyncMembersJob adds a new SYNC_MEMBERS job to the queue.
func QueueSyncMembersJob(ctx context.Context, db *gorm.DB, p *SyncMembersParams) error {
	job := models.GenericJob[*SyncMembersParams]{
		JobType:  models.JobTypeSyncMembers,
		Priority: models.JobPriorityHigh,
		Params:   p,
	}

	return QueueJob(ctx, db, job)
}
