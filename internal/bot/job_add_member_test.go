package bot

import (
	"bytes"
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hashicorp/go-hclog"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slacktest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
)

type AddMemberSuite struct {
	suite.Suite
	ctx    context.Context
	mock   sqlmock.Sqlmock
	db     *gorm.DB
	logger hclog.Logger
	buffer *bytes.Buffer
}

func (s *AddMemberSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)
	s.db, s.mock = database.NewMockedGormDB()
}

func (s *AddMemberSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *AddMemberSuite) Test_AddMember() {
	r := require.New(s.T())

	p := &AddMemberParams{
		ChannelID: "C0123456789",
		UserID:    "U9999999999",
	}

	s.mock.ExpectQuery(`SELECT (.?) FROM "members" WHERE user_id = (.+) AND channel_id = (.+)`).
		WithArgs(
			p.UserID,
			p.ChannelID,
			1,
		).
		WillReturnRows(sqlmock.NewRows(nil))

	s.mock.ExpectBegin()
	s.mock.ExpectQuery(`INSERT INTO "members" (.+) VALUES (.+) RETURNING`).
		WithArgs(
			p.UserID,
			p.ChannelID,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			false,
			false,
			database.AnyTime(),
			database.AnyTime(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	s.mock.ExpectCommit()

	greetMemberParams := &GreetMemberParams{
		ChannelID: p.ChannelID,
		UserID:    p.UserID,
	}

	database.MockQueueJob(
		s.mock,
		greetMemberParams,
		models.JobTypeGreetMember.String(),
		models.JobPriorityStandard,
	)

	slackServer := slacktest.NewTestServer()
	go slackServer.Start()
	defer slackServer.Stop()

	client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(slackServer.GetAPIURL()))

	err := AddMember(s.ctx, s.db, client, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added Slack user to the database")
	r.Contains(s.buffer.String(), "queued GREET_MEMBER job for this user")
}

func (s *AddMemberSuite) Test_QueueAddMemberJob() {
	r := require.New(s.T())

	channelID := "C0123456789"
	userID := "U9999999999"

	p := &AddMemberParams{
		ChannelID: channelID,
		UserID:    userID,
	}

	database.MockQueueJob(s.mock, p, models.JobTypeAddMember.String(), models.JobPriorityHigh)

	err := QueueAddMemberJob(s.ctx, s.db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")
}

func Test_AddMember_suite(t *testing.T) {
	suite.Run(t, new(AddMemberSuite))
}
