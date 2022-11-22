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
	"github.com/chat-roulettte/chat-roulette/internal/timex"
)

// AddChannelParams are the parameters for the ADD_CHANNEL job.
type AddChannelParams struct {
	ChannelID string    `json:"channel_id"`
	Invitor   string    `json:"invitor"`
	Interval  string    `json:"interval"`
	Weekday   string    `json:"weekday"`
	Hour      int       `json:"hour"`
	NextRound time.Time `json:"next_round"`
}

// AddChannel adds a Slack channel to the database.
func AddChannel(ctx context.Context, db *gorm.DB, client *slack.Client, p *AddChannelParams) error {

	logger := hclog.FromContext(ctx).With(attributes.SlackChannelID, p.ChannelID)

	logger.Debug("adding Slack channel to the database")

	// Add Slack channel to the database
	weekday, _ := timex.ParseWeekday(p.Weekday)
	interval, err := models.ParseInterval(p.Interval)
	if err != nil {
		logger.Error("failed to parse interval", "error", err)
		return err
	}

	newChannel := &models.Channel{
		ChannelID: p.ChannelID,
		Inviter:   p.Invitor,
		Interval:  interval,
		Weekday:   weekday,
		Hour:      p.Hour,
		NextRound: p.NextRound,
	}

	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	result := db.WithContext(dbCtx).FirstOrCreate(newChannel)
	if result.Error != nil {
		message := "failed to add new Slack channel to the database"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	if result.RowsAffected != 1 {
		logger.Debug("Slack channel already exists in the database")
		return nil
	}

	logger.Debug("added Slack channel to the database")

	// Queue the first CREATE_ROUND job for the Slack channel.
	createRoundParams := &CreateRoundParams{
		ChannelID: p.ChannelID,
		NextRound: p.NextRound,
		Interval:  p.Interval,
	}

	if err := QueueCreateRoundJob(ctx, db, createRoundParams); err != nil {
		message := "failed to add job to the queue"
		logger.Error(message, "error", err, "job", "CREATE_ROUND")
		return errors.Wrap(err, message)
	}

	// Queue a SYNC_MEMBERS job.
	syncMembersParams := &SyncMembersParams{
		ChannelID: p.ChannelID,
	}

	if err := QueueSyncMembersJob(ctx, db, syncMembersParams); err != nil {
		// Don't fail the entire job if this errors.
		// SYNC_MEMBERS will be scheduled before the start of each round.
		logger.Error("failed to add job to the queue", "error", err, "job", models.JobTypeSyncMembers.String())
	}

	return nil
}

// QueueAddChannelJob adds a new ADD_CHANNEL job to the queue
func QueueAddChannelJob(ctx context.Context, db *gorm.DB, p *AddChannelParams) error {
	job := models.GenericJob[*AddChannelParams]{
		JobType:  models.JobTypeAddChannel,
		Priority: models.JobPriorityHighest,
		Params:   p,
	}

	return QueueJob(ctx, db, job)
}
