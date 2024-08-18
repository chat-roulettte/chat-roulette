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

type AddChannelSuite struct {
	suite.Suite
	ctx    context.Context
	mock   sqlmock.Sqlmock
	db     *gorm.DB
	logger hclog.Logger
	buffer *bytes.Buffer
}

func (s *AddChannelSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)
	s.db, s.mock = database.NewMockedGormDB()
}

func (s *AddChannelSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *AddChannelSuite) Test_AddChannel() {
	r := require.New(s.T())

	channelID := "C0123456789"
	invitor := "U9999999999"
	now := time.Date(2022, time.January, 1, 12, 0, 0, 0, time.UTC)

	s.mock.ExpectQuery(`SELECT (.+) FROM "channels" WHERE "channels"."channel_id" = (.+) ORDER BY "channels"."channel_id" LIMIT  (?)`).
		WithArgs(
			channelID,
			1,
		).
		WillReturnRows(sqlmock.NewRows(nil))

	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO "channels" (.+) VALUES (.+)`).
		WithArgs(
			channelID,
			invitor,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			database.AnyTime(),
			database.AnyTime(),
			database.AnyTime(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	createRoundParams := CreateRoundParams{
		ChannelID: channelID,
		Interval:  "weekly",
		NextRound: now,
	}

	database.MockQueueJob(
		s.mock,
		createRoundParams,
		models.JobTypeCreateRound.String(),
		models.JobPriorityStandard,
	)

	syncMembersParams := SyncMembersParams{
		ChannelID: channelID,
	}

	database.MockQueueJob(
		s.mock,
		syncMembersParams,
		models.JobTypeSyncMembers.String(),
		models.JobPriorityHigh,
	)

	p := &AddChannelParams{
		ChannelID: channelID,
		Invitor:   invitor,
		Interval:  "weekly",
		Weekday:   "Monday",
		Hour:      11,
		NextRound: now,
	}

	err := AddChannel(s.ctx, s.db, nil, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added Slack channel to the database")
}

func (s *AddChannelSuite) Test_QueueAddChannelJob() {
	r := require.New(s.T())

	channelID := "C0123456789"

	p := &AddChannelParams{
		ChannelID: channelID,
	}

	database.MockQueueJob(
		s.mock,
		p,
		models.JobTypeAddChannel.String(),
		models.JobPriorityHighest,
	)

	err := QueueAddChannelJob(s.ctx, s.db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")
}

func Test_AddChannel_suite(t *testing.T) {
	suite.Run(t, new(AddChannelSuite))
}
