package bot

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
)

func Test_QueueJob(t *testing.T) {
	r := require.New(t)

	resource, databaseURL, err := database.NewTestPostgresDB(true)
	r.NoError(err)
	defer resource.Close()

	db, err := database.NewGormDB(databaseURL)
	r.NoError(err)

	logger, buffer := o11y.NewBufferedLogger()
	ctx := hclog.WithContext(context.Background(), logger)

	channelID := "C0123456789"
	timestamp := time.Now().Add(60 * time.Minute)

	job := models.GenericJob[*UpdateChannelParams]{
		JobType:  models.JobTypeUpdateChannel,
		Priority: models.JobPriorityHigh,
		Params: &UpdateChannelParams{
			ChannelID: "C0123456789",
			Interval:  "biweekly",
			Weekday:   "Monday",
			Hour:      12,
		},
		ExecAt: timestamp,
	}

	err = QueueJob(ctx, db, job)
	r.NoError(err)
	r.Contains(buffer.String(), "added new job to the database")

	var count int64
	result := db.Model(&models.Job{}).
		Where("job_type = ?", models.JobTypeUpdateChannel).
		Where("priority = ?", models.JobPriorityHigh).
		Where("data->>'channel_id' = ?", channelID).
		Where("exec_at = ?", timestamp).
		Count(&count)
	r.NoError(result.Error)
	r.Equal(int64(1), count)
}

func Test_ExecJob(t *testing.T) {
	type Params struct {
		Content string
	}

	jobFunc := func(ctx context.Context, db *gorm.DB, client *slack.Client, p *Params) error {
		return nil
	}

	p := &Params{
		Content: "Hello World",
	}

	data, _ := json.Marshal(p)
	job := models.NewJob(models.JobTypeUpdateMember, data)

	err := ExecJob(context.Background(), nil, nil, job, jobFunc)
	assert.NoError(t, err)
}

func Test_extractChannelIDFromParams(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		channelID := "C0123456789"

		p := AddMemberParams{
			ChannelID: channelID,
			UserID:    "U9876543210",
		}

		s := extractChannelIDFromParams(p)

		assert.Equal(t, channelID, s)
	})

	t.Run("not found", func(t *testing.T) {
		s := extractChannelIDFromParams(UpdateMatchParams{})

		assert.Equal(t, "", s)
	})
}
