package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/require"

	"github.com/chat-roulettte/chat-roulette/internal/server"
)

func Test_slackOptionsHandler(t *testing.T) {
	r := require.New(t)

	opts := &server.ServerOptions{
		DevMode: true,
	}

	srv := server.NewTestServer(opts)
	s := &implServer{srv}

	path := "/v1/slack/options"

	var interaction slack.InteractionCallback

	raw := []byte(`
{
  "type": "block_suggestion",
  "action_id": "onboarding-location-country",
  "block_id": "onboarding-country",
  "value": "zam",
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
  "view": {
        "id": "V0123456789",
        "team_id": "T0123456789",
        "type": "modal",
        "blocks": [],
		"private_metadata": "",
        "callback_id": "onboarding-location",
        "state": {
            "values": {}
        },
        "title": {
            "type": "plain_text",
            "text": "Chat Roulette",
            "emoji": true
        },
        "clear_on_close": false,
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
        }
  }
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
	server.Handle(path, http.HandlerFunc(s.slackOptionsHandler))
	server.ServeHTTP(resp, req)

	r.Equal(http.StatusOK, resp.Code)

	var optionsResponse *slack.OptionsResponse
	err = json.NewDecoder(resp.Body).Decode(&optionsResponse)
	r.NoError(err)
	r.Equal("application/json", resp.Result().Header["Content-Type"][0])
	r.Len(optionsResponse.Options, 1)
	r.Equal("Zambia", optionsResponse.Options[0].Value)
}
