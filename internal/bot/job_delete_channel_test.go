package bot

import (
	"bytes"
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
)

type DeleteChannelSuite struct {
	suite.Suite
	ctx    context.Context
	mock   sqlmock.Sqlmock
	db     *gorm.DB
	logger hclog.Logger
	buffer *bytes.Buffer
}

func (s *DeleteChannelSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)
	s.db, s.mock = database.NewMockedGormDB()
}

func (s *DeleteChannelSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *DeleteChannelSuite) Test_DeleteChannel() {
	r := require.New(s.T())

	channelID := "C0123456789"

	// Mock query to cancel all pending jobs
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE "jobs" SET "status"=(.+),"is_completed"=(.+),"updated_at"=(.+) WHERE data->>'channel_id' = (.+) AND is_completed = false`).
		WithArgs(
			models.JobStatusCanceled,
			true,
			database.AnyTime(),
			channelID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	// Mock query to delete the channel
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`DELETE FROM "channels" WHERE channel_id = (.+)`).
		WithArgs(channelID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	p := &DeleteChannelParams{
		ChannelID: channelID,
	}

	err := DeleteChannel(s.ctx, s.db, nil, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "deleted Slack channel from the database")
}

func (s *DeleteChannelSuite) Test_DeleteChannelJob() {
	r := require.New(s.T())

	p := &DeleteChannelParams{
		ChannelID: "C0123456789",
	}

	database.MockQueueJob(
		s.mock,
		p,
		models.JobTypeDeleteChannel.String(),
		models.JobPriorityHighest,
	)

	err := QueueDeleteChannelJob(s.ctx, s.db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")
}

func Test_DeleteChannel_suite(t *testing.T) {
	suite.Run(t, new(DeleteChannelSuite))
}
