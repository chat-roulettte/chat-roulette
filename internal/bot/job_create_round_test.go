package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

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

type CreateRoundSuite struct {
	suite.Suite
	ctx    context.Context
	mock   sqlmock.Sqlmock
	db     *gorm.DB
	logger hclog.Logger
	buffer *bytes.Buffer
}

func (s *CreateRoundSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)
	s.db, s.mock = database.NewMockedGormDB()
}

func (s *CreateRoundSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *CreateRoundSuite) Test_CreateRound() {
	r := require.New(s.T())

	now := time.Date(2022, time.January, 1, 3, 0, 0, 0, time.UTC)

	p := &CreateRoundParams{
		ChannelID: "C0123456789",
		NextRound: now,
		Interval:  "weekly",
	}

	// Mock Slack API calls to /users.info and /users.conversations
	handlerFn := func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"ok": true,
			"channels": []slack.Channel{
				{
					GroupConversation: slack.GroupConversation{
						Conversation: slack.Conversation{
							ID: p.ChannelID,
						},
						Creator: "U0123456789",
					},
				},
			},
		}

		json.NewEncoder(w).Encode(response)
	}

	slackServer := slacktest.NewTestServer()
	slackServer.Handle("/users.conversations", handlerFn)
	go slackServer.Start()
	defer slackServer.Stop()

	client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(slackServer.GetAPIURL()))

	// Mock query to check/create if chat roulette round has ended
	s.mock.ExpectQuery(`SELECT (.+) FROM "rounds" WHERE channel_id = (.+) AND has_ended = false`).
		WithArgs(
			p.ChannelID,
			1,
		).
		WillReturnRows(sqlmock.NewRows(nil))

	s.mock.ExpectBegin()
	s.mock.ExpectQuery(`INSERT INTO "rounds" (.+) VALUES (.+) RETURNING`).
		WithArgs(
			p.ChannelID,
			false,
			0,
			database.AnyTime(),
			database.AnyTime(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	s.mock.ExpectCommit()

	// Mock query to update next_round column for the channel
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE "channels" SET "next_round"=(.+) WHERE channel_id = (.+)`).
		WithArgs(
			database.AnyTime(),
			database.AnyTime(),
			p.ChannelID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	// Mock query to queue END_ROUND job
	nextRound := NextChatRouletteRound(p.NextRound, models.Weekly)

	endRoundParams := &EndRoundParams{
		ChannelID: p.ChannelID,
		NextRound: nextRound,
	}

	database.MockQueueJob(
		s.mock,
		endRoundParams,
		models.JobTypeEndRound.String(),
		models.JobPriorityStandard,
	)

	reportStatsParams := &ReportStatsParams{
		ChannelID: p.ChannelID,
		NextRound: nextRound,
		RoundID:   1,
	}

	// Mock query to queue REPORT_STATS job
	database.MockQueueJob(
		s.mock,
		reportStatsParams,
		models.JobTypeReportStats.String(),
		models.JobPriorityLow,
	)

	// Mock query to queue CREATE_ROUND job
	database.MockQueueJob(
		s.mock,
		&CreateRoundParams{
			ChannelID: p.ChannelID,
			NextRound: nextRound,
			Interval:  p.Interval,
		},
		models.JobTypeCreateRound.String(),
		models.JobPriorityStandard,
	)

	// Mock query to queue SYNC_MEMBERS job
	database.MockQueueJob(
		s.mock,
		&SyncMembersParams{
			ChannelID: p.ChannelID,
		},
		models.JobTypeSyncMembers.String(),
		models.JobPriorityHigh,
	)

	// Mock query to queue CREATE_MATCHES job
	database.MockQueueJob(
		s.mock,
		&CreateMatchesParams{
			ChannelID: p.ChannelID,
			RoundID:   1,
		},
		models.JobTypeCreateMatches.String(),
		models.JobPriorityLow,
	)

	err := CreateRound(s.ctx, s.db, client, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "starting new chat-roulette round")
}

func (s *CreateRoundSuite) Test_QueueCreateRoundJob() {
	r := require.New(s.T())

	p := &CreateRoundParams{
		ChannelID: "C0123456789",
		NextRound: time.Now().UTC(),
		Interval:  "weekly",
	}

	database.MockQueueJob(s.mock, p, models.JobTypeCreateRound.String(), models.JobPriorityStandard)

	err := QueueCreateRoundJob(s.ctx, s.db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")
}

func Test_CreateRound_suite(t *testing.T) {
	suite.Run(t, new(CreateRoundSuite))
}
