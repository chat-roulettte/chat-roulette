package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bincyber/go-sqlcrypter"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/require"

	"github.com/chat-roulettte/chat-roulette/internal/bot"
	"github.com/chat-roulettte/chat-roulette/internal/config"
	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/server"
)

func Test_slackInteractionHandler_BlockAction(t *testing.T) {
	r := require.New(t)

	opts := &server.ServerOptions{
		DevMode: true,
	}

	srv := server.NewTestServer(opts)
	s := &implServer{srv}

	path := "/v1/slack/interactions"

	var interaction slack.InteractionCallback

	raw := []byte(`
{
  "type": "block_actions",
  "user": {
    "id": "U0123456789",
    "username": "test-user",
    "team_id": "T0123456789"
  },
  "api_app_id": "A0123456789",
  "team": {
    "id": "T0123456789",
    "domain": "test-slack-workspace"
  },
  "channel": {
    "id": "C0123456789",
    "name": "privategroup"
  },
  "message": {
    "bot_id": "B0123456789",
    "type": "message",
    "user": "U0123456789",
    "ts": "1634824059.000100",
    "team": "T0123456789",
    "blocks": [
      {
        "type": "section",
        "block_id": "yvu",
        "text": {
          "type": "mrkdwn",
          "text": "Hello <@U0123456789> :wave:",
          "verbatim": false
        }
      },
      {
        "type": "actions",
        "block_id": "cuNvH",
        "elements": [
          {
            "type": "button",
            "action_id": "GREET_MEMBER|true",
            "text": {
              "type": "plain_text",
              "text": ":white_check_mark: Confirm",
              "emoji": true
            },
            "style": "primary",
            "value": "true"
          }
        ]
      }
    ]
  },
  "state": {
    "values": {}
  },
  "response_url": "https://hooks.slack.com/actions/T0123456789/X/Y",
  "actions": [
    {
      "action_id": "ADD_MEMBER|true",
      "block_id": "cuNvH",
      "text": {
        "type": "plain_text",
        "text": ":white_check_mark: Confirm",
        "emoji": true
      },
      "value": "true",
      "style": "danger",
      "type": "button",
      "action_ts": "1634905520.707421"
    }
  ]
}
`)
	err := json.Unmarshal(raw, &interaction)
	r.NoError(err)

	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(interaction)

	d := url.Values{
		"payload": []string{string(raw)},
	}

	req, _ := http.NewRequest(http.MethodPost, path, strings.NewReader(d.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp := httptest.NewRecorder()

	server := http.NewServeMux()
	server.Handle(path, http.HandlerFunc(s.slackInteractionHandler))
	server.ServeHTTP(resp, req)

	r.Equal(http.StatusOK, resp.Code)
}

func Test_slackInteractionHandler_ViewSubmission(t *testing.T) {
	r := require.New(t)

	opts := &server.ServerOptions{
		DevMode: true,
		Config: &config.Config{
			Server: config.ServerConfig{
				RedirectURL: "http://localhost/oauth/callback",
			},
		},
	}

	srv := server.NewTestServer(opts)
	s := &implServer{srv}

	path := "/v1/slack/interactions"

	var interaction slack.InteractionCallback

	raw := []byte(`
{
  "type": "view_submission",
  "user": {
    "id": "U0123456789",
    "username": "test-user",
    "team_id": "T0123456789"
  },
  "api_app_id": "A0123456789",
  "team": {
    "id": "T0123456789",
    "domain": "test-slack-workspace"
  },
  "trigger_id": "a.b.cd",
  "view": {
    "id": "V0123456789",
    "callback_id": "onboarding-member-modal",
    "type": "modal",
    "blocks": [],
    "private_metadata": "base64-encoded-blob-here",
    "clear_on_close": true,
    "notify_on_close": false,
    "close": {
        "type": "plain_text",
        "text": "Cancel",
        "emoji": true
    },
    "submit": {
        "type": "plain_text",
        "text": "Next",
        "emoji": true
    },
    "previous_view_id": null
  },
  "response_urls": []
}
`)
	err := json.Unmarshal(raw, &interaction)
	r.NoError(err)

	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(interaction)

	d := url.Values{
		"payload": []string{string(raw)},
	}

	req, _ := http.NewRequest(http.MethodPost, path, strings.NewReader(d.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp := httptest.NewRecorder()

	server := http.NewServeMux()
	server.Handle(path, http.HandlerFunc(s.slackInteractionHandler))
	server.ServeHTTP(resp, req)

	r.Equal(http.StatusOK, resp.Code)
	r.Equal("application/json", resp.Result().Header["Content-Type"][0])
	r.NotNil(resp.Body)
}

func Test_slackInteractionHandler_ViewSubmission_OnboardingTimezone(t *testing.T) {
	r := require.New(t)

	channelID := "C0123456789"

	sqlcrypter.Init(database.NoOpCrypter{})

	db, mock := database.NewMockedGormDB()

	database.MockQueueJob(
		mock,
		&bot.UpdateMemberParams{
			ChannelID: channelID,
			UserID:    "U0123456789",
			Timezone:  sqlcrypter.NewEncryptedBytes("America/Vancouver"),
		},
		models.JobTypeUpdateMember.String(),
		models.JobPriorityHigh,
	)

	mock.ExpectQuery(`SELECT "connection_mode" FROM "channels" WHERE channel_id = \$1`).
		WithArgs(
			channelID,
		).
		WillReturnRows(sqlmock.NewRows([]string{"connection_mode"}).AddRow(models.ConnectionModeHybrid.String()))

	opts := &server.ServerOptions{
		DevMode: true,
		DB:      db,
		Config: &config.Config{
			Server: config.ServerConfig{
				RedirectURL: "http://localhost/oauth/callback",
			},
		},
	}

	srv := server.NewTestServer(opts)
	s := &implServer{srv}

	path := "/v1/slack/interactions"

	var interaction slack.InteractionCallback

	raw := []byte(`
{
  "type": "view_submission",
  "user": {
    "id": "U0123456789",
    "username": "test-user",
    "team_id": "T0123456789"
  },
  "api_app_id": "A0123456789",
  "team": {
    "id": "T0123456789",
    "domain": "test-slack-workspace"
  },
  "trigger_id": "x.y.z",
  "view": {
    "id": "V0123456789",
    "callback_id": "onboarding-timezone",
    "type": "modal",
    "blocks": [],
    "private_metadata": "eyJjaGFubmVsX2lkIjoiQzAxMjM0NTY3ODkiLCJyZXNwb25zZV91cmwiOiJodHRwOi8vbG9jYWxob3N0L2FjdGlvbnMvYS9iL2MifQ==",
    "state": {
        "values": {
            "onboarding-timezone": {
                "onboarding-timezone": {
                    "type": "static_select",
                    "selected_option": {
                        "text": {
                            "type": "plain_text",
                            "text": "America/Vancouver",
                            "emoji": false
                        },
                        "value": "America/Vancouver"
                    }
                }
            }
        }
    },
    "clear_on_close": true,
    "notify_on_close": false,
    "close": {
        "type": "plain_text",
        "text": "Cancel",
        "emoji": true
    },
    "submit": {
        "type": "plain_text",
        "text": "Next",
        "emoji": true
    },
    "previous_view_id": null
  },
  "response_urls": []
}
`)
	err := json.Unmarshal(raw, &interaction)
	r.NoError(err)

	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(interaction)

	d := url.Values{
		"payload": []string{string(raw)},
	}

	req, _ := http.NewRequest(http.MethodPost, path, strings.NewReader(d.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp := httptest.NewRecorder()

	server := http.NewServeMux()
	server.Handle(path, http.HandlerFunc(s.slackInteractionHandler))
	server.ServeHTTP(resp, req)

	r.Equal(http.StatusOK, resp.Code)
	r.Equal("application/json", resp.Result().Header["Content-Type"][0])
	r.Contains(resp.Body.String(), "your preferred connection mode")
	r.NoError(mock.ExpectationsWereMet())
}
