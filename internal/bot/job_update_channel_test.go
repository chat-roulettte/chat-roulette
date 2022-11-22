package bot

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
)

type UpdateChannelSuite struct {
	suite.Suite
	ctx    context.Context
	mock   sqlmock.Sqlmock
	db     *gorm.DB
	logger hclog.Logger
	buffer *bytes.Buffer
}

func (s *UpdateChannelSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)
	s.db, s.mock = database.NewMockedGormDB()
}

func (s *UpdateChannelSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *UpdateChannelSuite) Test_UpdateChannel() {
	r := require.New(s.T())

	channelID := "C0123456789"
	interval := models.Biweekly
	weekday := time.Monday
	hour := 12

	// Mock updating the chat roulette channel's settings
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE "channels" SET (.*) WHERE channel_id = (.+)`).
		WithArgs(
			channelID,
			interval,
			weekday,
			hour,
			database.AnyTime(),
			database.AnyTime(),
			channelID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	p := &UpdateChannelParams{
		ChannelID: channelID,
		Interval:  "biweekly",
		Weekday:   "Monday",
		Hour:      12,
		NextRound: time.Now().UTC(),
	}

	// Mock canceling pending CREATE_ROUND jobs
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE "jobs" SET .* WHERE data->>'channel_id' = (.+) AND is_completed = false AND job_type = (.+)`).
		WithArgs(
			models.JobStatusCanceled,
			true,
			database.AnyTime(),
			channelID,
			models.JobTypeCreateRound,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	// Mock query to queue CREATE_ROUND job
	s.mock.ExpectBegin()
	s.mock.ExpectQuery(`INSERT INTO "jobs" (.+) VALUES (.+) RETURNING`).
		WithArgs(
			sqlmock.AnyArg(),
			models.JobTypeCreateRound.String(),
			models.JobPriorityStandard,
			models.JobStatusPending,
			false,
			sqlmock.AnyArg(),
			database.AnyTime(),
			database.AnyTime(),
			database.AnyTime(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	s.mock.ExpectCommit()

	err := UpdateChannel(s.ctx, s.db, nil, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "updated database row for the channel")
}

func (s *UpdateChannelSuite) Test_QueueUpdateChannelJob() {
	r := require.New(s.T())

	p := &UpdateChannelParams{
		ChannelID: "C0123456789",
		Interval:  "biweekly",
		Weekday:   "Monday",
		Hour:      12,
		NextRound: time.Now().Add(48 * time.Hour),
	}

	database.MockQueueJob(
		s.mock,
		p,
		models.JobTypeUpdateChannel.String(),
		models.JobPriorityHigh,
	)

	err := QueueUpdateChannelJob(s.ctx, s.db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")
}

func Test_UpdateChannel_suite(t *testing.T) {
	suite.Run(t, new(UpdateChannelSuite))
}
