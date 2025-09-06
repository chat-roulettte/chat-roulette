package bot

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bincyber/go-sqlcrypter"
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

type GreetMemberSuite struct {
	suite.Suite
	ctx    context.Context
	mock   sqlmock.Sqlmock
	db     *gorm.DB
	logger hclog.Logger
	buffer *bytes.Buffer
}

func (s *GreetMemberSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)
	s.db, s.mock = database.NewMockedGormDB()
}

func (s *GreetMemberSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *GreetMemberSuite) Test_GreetMember() {
	r := require.New(s.T())

	p := &GreetMemberParams{
		ChannelID: "C9876543210",
		UserID:    "U0123456789",
	}

	columns := []string{
		"channel_id",
		"inviter",
		"interval",
		"weekday",
		"hour",
		"next_round",
	}

	row := []driver.Value{
		p.ChannelID,
		"U8967452301",
		models.Weekly,
		time.Sunday,
		12,
		time.Now(),
	}

	// Mock retrieving channel metadata
	s.mock.ExpectQuery(`SELECT \* FROM "channels" WHERE channel_id = (.+) ORDER BY`).
		WithArgs(
			p.ChannelID,
			1,
		).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(row...))

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

		r.Len(blocks.BlockSet, 7)

		w.Write([]byte(`{
			"ok": true,
			"channel": "D1111111111"
		}`))
	})

	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	url := fmt.Sprintf("%s/", httpServer.URL)
	client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(url))

	err := GreetMember(s.ctx, s.db, client, p)
	r.NoError(err)
}

func (s *GreetMemberSuite) Test_GreetMemberJob() {
	r := require.New(s.T())

	p := &GreetMemberParams{
		ChannelID: "C0123456789",
		UserID:    "U1111111111",
	}

	database.MockQueueJob(
		s.mock,
		p,
		models.JobTypeGreetMember.String(),
		models.JobPriorityStandard,
	)

	err := QueueGreetMemberJob(s.ctx, s.db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")
}

func Test_GreetMember_suite(t *testing.T) {
	suite.Run(t, new(GreetMemberSuite))
}

func Test_greetMemberTemplate(t *testing.T) {
	g := goldie.New(t)

	nextRound := time.Date(2022, time.January, 3, 12, 0, 0, 0, time.UTC)

	p := greetMemberTemplate{
		ChannelID: "C0123456789",
		Inviter:   "U9876543210",
		UserID:    "U0123456789",
		NextRound: nextRound,
	}

	testCases := []struct {
		name           string
		connectionMode string
		interval       models.IntervalEnum
		goldenFile     string
	}{
		{"biweekly physical", models.ConnectionModePhysical.String(), models.Biweekly, "greet_member_biweekly.json"},
		{"monthly virtual", models.ConnectionModeVirtual.String(), models.Monthly, "greet_member_monthly.json"},
		{"weekly hybrid", models.ConnectionModeHybrid.String(), models.Weekly, "greet_member_hybrid.json"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p.ConnectionMode = tc.connectionMode
			p.When = formatSchedule(tc.interval, nextRound)

			content, err := renderTemplate(greetMemberTemplateFilename, p)
			assert.Nil(t, err)

			g.Assert(t, tc.goldenFile, []byte(content))
		})
	}
}

func Test_HandleGreetMemberButton(t *testing.T) {
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
	"message":{
        "bot_id":"B0123456789",
        "type":"message",
        "user":"U0123456789",
        "team":"T0123456789",
		"blocks": [
			{
				"type": "section",
				"text": {
					"type": "mrkdwn",
					"text": ":wave: Hello <@U0123456789>"
				}
			},
			{
				"type": "section",
				"text": {
					"type": "mrkdwn",
					"text": "Welcome to the <#C0123456789> channel!"
				}
			},
			{
				"type": "section",
				"text": {
					"type": "mrkdwn",
					"text": "Chat Roulette has been enabled on this channel. *Biweekly* on *Mondays*, you will be introduced to another member in the <#C0123456789> channel. You will have until the end of each round to meet in-person at a location of your choosing, whether it's for coffee or a meal!",
					"verbatim": false
				}
			},
			{
				"type": "section",
				"text": {
					"type": "mrkdwn",
					"text": "The next Chat Roulette round begins on *Monday, January 3rd, 2022*!"
				}
			},
			{
				"type": "section",
				"text": {
					"type": "mrkdwn",
					"text": "Chat Roulette has been enabled on this channel by <@U9876543210>, so if you have any questions, please reach out to them."
				}
			},
			{
				"type": "divider"
			},
			{
				"type": "section",
				"block_id": "Q5maS",
				"text": {
					"type": "mrkdwn",
					"text": "*To participate in Chat roulette, click the following button to complete onboarding:*"
				},
				"accessory": {
					"type": "button",
					"text": {
						"type": "plain_text",
						"text": "Opt In",
						"emoji": true
					},
					"value": "C0123456789",
					"action_id": "GREET_MEMBER|confirm"
				}
			}
		]
    },
    "response_url":"REPLACE ME",
    "actions":[{
        "action_id":"GREET_MEMBER|confirm",
        "block_id":"Q5maS",
        "text":{
            "type":"plain_text",
            "text":"Opt In",
            "emoji":true
        },
        "value":"C0123456789",
        "type":"button",
        "action_ts":"1652742438.946792"
    }]
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

	err := HandleGreetMemberButton(context.Background(), client, &interaction)
	assert.Nil(t, err)
}

func Test_RenderOnboardingLocationView(t *testing.T) {
	g := goldie.New(t)

	interaction := &slack.InteractionCallback{
		User: slack.User{
			ID: "U0123456789",
		},
		View: slack.View{
			PrivateMetadata: "base64-encoded-data-here",
		},
	}

	content, err := RenderOnboardingLocationView(context.Background(), interaction, "http://localhost/")
	assert.Nil(t, err)
	assert.NotNil(t, content)

	g.Assert(t, "onboarding_location.json", content)
}

func Test_UpsertMemberLocationInfo(t *testing.T) {
	userID := "U0123456789"
	country := "United States"
	city := "New York City"

	interaction := &slack.InteractionCallback{
		User: slack.User{
			ID: "U0123456789",
		},
		View: slack.View{
			PrivateMetadata: `eyJjaGFubmVsX2lkIjoiQzAxMjM0NTY3ODkiLCJyZXNwb25zZV91cmwiOiJodHRwOi8vbG9jYWxo
b3N0L2FjdGlvbnMvYS9iL2MifQ==`,
			State: &slack.ViewState{
				Values: map[string]map[string]slack.BlockAction{
					"onboarding-country": {
						"onboarding-location-country": slack.BlockAction{
							SelectedOption: slack.OptionBlockObject{
								Value: country,
							},
						},
					},
					"onboarding-city": {
						"onboarding-location-city": {
							Value: city,
						},
					},
				},
			},
		},
	}

	sqlcrypter.Init(database.NoOpCrypter{})

	db, mock := database.NewMockedGormDB()

	database.MockQueueJob(
		mock,
		&UpdateMemberParams{
			ChannelID: "C0123456789",
			UserID:    userID,
			Country:   sqlcrypter.NewEncryptedBytes(country),
			City:      sqlcrypter.NewEncryptedBytes(city),
		},
		models.JobTypeUpdateMember.String(),
		models.JobPriorityHigh,
	)

	err := UpsertMemberLocationInfo(context.Background(), db, interaction)
	assert.Nil(t, err)
}

func Test_RenderOnboardingTimezoneView(t *testing.T) {
	g := goldie.New(t)

	interaction := &slack.InteractionCallback{
		User: slack.User{
			ID: "U0123456789",
		},
		View: slack.View{
			PrivateMetadata: "base64-encoded-data-here",
			State: &slack.ViewState{
				Values: map[string]map[string]slack.BlockAction{
					"onboarding-country": {
						"onboarding-location-country": slack.BlockAction{
							SelectedOption: slack.OptionBlockObject{
								Value: "Malta",
							},
						},
					},
				},
			},
		},
	}

	content, err := RenderOnboardingTimezoneView(context.Background(), interaction, "http://localhost/")
	assert.Nil(t, err)
	assert.NotNil(t, content)

	g.Assert(t, "onboarding_timezone.json", content)
}

func Test_UpsertMemberTimezoneInfo(t *testing.T) {
	userID := "U0123456789"
	timezone := "Europe/Malta"

	interaction := &slack.InteractionCallback{
		User: slack.User{
			ID: "U0123456789",
		},
		View: slack.View{
			PrivateMetadata: `eyJjaGFubmVsX2lkIjoiQzAxMjM0NTY3ODkiLCJyZXNwb25zZV91cmwiOiJodHRwOi8vbG9jYWxo
b3N0L2FjdGlvbnMvYS9iL2MifQ==`,
			State: &slack.ViewState{
				Values: map[string]map[string]slack.BlockAction{
					"onboarding-timezone": {
						"onboarding-timezone": slack.BlockAction{
							SelectedOption: slack.OptionBlockObject{
								Value: timezone,
							},
						},
					},
				},
			},
		},
	}

	sqlcrypter.Init(database.NoOpCrypter{})

	db, mock := database.NewMockedGormDB()

	database.MockQueueJob(
		mock,
		&UpdateMemberParams{
			ChannelID: "C0123456789",
			UserID:    userID,
			Timezone:  sqlcrypter.NewEncryptedBytes(timezone),
		},
		models.JobTypeUpdateMember.String(),
		models.JobPriorityHigh,
	)

	err := UpsertMemberTimezoneInfo(context.Background(), db, interaction)
	assert.Nil(t, err)
}

func Test_RenderOnboardingConnectionModeView(t *testing.T) {
	g := goldie.New(t)

	interaction := &slack.InteractionCallback{
		User: slack.User{
			ID: "U0123456789",
		},
		View: slack.View{
			PrivateMetadata: `eyJjaGFubmVsX2lkIjoiQzAxMjM0NTY3ODkiLCJyZXNwb25zZV91cmwiOiJodHRwOi8vbG9jYWxob3N0L2FjdGlvbnMvYS9iL2MifQ==`,
		},
	}

	content, err := RenderOnboardingConnectionModeView(context.Background(), interaction, "http://localhost/")
	assert.Nil(t, err)
	assert.NotNil(t, content)

	g.Assert(t, "onboarding_connection_mode.json", content)
}

func Test_RenderOnboardingGenderView(t *testing.T) {
	g := goldie.New(t)

	interaction := &slack.InteractionCallback{
		User: slack.User{
			ID: "U0123456789",
		},
		View: slack.View{
			PrivateMetadata: "base64-encoded-data-here",
		},
	}

	content, err := RenderOnboardingGenderView(context.Background(), interaction, "http://localhost/")
	assert.Nil(t, err)
	assert.NotNil(t, content)

	g.Assert(t, "onboarding_gender.json", content)
}

func Test_UpsertMemberConnectionMode(t *testing.T) {
	userID := "U0123456789"

	interaction := &slack.InteractionCallback{
		User: slack.User{
			ID: "U0123456789",
		},
		View: slack.View{
			PrivateMetadata: `eyJjaGFubmVsX2lkIjoiQzAxMjM0NTY3ODkiLCJyZXNwb25zZV91cmwiOiJodHRwOi8vbG9jYWxo
b3N0L2FjdGlvbnMvYS9iL2MifQ==`,
			State: &slack.ViewState{
				Values: map[string]map[string]slack.BlockAction{
					"onboarding-connection-mode": {
						"onboarding-connection-mode-select": {
							SelectedOption: slack.OptionBlockObject{
								Value: models.ConnectionModeVirtual.String(),
							},
						},
					},
				},
			},
		},
	}

	db, mock := database.NewMockedGormDB()

	database.MockQueueJob(
		mock,
		&UpdateMemberParams{
			ChannelID:      "C0123456789",
			UserID:         userID,
			ConnectionMode: models.ConnectionModeVirtual.String(),
		},
		models.JobTypeUpdateMember.String(),
		models.JobPriorityHigh,
	)

	err := UpsertMemberConnectionMode(context.Background(), db, interaction)
	assert.Nil(t, err)
}

func Test_UpsertMemberGenderInfo(t *testing.T) {
	userID := "U0123456789"
	gender := models.Male.String()

	interaction := &slack.InteractionCallback{
		User: slack.User{
			ID: "U0123456789",
		},
		View: slack.View{
			PrivateMetadata: `eyJjaGFubmVsX2lkIjoiQzAxMjM0NTY3ODkiLCJyZXNwb25zZV91cmwiOiJodHRwOi8vbG9jYWxo
b3N0L2FjdGlvbnMvYS9iL2MifQ==`,
			State: &slack.ViewState{
				Values: map[string]map[string]slack.BlockAction{
					"onboarding-gender-select": {
						"onboarding-gender-select": {
							SelectedOption: slack.OptionBlockObject{
								Value: gender,
							},
						},
					},
					"onboarding-gender-checkbox": {
						"onboarding-gender-checkbox": {
							SelectedOptions: []slack.OptionBlockObject{
								{Value: "true"},
							},
						},
					},
				},
			},
		},
	}

	db, mock := database.NewMockedGormDB()

	hasGenderPreference := true

	database.MockQueueJob(
		mock,
		&UpdateMemberParams{
			ChannelID:           "C0123456789",
			UserID:              userID,
			Gender:              models.Male.String(),
			HasGenderPreference: &hasGenderPreference,
		},
		models.JobTypeUpdateMember.String(),
		models.JobPriorityHigh,
	)

	err := UpsertMemberGenderInfo(context.Background(), db, interaction)
	assert.Nil(t, err)
}

func Test_RenderOnboardingProfileView(t *testing.T) {
	g := goldie.New(t)

	interaction := &slack.InteractionCallback{
		User: slack.User{
			ID: "U0123456789",
		},
		View: slack.View{
			PrivateMetadata: "base64-encoded-data-here",
		},
	}

	content, err := RenderOnboardingProfileView(context.Background(), interaction, "http://localhost/")
	assert.Nil(t, err)
	assert.NotNil(t, content)

	g.Assert(t, "onboarding_profile.json", content)
}

func Test_UpsertMemberProfileInfo(t *testing.T) {
	userID := "U0123456789"
	profileType := "GitHub"
	profileLink := "github.com/bincyber"

	interaction := &slack.InteractionCallback{
		User: slack.User{
			ID: "U0123456789",
		},
		View: slack.View{
			PrivateMetadata: `eyJjaGFubmVsX2lkIjoiQzAxMjM0NTY3ODkiLCJyZXNwb25zZV91cmwiOiJodHRwOi8vbG9jYWxo
b3N0L2FjdGlvbnMvYS9iL2MifQ==`,
			State: &slack.ViewState{
				Values: map[string]map[string]slack.BlockAction{
					"onboarding-profile-type": {
						"onboarding-profile-type": slack.BlockAction{
							SelectedOption: slack.OptionBlockObject{
								Value: profileType,
							},
						},
					},
					"onboarding-profile-link": {
						"onboarding-profile-link": {
							Value: profileLink,
						},
					},
				},
			},
		},
	}

	sqlcrypter.Init(database.NoOpCrypter{})

	db, mock := database.NewMockedGormDB()

	database.MockQueueJob(
		mock,
		&UpdateMemberParams{
			ChannelID:   "C0123456789",
			UserID:      userID,
			ProfileType: sqlcrypter.NewEncryptedBytes(profileType),
			ProfileLink: sqlcrypter.NewEncryptedBytes(profileLink),
		},
		models.JobTypeUpdateMember.String(),
		models.JobPriorityHigh,
	)

	err := UpsertMemberProfileInfo(context.Background(), db, interaction)
	assert.Nil(t, err)
}

func Test_ValidateMemberProfileInfo(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		interaction := &slack.InteractionCallback{
			View: slack.View{
				State: &slack.ViewState{
					Values: map[string]map[string]slack.BlockAction{
						"onboarding-profile-type": {
							"onboarding-profile-type": slack.BlockAction{
								SelectedOption: slack.OptionBlockObject{
									Value: "Twitter",
								},
							},
						},
						"onboarding-profile-link": {
							"onboarding-profile-link": {
								Value: "twitter.com/bincyber",
							},
						},
					},
				},
			},
		}

		err := ValidateMemberProfileInfo(context.Background(), interaction)
		assert.Nil(t, err)
	})

	t.Run("url parsing error", func(t *testing.T) {
		interaction := &slack.InteractionCallback{
			View: slack.View{
				State: &slack.ViewState{
					Values: map[string]map[string]slack.BlockAction{
						"onboarding-profile-type": {
							"onboarding-profile-type": slack.BlockAction{
								SelectedOption: slack.OptionBlockObject{
									Value: "Twitter",
								},
							},
						},
						"onboarding-profile-link": {
							"onboarding-profile-link": {
								Value: "not a valid url",
							},
						},
					},
				},
			},
		}

		err := ValidateMemberProfileInfo(context.Background(), interaction)
		assert.NotNil(t, err)
	})

	t.Run("missing user handler", func(t *testing.T) {
		interaction := &slack.InteractionCallback{
			View: slack.View{
				State: &slack.ViewState{
					Values: map[string]map[string]slack.BlockAction{
						"onboarding-profile-type": {
							"onboarding-profile-type": slack.BlockAction{
								SelectedOption: slack.OptionBlockObject{
									Value: "Twitter",
								},
							},
						},
						"onboarding-profile-link": {
							"onboarding-profile-link": {
								Value: "twitter.com/",
							},
						},
					},
				},
			},
		}

		err := ValidateMemberProfileInfo(context.Background(), interaction)
		assert.NotNil(t, err)
	})

	t.Run("mismatch", func(t *testing.T) {
		interaction := &slack.InteractionCallback{
			View: slack.View{
				State: &slack.ViewState{
					Values: map[string]map[string]slack.BlockAction{
						"onboarding-profile-type": {
							"onboarding-profile-type": slack.BlockAction{
								SelectedOption: slack.OptionBlockObject{
									Value: "GitHub",
								},
							},
						},
						"onboarding-profile-link": {
							"onboarding-profile-link": {
								Value: "twitter.com/bincyber",
							},
						},
					},
				},
			},
		}

		err := ValidateMemberProfileInfo(context.Background(), interaction)
		assert.NotNil(t, err)
	})
}

func Test_RenderOnboardingCalendlyView(t *testing.T) {
	g := goldie.New(t)

	interaction := &slack.InteractionCallback{
		User: slack.User{
			ID: "U0123456789",
		},
		View: slack.View{
			PrivateMetadata: "base64-encoded-data-here",
		},
	}

	content, err := RenderOnboardingCalendlyView(context.Background(), interaction, "http://localhost/")
	assert.Nil(t, err)
	assert.NotNil(t, content)

	g.Assert(t, "onboarding_calendly.json", content)
}

func Test_ValidateMemberCalendlyLink(t *testing.T) {
	type test struct {
		name  string
		link  string
		isErr bool
	}

	tt := []test{
		{"nil", "", false},
		{"success", "https://calendly.com/bincyber", false},
		{"no scheme", "calendly.com/bincyber", false},
		{"missing user", "calendly.com/", true},
		{"malformed", "calendly.com/ typo", true},
		{"invalid domain", "scheduling.io/bincyber", true},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateMemberCalendlyLink(context.Background(), tc.link)

			if tc.isErr {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func Test_UpsertMemberCalendlyLink(t *testing.T) {
	userID := "U0123456789"
	calendlyLink := "calendly.com/bincyber"

	interaction := &slack.InteractionCallback{
		User: slack.User{
			ID: "U0123456789",
		},
		View: slack.View{
			PrivateMetadata: `eyJjaGFubmVsX2lkIjoiQzAxMjM0NTY3ODkiLCJyZXNwb25zZV91cmwiOiJodHRwOi8vbG9jYWxo
b3N0L2FjdGlvbnMvYS9iL2MifQ==`,
			State: &slack.ViewState{
				Values: map[string]map[string]slack.BlockAction{
					"onboarding-calendly": {
						"onboarding-calendly": {
							Value: calendlyLink,
						},
					},
				},
			},
		},
	}

	sqlcrypter.Init(database.NoOpCrypter{})

	db, mock := database.NewMockedGormDB()

	database.MockQueueJob(
		mock,
		&UpdateMemberParams{
			ChannelID:    "C0123456789",
			UserID:       userID,
			CalendlyLink: sqlcrypter.NewEncryptedBytes(calendlyLink),
		},
		models.JobTypeUpdateMember.String(),
		models.JobPriorityHigh,
	)

	err := UpsertMemberCalendlyLink(context.Background(), db, interaction)
	assert.Nil(t, err)
}

func Test_SetMemberIsActive(t *testing.T) {
	// TODO
}

func Test_RespondGreetMemberWebhook(t *testing.T) {
	userID := "U0123456789"
	channelID := "C9876543210"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header["Content-Type"][0])

		var request *slack.WebhookMessage
		err := json.NewDecoder(r.Body).Decode(&request)
		assert.Nil(t, err)
		assert.Len(t, request.Blocks.BlockSet, 8)

		sectionBlock := request.Blocks.BlockSet[6].(*slack.SectionBlock)
		assert.Contains(t, sectionBlock.Text.Text, "Thank you")

		contextBlock := request.Blocks.BlockSet[7].(*slack.ContextBlock)
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

	err = RespondGreetMemberWebhook(context.Background(), &http.Client{}, interaction)
	assert.Nil(t, err)
}
