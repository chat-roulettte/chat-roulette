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
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
)

func Test_markInactiveTemplate(t *testing.T) {
	g := goldie.New(t)

	nextRound := time.Date(2022, time.January, 3, 12, 0, 0, 0, time.UTC)

	p := markInactiveTemplate{
		ChannelID: "C0123456789",
		UserID:    "U9876543210",
		NextRound: nextRound,
		AppHome:   "slack://app?id=A1234567890&tab=home&team=T1234567890",
	}

	content, err := renderTemplate(markInactiveTemplateFilename, p)
	assert.Nil(t, err)

	g.Assert(t, "mark_inactive.json", []byte(content))
}

type MarkInactiveSuite struct {
	suite.Suite
	ctx        context.Context
	db         *gorm.DB
	logger     hclog.Logger
	buffer     *bytes.Buffer
	resource   *dockertest.Resource
	httpServer *httptest.Server
	client     *slack.Client
	params     *MarkInactiveParams
	testname   string
}

func (s *MarkInactiveSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)

	resource, databaseURL, err := database.NewTestPostgresDB(false)
	require.NoError(s.T(), err)
	s.resource = resource

	db, err := database.NewGormDB(databaseURL)
	require.NoError(s.T(), err)
	require.NoError(s.T(), database.Migrate(databaseURL))

	s.db = db

	channelID := "C0123456789"
	mpimID := "G1111111111"
	participant := "U0123456789"
	partner := "U8765432109"
	bot := "U1029384756"

	nextRound := time.Now().Add(48 * time.Hour) // 2 days in the future

	// Write channel to the database
	s.db.Create(&models.Channel{
		ChannelID:      channelID,
		Inviter:        "U9876543210",
		ConnectionMode: models.VirtualConnectionMode,
		Interval:       models.Biweekly,
		Weekday:        time.Friday,
		Hour:           12,
		NextRound:      nextRound,
	})

	// Write records in the rounds and matches table
	s.db.Create(&models.Round{
		ChannelID: channelID,
	})

	s.db.Create(&models.Match{
		RoundID:     1,
		MpimID:      mpimID,
		WasNotified: true,
		CreatedAt:   time.Now().Add(-168 * time.Hour),
	})

	// Write records in the members table
	isActive := true
	hasGenderPreference := false

	s.db.Create(&models.Member{
		ChannelID:           channelID,
		UserID:              participant,
		IsActive:            &isActive,
		Gender:              models.Female,
		HasGenderPreference: &hasGenderPreference,
	})
	s.db.Create(&models.Member{
		ChannelID:           channelID,
		UserID:              partner,
		IsActive:            &isActive,
		Gender:              models.Male,
		HasGenderPreference: &hasGenderPreference,
	})

	// Mock Slack API calls
	mux := http.NewServeMux()

	mux.HandleFunc("/conversations.history", func(w http.ResponseWriter, req *http.Request) {
		var resp *slack.GetConversationHistoryResponse

		switch s.testname {
		case "no messages exchanged":
			resp = &slack.GetConversationHistoryResponse{
				SlackResponse: slack.SlackResponse{Ok: true},
				Messages: []slack.Message{
					{
						Msg: slack.Msg{
							Channel: mpimID,
							User:    bot,
							Text:    fmt.Sprintf("Hi %s %s :wave:", participant, partner),
							Type:    "message",
						},
					},
					{
						Msg: slack.Msg{
							Channel: mpimID,
							User:    bot,
							Text:    "*Did you get a chance to connect?*",
							Type:    "message",
						},
					},
				},
			}
		case "one button clicked":
			resp = &slack.GetConversationHistoryResponse{
				SlackResponse: slack.SlackResponse{Ok: true},
				Messages: []slack.Message{
					{
						Msg: slack.Msg{
							Channel: mpimID,
							User:    bot,
							Text:    fmt.Sprintf("Hi <@%s> <@%s> :wave:", participant, partner),
							Type:    "message",
						},
					},
					{
						Msg: slack.Msg{
							Channel: mpimID,
							User:    partner,
							Text:    fmt.Sprintf("Hi <@%s>", participant), // participant will be marked as inactive
							Type:    "message",
						},
					},
					{
						Msg: slack.Msg{
							Channel: mpimID,
							User:    bot,
							Text:    fmt.Sprintf(":x: <@%s> said that you did not meet. I'm really sorry to hear that :sob:", participant),
							Type:    "message",
						},
					},
				},
			}
		case "no buttons clicked":
			resp = &slack.GetConversationHistoryResponse{
				SlackResponse: slack.SlackResponse{Ok: true},
				Messages: []slack.Message{
					{
						Msg: slack.Msg{
							Channel: mpimID,
							User:    bot,
							Text:    fmt.Sprintf("Hi <@%s> <@%s> :wave:", participant, partner),
							Type:    "message",
						},
					},
					{
						Msg: slack.Msg{
							Channel: mpimID,
							User:    bot,
							Text:    "*Did you get a chance to connect?*",
							Type:    "message",
						},
					},
					{
						Msg: slack.Msg{
							Channel: mpimID,
							User:    participant,
							Text:    fmt.Sprintf("Hi <@%s>", partner), // both will be marked as inactive
							Type:    "message",
						},
					},
					{
						Msg: slack.Msg{
							Channel: mpimID,
							User:    bot,
							Text:    "*Did you get a chance to connect?*",
							Type:    "message",
						},
					},
				},
			}
		case "messages exchanged":
			resp = &slack.GetConversationHistoryResponse{
				SlackResponse: slack.SlackResponse{Ok: true},
				Messages: []slack.Message{
					{
						Msg: slack.Msg{
							Channel: mpimID,
							User:    bot,
							Text:    fmt.Sprintf("Hi <@%s> <@%s> :wave:", participant, partner),
							Type:    "message",
						},
					},
					{
						Msg: slack.Msg{
							Channel: mpimID,
							User:    partner,
							Text:    fmt.Sprintf("Hi <@%s>", participant),
							Type:    "message",
						},
					},
					{
						Msg: slack.Msg{
							Channel: mpimID,
							User:    participant,
							Text:    "What's up",
							Type:    "message",
						},
					},
					{
						Msg: slack.Msg{
							Channel: mpimID,
							User:    bot,
							Text:    fmt.Sprintf(":white_check_mark: <@%s> said that you met! That's awesome :tada:", participant),
							Type:    "message",
						},
					},
				},
			}
		}
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/auth.test", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(`{"ok":true,"team_id":"T1111111111","user_id":"U1029384756"}`))
	})
	mux.HandleFunc("/bots.info", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(`{"ok":true,"app_id":"A1111111111","user_id":"U1029384756"}`))
	})
	mux.HandleFunc("/conversations.open", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(`{"ok":true,"channel":{"id":"G1111111111"}}`))
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

		require.Len(s.T(), blocks.BlockSet, 6)
		require.Contains(s.T(), b, "you have been marked as inactive")

		w.Write([]byte(`{
			"ok": true,
			"channel": "G1111111111"
		}`))
	})

	s.httpServer = httptest.NewServer(mux)

	url := s.httpServer.URL + "/"
	s.client = slack.New("xoxb-test-token", slack.OptionAPIURL(url))

	s.params = &MarkInactiveParams{
		ChannelID:    channelID,
		MatchID:      1,
		Participants: []string{participant, partner},
		NextRound:    nextRound,
	}
}

func (s *MarkInactiveSuite) AfterTest(_, _ string) {
	s.httpServer.Close()
}

func (s *MarkInactiveSuite) Test_MarkInactive_NoMessages() {
	r := require.New(s.T())

	s.testname = "no messages"

	err := MarkInactive(s.ctx, s.db, s.client, s.params)
	r.NoError(err)
	r.Contains(s.buffer.String(), "marked Slack user as inactive in the database")
	r.Contains(s.buffer.String(), "count=2")

	var inactiveCount int64
	s.db.Model(&models.Member{}).Where("channel_id = ? AND is_active = false", s.params.ChannelID).Count(&inactiveCount)
	r.Equal(inactiveCount, int64(2))

	var counter int16
	s.db.Model(&models.Round{}).Select("inactive_users").Where("id = ?", 1).First(&counter)
	r.Equal(counter, int16(2))
}

func (s *MarkInactiveSuite) Test_MarkInactive_OneButtonClicked() {
	r := require.New(s.T())

	s.testname = "one button clicked"

	err := MarkInactive(s.ctx, s.db, s.client, s.params)
	r.NoError(err)
	r.Contains(s.buffer.String(), "marked Slack user as inactive in the database")
	r.Contains(s.buffer.String(), "count=1")

	var status bool
	s.db.Model(&models.Member{}).Select("is_active").Where("channel_id = ? AND user_id = ?", s.params.ChannelID, s.params.Participants[0]).First(&status)
	r.False(status)

	var counter int16
	s.db.Model(&models.Round{}).Select("inactive_users").Where("id = ?", 1).First(&counter)
	r.Equal(counter, int16(1))
}

func (s *MarkInactiveSuite) Test_MarkInactive_NoButtonsClicked() {
	r := require.New(s.T())

	s.testname = "no buttons clicked"

	err := MarkInactive(s.ctx, s.db, s.client, s.params)
	r.NoError(err)
	r.Contains(s.buffer.String(), "marked Slack user as inactive in the database")
	r.Contains(s.buffer.String(), "count=2")

	var inactiveCount int64
	s.db.Model(&models.Member{}).Where("channel_id = ? AND is_active = false", s.params.ChannelID).Count(&inactiveCount)
	r.Equal(inactiveCount, int64(2))

	var counter int16
	s.db.Model(&models.Round{}).Select("inactive_users").Where("id = ?", 1).First(&counter)
	r.Equal(counter, int16(2))
}

func (s *MarkInactiveSuite) Test_MarkInactive_MessagesExchanged() {
	r := require.New(s.T())

	s.testname = "messages exchanged"

	err := MarkInactive(s.ctx, s.db, s.client, s.params)
	r.NoError(err)
	r.NotContains(s.buffer.String(), "marked Slack user as inactive in the database")

	var inactiveCount int64
	s.db.Model(&models.Member{}).Where("channel_id = ? AND is_active = true", s.params.ChannelID).Count(&inactiveCount)
	r.Equal(int64(2), inactiveCount)

	var counter int16
	s.db.Model(&models.Round{}).Select("inactive_users").Where("id = ?", 1).First(&counter)
	r.Equal(counter, int16(0))
}

func (s *MarkInactiveSuite) Test_QueueMarkInactiveJob() {
	r := require.New(s.T())

	db, mock := database.NewMockedGormDB()

	nextRound := time.Date(2022, time.January, 3, 12, 0, 0, 0, time.UTC)

	p := &MarkInactiveParams{
		ChannelID: "C0123456789",
		MatchID:   1,
		Participants: []string{
			"U0111111111",
			"U2022222222",
		},
		NextRound: nextRound,
	}

	database.MockQueueJob(
		mock,
		p,
		models.JobTypeMarkInactive.String(),
		models.JobPriorityHigh,
	)

	err := QueueMarkInactiveJob(s.ctx, db, p, nextRound.Add(-24*time.Hour))
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")
	r.NoError(mock.ExpectationsWereMet())
}

func Test_MarkInactive_suite(t *testing.T) {
	suite.Run(t, new(MarkInactiveSuite))
}
