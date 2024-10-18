package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/ory/dockertest"
	"github.com/sebdah/goldie/v2"
	"github.com/segmentio/ksuid"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
)

func Test_kickoffPairTemplate(t *testing.T) {
	g := goldie.New(t)

	p := kickoffPairTemplate{
		Participant: "U0123456789",
		Partner:     "U9876543210",
		Volunteer:   "U9876543210",
	}

	content, err := renderTemplate(kickoffPairTemplateFilename, p)
	assert.Nil(t, err)

	g.Assert(t, "kickoff_pair.json", []byte(content))
}

type KickoffPairSuite struct {
	suite.Suite
	ctx      context.Context
	db       *gorm.DB
	logger   hclog.Logger
	buffer   *bytes.Buffer
	resource *dockertest.Resource
}

func (s *KickoffPairSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)

	resource, databaseURL, err := database.NewTestPostgresDB(false)
	require.NoError(s.T(), err)
	s.resource = resource

	db, err := database.NewGormDB(databaseURL)
	require.NoError(s.T(), err)
	require.NoError(s.T(), database.Migrate(databaseURL))

	s.db = db
}

func (s *KickoffPairSuite) AfterTest(_, _ string) {}

func (s *KickoffPairSuite) Test_KickoffPair() {
	r := require.New(s.T())

	channelID := "C0123456789"
	mpimID := "G1111111111"

	// Write channel to the database
	s.db.Create(&models.Channel{
		ChannelID:      channelID,
		Inviter:        "U9876543210",
		ConnectionMode: models.VirtualConnectionMode,
		Interval:       models.Biweekly,
		Weekday:        time.Friday,
		Hour:           12,
		NextRound:      time.Now().Add(336 * time.Hour), // 14 days in the future
	})

	// Write records in the rounds and matches table
	s.db.Create(&models.Round{
		ChannelID: channelID,
	})

	s.db.Create(&models.Match{
		RoundID:     1,
		MpimID:      "G1111111111",
		WasNotified: true,
	})

	// Write NOTIFY_PAIR job to the database
	p := &NotifyPairParams{
		ChannelID:   channelID,
		MatchID:     1,
		Participant: "U0123456789",
		Partner:     "U8765432109",
	}

	data, _ := json.Marshal(p)

	s.db.Create(&models.Job{
		JobID:       ksuid.New(),
		JobType:     models.JobTypeNotifyPair,
		Priority:    models.JobPriorityStandard,
		Status:      models.JobStatusSucceeded,
		IsCompleted: true,
		Data:        data,
		ExecAt:      time.Now().UTC().Add(-(24 * time.Hour)),
	})

	// Mock Slack API call
	mux := http.NewServeMux()
	mux.HandleFunc("/conversations.open", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(`{"ok":true,"channel":{"id":"G1111111111", "is_mpim":true}}`))
	})

	var testname string
	mux.HandleFunc("/conversations.history", func(w http.ResponseWriter, req *http.Request) {
		var r *slack.GetConversationHistoryResponse

		switch testname {
		case "bot message":
			r = &slack.GetConversationHistoryResponse{
				SlackResponse: slack.SlackResponse{
					Ok: true,
				},
				Messages: []slack.Message{
					{
						Msg: slack.Msg{
							Channel: mpimID,
							User:    "U1029384756", // Bot user
							Text:    fmt.Sprintf("Hi %s %s :wave:", p.Participant, p.Partner),
							Type:    "message",
						},
					},
				},
			}
		case "skipped":
			r = &slack.GetConversationHistoryResponse{
				SlackResponse: slack.SlackResponse{
					Ok: true,
				},
				Messages: []slack.Message{
					{
						Msg: slack.Msg{
							Channel: mpimID,
							User:    p.Participant,
							Text:    "Hi!",
							Type:    "message",
						},
					},
					{
						Msg: slack.Msg{
							Channel: mpimID,
							User:    p.Partner,
							Text:    "Hello",
							Type:    "message",
						},
					},
				},
			}
		}
		json.NewEncoder(w).Encode(&r)
	})

	mux.HandleFunc("/chat.postMessage", func(w http.ResponseWriter, req *http.Request) {
		req.ParseForm()

		b := req.FormValue("blocks")

		if b == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var blocks slack.Blocks
		if err := json.Unmarshal([]byte(b), &blocks); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"ok":false}`))
		}

		r.Len(blocks.BlockSet, 4)

		w.Write([]byte(`{
			"ok": true,
			"channel": "G1111111111"
		}`))
	})

	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	url := fmt.Sprintf("%s/", httpServer.URL)
	client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(url))

	params := &KickoffPairParams{
		ChannelID:   channelID,
		MatchID:     1,
		Participant: p.Participant,
		Partner:     p.Partner,
	}

	s.Run("bot message", func() {
		testname = "bot message"

		err := KickoffPair(s.ctx, s.db, client, params)
		r.NoError(err)
		r.Contains(s.buffer.String(), "retrieved chat history from the Slack Group DM")
		r.Contains(s.buffer.String(), "messages=1")
	})

	s.Run("skipped", func() {
		testname = "skipped"

		err := KickoffPair(s.ctx, s.db, client, params)
		r.NoError(err)
		r.Contains(s.buffer.String(), "retrieved chat history from the Slack Group DM")
		r.Contains(s.buffer.String(), "messages=2")
		r.Contains(s.buffer.String(), "skipping sending message")
	})
}

func (s *KickoffPairSuite) Test_QueueKickoffPairJob() {
	r := require.New(s.T())

	db, mock := database.NewMockedGormDB()

	p := &KickoffPairParams{
		ChannelID:   "C0123456789",
		MatchID:     1,
		Participant: "U0111111111",
		Partner:     "U2022222222",
	}

	database.MockQueueJob(
		mock,
		p,
		models.JobTypeKickoffPair.String(),
		models.JobPriorityLow,
	)

	err := QueueKickoffPairJob(s.ctx, db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")

	r.NoError(mock.ExpectationsWereMet())
}

func Test_KickoffPair_suite(t *testing.T) {
	suite.Run(t, new(KickoffPairSuite))
}
