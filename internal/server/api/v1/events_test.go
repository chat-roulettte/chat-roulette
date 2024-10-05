package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hashicorp/go-hclog"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/stretchr/testify/require"

	"github.com/chat-roulettte/chat-roulette/internal/bot"
	"github.com/chat-roulettte/chat-roulette/internal/config"
	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
	"github.com/chat-roulettte/chat-roulette/internal/server"
)

func Test_slackEventHandler_UrlVerificationEvent_Success(t *testing.T) {
	r := require.New(t)

	opts := &server.ServerOptions{
		DevMode: true,
	}

	srv := server.NewTestServer(opts)
	s := &implServer{srv}

	path := "/v1/slack/events"

	challenge := "3eZbrw1aBm2rZgRNFdxV2595E9CY3gmdALWMmHkvFXO7tYXAYM8P"

	event := &slackevents.EventsAPIURLVerificationEvent{
		Type:      "url_verification",
		Challenge: challenge,
	}

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(event)

	req, _ := http.NewRequest(http.MethodPost, path, body)
	resp := httptest.NewRecorder()

	server := http.NewServeMux()
	server.Handle(path, http.HandlerFunc(s.slackEventHandler))
	server.ServeHTTP(resp, req)

	r.Equal(http.StatusOK, resp.Code)
	r.Equal(challenge, resp.Body.String())
}

func Test_slackEventHandler_ParseEvent_Failure(t *testing.T) {
	r := require.New(t)

	logger, out := o11y.NewBufferedLogger()

	opts := &server.ServerOptions{
		DevMode: true,
	}

	srv := server.NewTestServer(opts)
	s := &implServer{srv}

	path := "/v1/slack/events"

	req, _ := http.NewRequest(http.MethodPost, path, bytes.NewBufferString("invalid request"))
	req = req.WithContext(hclog.WithContext(req.Context(), logger))

	resp := httptest.NewRecorder()

	server := http.NewServeMux()
	server.Handle(path, http.HandlerFunc(s.slackEventHandler))
	server.ServeHTTP(resp, req)

	r.Equal(http.StatusBadRequest, resp.Code)
	r.Contains(out.String(), "failed to parse Slack event")
	r.Contains(out.String(), "error=")
}

func Test_slackEventHandler_BotJoinedChannelEvent(t *testing.T) {
	r := require.New(t)

	// Setup DB for this test
	resource, databaseURL, err := database.NewTestPostgresDB(false)
	r.NoError(err)
	defer resource.Close()

	if err := database.Migrate(databaseURL); err != nil {
		r.NoError(err)
	}

	db, err := database.NewGormDB(databaseURL)
	r.NoError(err)

	channelID := "C9876543210"
	userID := "U1111111111"
	inviter := "U2222222222"

	opts := &server.ServerOptions{
		DB:             db,
		DevMode:        true,
		SlackBotUserID: userID,
	}

	srv := server.NewTestServer(opts)
	s := &implServer{srv}

	path := "/v1/slack/events"

	var event slackevents.EventsAPICallbackEvent

	rawE := []byte(fmt.Sprintf(`
{
	"token": "XXYYZZ",
	"team_id": "TXXXXXXXX",
	"api_app_id": "AXXXXXXXXX",
	"event": {
		"type": "member_joined_channel",
		"channel": %q,
		"user": %q,
		"channel_type": "C",
		"team": "T1928374560",
		"inviter": %q
	},
	"type": "event_callback",
	"authed_users": [ "UXXXXXXX1" ],
	"event_id": "Ev08MFMKH6",
	"event_time": 1234567890
}
`, channelID, userID, inviter))

	r.NoError(json.Unmarshal(rawE, &event))

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(event)

	req, _ := http.NewRequest(http.MethodPost, path, body)
	resp := httptest.NewRecorder()

	server := http.NewServeMux()
	server.Handle(path, http.HandlerFunc(s.slackEventHandler))
	server.ServeHTTP(resp, req)

	r.Equal(http.StatusOK, resp.Code)

	// Validate exactly one row was added to the database
	var count int64
	db.Model(&models.Job{}).Count(&count)
	r.Equal(int64(1), count)

	// Validate the contents of the single row added
	var job models.Job
	r.NoError(db.First(&job).Error)
	r.Equal(job.JobType, models.JobTypeGreetAdmin)
}

func Test_slackEventHandler_BotJoinedChannelEvent_failure(t *testing.T) {
	r := require.New(t)

	db, mock := database.NewMockedGormDB()
	channelID := "C9876543210"
	userID := "U1111111111"
	inviter := "U2222222222"

	opts := &server.ServerOptions{
		DB:             db,
		DevMode:        true,
		SlackBotUserID: userID,
	}

	srv := server.NewTestServer(opts)
	s := &implServer{srv}

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "jobs" (.+) VALUES (.+) RETURNING`).
		WithArgs(
			sqlmock.AnyArg(),
			models.JobTypeGreetAdmin.String(),
			models.JobPriorityHigh,
			models.JobStatusPending,
			false,
			sqlmock.AnyArg(),
			database.AnyTime(),
			database.AnyTime(),
			database.AnyTime(),
		).
		WillReturnError(fmt.Errorf("failed to add job to the queue"))
	mock.ExpectRollback()

	path := "/v1/slack/events"

	var event slackevents.EventsAPICallbackEvent

	rawE := []byte(fmt.Sprintf(`
{
	"event": {
		"type": "member_joined_channel",
		"channel": %q,
		"user": %q,
		"channel_type": "C",
		"team": "T1928374560",
		"inviter": %q
	},
	"type": "event_callback"
}
`, channelID, userID, inviter))

	err := json.Unmarshal(rawE, &event)
	r.NoError(err)

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(event)

	req, _ := http.NewRequest(http.MethodPost, path, body)
	resp := httptest.NewRecorder()

	server := http.NewServeMux()
	server.Handle(path, http.HandlerFunc(s.slackEventHandler))
	server.ServeHTTP(resp, req)

	r.Equal(http.StatusServiceUnavailable, resp.Code)

	r.Nil(mock.ExpectationsWereMet())
}

func Test_slackEventHandler_MemberJoinedChannelEvent(t *testing.T) {
	r := require.New(t)

	db, mock := database.NewMockedGormDB()

	opts := &server.ServerOptions{
		DB:      db,
		DevMode: true,
	}

	srv := server.NewTestServer(opts)
	s := &implServer{srv}

	channelID := "C9876543210"
	userID := "U0123456789"

	// Mock Slack channel lookup
	mock.ExpectQuery(`SELECT "channel_id" FROM "channels" WHERE channel_id =`).
		WithArgs(
			channelID,
			1,
		).
		WillReturnRows(sqlmock.NewRows([]string{"channel_id"}).AddRow(channelID))

	// Mock adding the ADD_MEMBER job
	p := &bot.AddMemberParams{
		ChannelID: channelID,
		UserID:    userID,
	}

	database.MockQueueJob(mock, p, models.JobTypeAddMember.String(), models.JobPriorityHigh)

	path := "/v1/slack/events"

	var event slackevents.EventsAPICallbackEvent

	rawE := []byte(fmt.Sprintf(`
{
	"token": "XXYYZZ",
	"team_id": "TXXXXXXXX",
	"api_app_id": "AXXXXXXXXX",
	"event": {
		"type": "member_joined_channel",
		"channel": %q,
		"user": %q,
		"channel_type": "C",
		"team": "T1928374560",
		"inviter": "U0111111111"
	},
	"type": "event_callback",
	"authed_users": [ "UXXXXXXX1" ],
	"event_id": "Ev08MFMKH6",
	"event_time": 1234567890
}
`, channelID, userID))

	err := json.Unmarshal(rawE, &event)
	r.NoError(err)

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(event)

	req, _ := http.NewRequest(http.MethodPost, path, body)
	resp := httptest.NewRecorder()

	server := http.NewServeMux()
	server.Handle(path, http.HandlerFunc(s.slackEventHandler))
	server.ServeHTTP(resp, req)

	r.Equal(http.StatusOK, resp.Code)

	r.Nil(mock.ExpectationsWereMet())
}

func Test_slackEventHandler_MemberLeftChannelEvent(t *testing.T) {
	r := require.New(t)

	db, mock := database.NewMockedGormDB()

	opts := &server.ServerOptions{
		DB:      db,
		DevMode: true,
	}

	srv := server.NewTestServer(opts)
	s := &implServer{srv}

	p := &bot.DeleteMemberParams{
		ChannelID: "C9876543210",
		UserID:    "U0123456789",
	}

	// Mock Slack channel lookup
	mock.ExpectQuery(`SELECT "channel_id" FROM "channels" WHERE channel_id =`).
		WithArgs(
			p.ChannelID,
			1,
		).
		WillReturnRows(sqlmock.NewRows([]string{"channel_id"}).AddRow(p.ChannelID))

	// Mock adding the DELETE_MEMBER job
	database.MockQueueJob(mock, p, models.JobTypeDeleteMember.String(), models.JobPriorityHigh)

	path := "/v1/slack/events"

	var event slackevents.EventsAPICallbackEvent

	rawE := []byte(`
{
	"token": "XXYYZZ",
	"team_id": "TXXXXXXXX",
	"api_app_id": "AXXXXXXXXX",
	"event": {
		"type": "member_left_channel",
		"user": "U0123456789",
		"channel": "C9876543210",
		"channel_type": "C",
		"team": "T024BE7LD"
	},
	"type": "event_callback",
	"authed_users": [ "UXXXXXXX1" ],
	"event_id": "Ev08MFMKH6",
	"event_time": 1234567890
}
`)
	err := json.Unmarshal(rawE, &event)
	r.NoError(err)

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(event)

	req, _ := http.NewRequest(http.MethodPost, path, body)
	resp := httptest.NewRecorder()

	server := http.NewServeMux()
	server.Handle(path, http.HandlerFunc(s.slackEventHandler))
	server.ServeHTTP(resp, req)

	r.Equal(http.StatusOK, resp.Code)

	r.NoError(mock.ExpectationsWereMet())
}

func Test_slackEventHandler_AppHomeOpened_failure(t *testing.T) {
	r := require.New(t)

	logger, out := o11y.NewBufferedLogger()

	db, _ := database.NewMockedGormDB()

	opts := &server.ServerOptions{
		DB:             db,
		DevMode:        true,
		SlackClient:    slack.New("invalid-token"),
		SlackBotUserID: "U1111111111",
		Config: &config.Config{
			Server: config.ServerConfig{
				RedirectURL: "http://localhost:8888/oidc/callback",
			},
		},
	}

	srv := server.NewTestServer(opts)
	s := &implServer{srv}

	path := "/v1/slack/events"

	var event slackevents.EventsAPICallbackEvent

	rawE := []byte(`
{
	"token": "XXYYZZ",
	"team_id": "TXXXXXXXX",
	"api_app_id": "AXXXXXXXXX",
	"event": {
		"type": "app_home_opened",
		"user": "U0123456789",
		"channel": "C9876543210",
		"tab": "home",
		"view": {
			"type": "home",
			"blocks": []
		}
	},
	"type": "event_callback",
	"authed_users": [ "UXXXXXXX1" ],
	"event_id": "Ev08MFMKH6",
	"event_time": 1234567890
}
`)
	err := json.Unmarshal(rawE, &event)
	r.NoError(err)

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(event)

	req, _ := http.NewRequest(http.MethodPost, path, body)
	req = req.WithContext(hclog.WithContext(req.Context(), logger))
	resp := httptest.NewRecorder()

	server := http.NewServeMux()
	server.Handle(path, http.HandlerFunc(s.slackEventHandler))
	server.ServeHTTP(resp, req)

	r.Equal(http.StatusInternalServerError, resp.Code)
	r.Contains(out.String(), "failed to handle app_home_opened event")
}
