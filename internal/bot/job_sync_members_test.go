package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
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

type SyncMembersSuite struct {
	suite.Suite
	ctx    context.Context
	mock   sqlmock.Sqlmock
	db     *gorm.DB
	logger hclog.Logger
	buffer *bytes.Buffer
}

func (s *SyncMembersSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)
	s.db, s.mock = database.NewMockedGormDB()
}

func (s *SyncMembersSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *SyncMembersSuite) Test_SyncMembers() {
	r := require.New(s.T())

	addMember := "U5555555555"    // this user will be created
	deleteMember := "U9999999999" // this user will be deleted

	slackMembers := []string{
		"U0123456789",
		"U9876543210",
		"U1111111111",
		addMember,
	}

	// Mock Slack API call
	handlerFn := func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"ok":      true,
			"members": slackMembers,
		}

		json.NewEncoder(w).Encode(response)
	}

	slackServer := slacktest.NewTestServer()
	slackServer.Handle("/conversations.members", handlerFn)
	go slackServer.Start()
	defer slackServer.Stop()

	client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(slackServer.GetAPIURL()))

	p := &SyncMembersParams{
		ChannelID: "C0123456789",
	}

	dbMembers := append([]string{}, slackMembers[:3]...)
	dbMembers = append(dbMembers, deleteMember)

	// Mock DB call to retrieve the list of channel members
	s.mock.ExpectQuery(`SELECT "user_id" FROM "members" WHERE channel_id = (.+)`).
		WithArgs(p.ChannelID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).FromCSVString(strings.Join(dbMembers, "\n")))

	// Mock the ADD_MEMBER job
	database.MockQueueJob(
		s.mock,
		&AddMemberParams{
			ChannelID: p.ChannelID,
			UserID:    addMember,
		},
		models.JobTypeAddMember.String(),
		models.JobPriorityHigh,
	)

	// Mock the DELETE_MEMBER job
	database.MockQueueJob(
		s.mock,
		&DeleteMemberParams{
			ChannelID: p.ChannelID,
			UserID:    deleteMember,
		},
		models.JobTypeDeleteMember.String(),
		models.JobPriorityHigh,
	)

	err := SyncMembers(s.ctx, s.db, client, p)
	r.NoError(err)
}

func (s *SyncMembersSuite) Test_SyncMembersJob() {
	r := require.New(s.T())

	p := &SyncMembersParams{
		ChannelID: "C0123456789",
	}

	database.MockQueueJob(
		s.mock,
		p,
		models.JobTypeSyncMembers.String(),
		models.JobPriorityHigh,
	)

	err := QueueSyncMembersJob(s.ctx, s.db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")
}

func Test_SyncMembers_suite(t *testing.T) {
	suite.Run(t, new(SyncMembersSuite))
}
