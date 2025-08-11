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

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hashicorp/go-hclog"
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

type GreetAdminSuite struct {
	suite.Suite
	ctx    context.Context
	mock   sqlmock.Sqlmock
	db     *gorm.DB
	logger hclog.Logger
	buffer *bytes.Buffer
}

func (s *GreetAdminSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)
	s.db, s.mock = database.NewMockedGormDB()
}

func (s *GreetAdminSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *GreetAdminSuite) Test_GreetAdmin() {
	r := require.New(s.T())

	p := &GreetAdminParams{
		ChannelID: "C9876543210",
		Inviter:   "U8967452301",
	}

	// Mock Slack API calls
	mux := http.NewServeMux()

	mux.HandleFunc("/conversations.open", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(`{"ok":true,"channel":{"id":"D1111111111"}}`))
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

		r.Len(blocks.BlockSet, 6)

		w.Write([]byte(`{
			"ok": true,
			"channel": "D1111111111"
		}`))
	})

	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	url := fmt.Sprintf("%s/", httpServer.URL)
	client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(url))

	err := GreetAdmin(s.ctx, s.db, client, p)
	r.NoError(err)
}

func (s *GreetAdminSuite) Test_GreetAdminJob() {
	r := require.New(s.T())

	p := &GreetAdminParams{
		ChannelID: "C0123456789",
		Inviter:   "U1111111111",
	}

	database.MockQueueJob(
		s.mock,
		p,
		models.JobTypeGreetAdmin.String(),
		models.JobPriorityHigh,
	)

	err := QueueGreetAdminJob(s.ctx, s.db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")
}

func Test_GreetAdmin_suite(t *testing.T) {
	suite.Run(t, new(GreetAdminSuite))
}

func Test_greetAdminTemplate(t *testing.T) {
	g := goldie.New(t)

	p := greetAdminTemplate{
		ChannelID: "C0123456789",
		UserID:    "U9876543210",
	}

	content, err := renderTemplate(greetAdminTemplateFilename, p)
	assert.Nil(t, err)

	g.Assert(t, "greet_admin.json", []byte(content))
}

func Test_HandleGreetAdminButton(t *testing.T) {
	raw := []byte(`
{
    "type": "block_actions",
    "user": {
        "id": "U0123456789",
        "username": "testuser",
        "name": "testuser",
        "team_id": "T0123456789"
    },
    "trigger_id": "a.b.cd",
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
                    "text": "Hi <@U9876543210> :wave:"
                }
            },
            {
                "type": "section",
                "text": {
                    "type": "mrkdwn",
                    "text": "Thank you for inviting me to the <#C0123456789> channel :tada:"
                }
            },
            {
                "type": "section",
                "text": {
                    "type": "mrkdwn",
                    "text": "I'm here to help your Slack community stay connected by introducing members of <#C0123456789> to each other on a regular cadence.",
                    "verbatim": false
                }
            },
            {
                "type": "section",
                "text": {
                    "type": "mrkdwn",
                    "text": "Before we can begin our first round of Chat Roulette, we'll need to complete setup!",
                    "verbatim": false
                }
            },
            {
                "type": "divider"
            },
            {
                "type": "section",
                "text": {
                    "type": "mrkdwn",
                    "text": "*Click the following button to enable Chat Roulette for this channel:*"
                },
                "accessory": {
                    "type": "button",
                    "text": {
                        "type": "plain_text",
                        "text": "Let's Go!",
                        "emoji": true
                    },
                    "value": "C0123456789",
                    "action_id": "GREET_ADMIN|confirm"
                }
            }
        ]
    },
    "response_url": "REPLACE ME",
    "actions": [
        {
            "action_id": "GREET_MEMBER|confirm",
            "block_id": "Q5maS",
            "text": {
                "type": "plain_text",
                "text": "Opt In",
                "emoji": true
            },
            "value": "C0123456789",
            "type": "button",
            "action_ts": "1652742438.946792"
        }
    ]
}
`)

	var interaction slack.InteractionCallback
	assert.Nil(t, interaction.UnmarshalJSON(raw))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type openViewRequest struct {
			TriggerID string                 `json:"trigger_id"`
			View      slack.ModalViewRequest `json:"view"`
		}

		var request *openViewRequest
		err := json.NewDecoder(r.Body).Decode(&request)
		assert.Nil(t, err)

		// Verify private_metadata is base64 encoded
		assert.NotNil(t, request.View.PrivateMetadata)
		var pm privateMetadata
		assert.Nil(t, pm.Decode(request.View.PrivateMetadata))

		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	url := fmt.Sprintf("%s/", server.URL)
	client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(url))

	interaction.ResponseURL = url

	err := HandleGreetAdminButton(context.Background(), client, &interaction)
	assert.Nil(t, err)
}

func Test_RenderOnboardingChannelView(t *testing.T) {
	g := goldie.New(t)

	interaction := &slack.InteractionCallback{
		User: slack.User{
			ID: "U0123456789",
		},
		View: slack.View{
			PrivateMetadata: `eyJjaGFubmVsX2lkIjoiQzAxMjM0NTY3ODkiLCJyZXNwb25zZV91cmwiOiJodHRwOi8vbG9jYWxo
b3N0L2FjdGlvbnMvYS9iL2MifQ==`,
		},
	}

	content, err := RenderOnboardingChannelView(context.Background(), interaction, "http://localhost/")
	assert.Nil(t, err)
	assert.NotNil(t, content)

	g.Assert(t, "onboarding_channel.json", content)
}

func Test_UpsertChannelSettings(t *testing.T) {
	inviter := "U0123456789"
	connectionMode := models.HybridConnectionMode
	interval := models.Biweekly
	firstRound := time.Date(2024, time.January, 4, 10, 0, 0, 0, time.UTC)

	interaction := &slack.InteractionCallback{
		User: slack.User{
			ID: inviter,
		},
		View: slack.View{
			PrivateMetadata: `eyJjaGFubmVsX2lkIjoiQzAxMjM0NTY3ODkiLCJyZXNwb25zZV91cmwiOiJodHRwOi8vbG9jYWxo
b3N0L2FjdGlvbnMvYS9iL2MifQ==`,
			State: &slack.ViewState{
				Values: map[string]map[string]slack.BlockAction{
					"onboarding-channel-connection-mode": {
						"onboarding-channel-connection-mode": slack.BlockAction{
							SelectedOption: slack.OptionBlockObject{
								Value: connectionMode.String(),
							},
						},
					},
					"onboarding-channel-interval": {
						"onboarding-channel-interval": slack.BlockAction{
							SelectedOption: slack.OptionBlockObject{
								Value: interval.String(),
							},
						},
					},
					"onboarding-channel-datetime": {
						"onboarding-channel-datetime": slack.BlockAction{
							SelectedDateTime: 1704362400,
						},
					},
				},
			},
		},
	}

	db, mock := database.NewMockedGormDB()

	database.MockQueueJob(
		mock,
		&AddChannelParams{
			ChannelID:      "C0123456789",
			Inviter:        inviter,
			ConnectionMode: connectionMode.String(),
			Interval:       interval.String(),
			Weekday:        time.Thursday.String(),
			Hour:           10,
			NextRound:      firstRound,
		},
		models.JobTypeAddChannel.String(),
		models.JobPriorityHighest,
	)

	err := UpsertChannelSettings(context.Background(), db, interaction)
	require.Nil(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func Test_RespondGreetAdminWebhook(t *testing.T) {
	userID := "U0123456789"
	channelID := "C9876543210"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header["Content-Type"][0])

		var request *slack.WebhookMessage
		err := json.NewDecoder(r.Body).Decode(&request)
		assert.Nil(t, err)
		assert.Len(t, request.Blocks.BlockSet, 7)

		sectionBlock := request.Blocks.BlockSet[5].(*slack.SectionBlock)
		assert.Contains(t, sectionBlock.Text.Text, "Chat Roulette is now enabled!")

		contextBlock := request.Blocks.BlockSet[6].(*slack.ContextBlock)
		text := contextBlock.ContextElements.Elements[0].(*slack.TextBlockObject).Text
		assert.Contains(t, text, "visit me in")
		assert.Contains(t, text, "slack://app?id=A1234567890&tab=home&team=T1234567890")

		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	msg := slack.NewBlockMessage(
		slack.NewDividerBlock(),
		slack.NewDividerBlock(),
		slack.NewDividerBlock(),
		slack.NewDividerBlock(),
		slack.NewDividerBlock(),
		slack.NewDividerBlock(),
		slack.NewDividerBlock(),
	)

	pm := &privateMetadata{
		ChannelID:   channelID,
		Blocks:      msg.Blocks,
		ResponseURL: server.URL,
	}

	s, err := pm.Encode()
	assert.Nil(t, err)

	interaction := &slack.InteractionCallback{
		APIAppID: "A1234567890",
		Team: slack.Team{
			ID: "T1234567890",
		},
		User: slack.User{
			ID: userID,
		},
		View: slack.View{
			PrivateMetadata: s,
		},
	}

	err = RespondGreetAdminWebhook(context.Background(), &http.Client{}, interaction)
	assert.Nil(t, err)
}
