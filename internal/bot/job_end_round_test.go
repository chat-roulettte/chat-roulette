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

type EndRoundSuite struct {
	suite.Suite
	ctx    context.Context
	mock   sqlmock.Sqlmock
	db     *gorm.DB
	logger hclog.Logger
	buffer *bytes.Buffer
}

func (s *EndRoundSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)
	s.db, s.mock = database.NewMockedGormDB()
}

func (s *EndRoundSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *EndRoundSuite) Test_EndRound() {
	r := require.New(s.T())

	p := &EndRoundParams{
		ChannelID: "C030DJ523N3",
		NextRound: time.Now().Add(24 * 60 * 7 * time.Hour),
	}

	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE "rounds" SET "has_ended"=(.+),"updated_at"=(.+) WHERE channel_id = (.+) AND has_ended = false`).
		WithArgs(
			true,
			database.AnyTime(),
			p.ChannelID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := EndRound(s.ctx, s.db, nil, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "ended the last chat-roulette round")
}

func (s *EndRoundSuite) Test_QueueEndRoundJob() {
	r := require.New(s.T())

	p := &EndRoundParams{
		ChannelID: "C030DJ523N3",
		NextRound: time.Now().Add(24 * 60 * 7 * time.Hour),
	}

	database.MockQueueJob(s.mock, p, models.JobTypeEndRound.String(), models.JobPriorityStandard)

	err := QueueEndRoundJob(s.ctx, s.db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")
}

func Test_EndRound_suite(t *testing.T) {
	suite.Run(t, new(EndRoundSuite))
}
