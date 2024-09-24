package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/sebdah/goldie/v2"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slacktest"
	"github.com/stretchr/testify/assert"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
)

func Test_CheckPair_Success(t *testing.T) {
	slackServer := slacktest.NewTestServer()
	go slackServer.Start()
	defer slackServer.Stop()

	p := &CheckPairParams{
		ChannelID:   "G0123456789",
		Participant: "U0123456789",
		Partner:     "U9876543210",
		MpimID:      "C0123456789",
	}

	client := slack.New("xoxb-test-token", slack.OptionAPIURL(slackServer.GetAPIURL()))

	err := CheckPair(context.Background(), nil, client, p)
	assert.Nil(t, err)
}

func Test_CheckPair_Failure(t *testing.T) {
	logger, out := o11y.NewBufferedLogger()

	ctx := hclog.WithContext(context.Background(), logger)

	p := &CheckPairParams{
		ChannelID:   "G0123456789",
		Participant: "U0123456789",
		Partner:     "U9876543210",
		MpimID:      "C0123456789",
		MatchID:     99,
	}

	client := slack.New("xoxb-invalid-slack-authtoken")

	err := CheckPair(ctx, nil, client, p)
	assert.NotNil(t, err)
	assert.Contains(t, out.String(), "failed to send Slack group message:")
	assert.Contains(t, out.String(), "match_id=99")
	assert.Contains(t, out.String(), "slack_channel_id=G0123456789")
}

func Test_checkPairTemplate(t *testing.T) {
	g := goldie.New(t)

	data := checkPairTemplate{
		Participant: "U0123456789",
		Partner:     "U9876543210",
		MatchID:     int32(99),
		Responder:   "U9876543210",
	}

	t.Run("default", func(t *testing.T) {
		content, err := renderTemplate(checkPairTemplateFilename, data)
		assert.Nil(t, err)

		g.Assert(t, "check_pair.json", []byte(content))

	})

	t.Run("HasMet", func(t *testing.T) {
		data.HasMet = true

		content, err := renderTemplate(checkPairResponseTemplateFilename, data)
		assert.Nil(t, err)

		g.Assert(t, "check_pair_response.json", []byte(content))
	})
}

func Test_checkPairButtonValue(t *testing.T) {
	participant := "U0123456789"
	partner := "U9876543210"
	matchID := int32(99)

	t.Run("encode", func(t *testing.T) {
		assert.NotPanics(t, func() {
			v := checkPairButtonValue{
				MatchID:     matchID,
				HasMet:      true,
				Participant: participant,
				Partner:     partner,
			}

			s := v.Encode()

			assert.Contains(t, s, "match_id")
			assert.Contains(t, s, "has_met")
			assert.Contains(t, s, "participant")
			assert.Contains(t, s, "partner")
		})
	})

	t.Run("decode", func(t *testing.T) {
		assert.NotPanics(t, func() {
			var v checkPairButtonValue

			s := fmt.Sprintf(
				`{"match_id":%d,"has_met":true,"participant":"%s","partner":"%s"}`,
				matchID,
				participant,
				partner)

			v.Decode(s)

			assert.True(t, v.HasMet)
		})
	})
}

func Test_HandleCheckPairButtons(t *testing.T) {
	raw := []byte(`
{
    "user": {
        "id": "U0123456789",
        "username": "testuser",
        "name": "testuser",
        "team_id": "T0123456789"
    },
    "message": {
        "bot_id": "B0123456789",
        "type": "message",
        "user": "U0123456789",
        "team": "T0123456789",
        "blocks": [
            {
                "type": "section",
                "text": {
                    "type": "mrkdwn",
                    "text": ":wave <@U0123456789> <@U9876543210>\n\nTime for an end of round check-in!",
                    "verbatim": false
                }
            },
            {
                "type": "section",
                "text": {
                    "type": "mrkdwn",
                    "text": "*Did you get a chance to connect?*",
                    "verbatim": false
                }
            },
            {
                "type": "actions",
                "elements": [
                    {
                        "type": "button",
                        "action_id": "CHECK_PAIR|yes",
                        "text": {
                            "type": "plain_text",
                            "text": ":white_check_mark: Yes",
                            "emoji": true
                        },
                        "style": "primary",
                        "value": "true"
                    }
                ]
            }
        ]
    },
    "response_url": "REPLACE ME",
    "actions": [
        {
            "type": "button",
			"block_id": "Xd4ny",
            "action_id": "CHECK_PAIR|yes",
            "text": {
                "type": "plain_text",
                "text": ":white_check_mark: Yes",
                "emoji": true
            },
            "style": "primary",
            "value": "{\"match_id\":99,\"has_met\":true,\"participant\":\"U0123456789\",\"partner\":\"U9876543210\"}",
            "action_ts": "1638032136.985353"
        }
    ]
}
`)

	var interaction slack.InteractionCallback
	assert.Nil(t, interaction.UnmarshalJSON(raw))

	db, mock := database.NewMockedGormDB()

	p := &UpdateMatchParams{
		MatchID: 99,
		HasMet:  true,
	}

	database.MockQueueJob(mock, p, models.JobTypeUpdateMatch.String(), models.JobPriorityHigh)

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		var webhook *slack.WebhookMessage

		err := json.NewDecoder(r.Body).Decode(&webhook)
		assert.Nil(t, err)
		assert.Len(t, webhook.Blocks.BlockSet, 3)
		assert.True(t, webhook.ReplaceOriginal)

		// Assert that the response matches the right template
		data := checkPairTemplate{
			Participant: "U0123456789",
			Partner:     "U9876543210",
			MatchID:     int32(99),
			Responder:   "U0123456789",
			HasMet:      true,
		}

		content, err := renderTemplate(checkPairResponseTemplateFilename, data)
		assert.Nil(t, err)

		var view slack.View
		err = json.Unmarshal([]byte(content), &view)
		assert.Nil(t, err)

		assert.Equal(t, *webhook.Blocks, view.Blocks)
	}))

	defer server.Close()

	interaction.ResponseURL = server.URL

	err := HandleCheckPairButtons(context.Background(), http.DefaultClient, db, &interaction)
	assert.Nil(t, err)

	assert.Nil(t, mock.ExpectationsWereMet())
}
