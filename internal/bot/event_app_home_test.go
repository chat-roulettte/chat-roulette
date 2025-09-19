package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	goldie "github.com/sebdah/goldie/v2"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
)

func Test_HandleAppHomeEvent(t *testing.T) {
	r := require.New(t)

	resource, databaseURL, err := database.NewTestPostgresDB(false)
	r.NoError(err)
	defer resource.Close()

	db, err := database.NewGormDB(databaseURL)
	r.NoError(err)
	r.NoError(database.Migrate(databaseURL))

	channelID1 := "C0123456789"
	channelID2 := "C9876543210"

	memberID := "U0123456789"

	nextRound := time.Date(2021, time.January, 4, 12, 0, 0, 0, time.UTC)

	// Write channels to the database
	channel1 := &models.Channel{
		ChannelID:      channelID1,
		Inviter:        memberID,
		ConnectionMode: models.ConnectionModeVirtual,
		Interval:       models.Biweekly,
		Weekday:        time.Monday,
		Hour:           12,
		NextRound:      nextRound,
	}
	channel2 := &models.Channel{
		ChannelID:      channelID2,
		Inviter:        memberID,
		ConnectionMode: models.ConnectionModeHybrid,
		Interval:       models.Monthly,
		Weekday:        time.Monday,
		Hour:           12,
		NextRound:      nextRound,
	}
	db.Create(channel1)
	db.Create(channel2)

	// Write records in the members table
	isActive := true
	hasGenderPreference := false
	db.Create(&models.Member{
		ChannelID:           channelID1,
		UserID:              memberID,
		IsActive:            &isActive,
		Gender:              models.Male,
		HasGenderPreference: &hasGenderPreference,
		ConnectionMode:      models.ConnectionModeVirtual,
	})
	db.Create(&models.Member{
		ChannelID:           channelID2,
		UserID:              memberID,
		IsActive:            &isActive,
		Gender:              models.Male,
		HasGenderPreference: &hasGenderPreference,
		ConnectionMode:      models.ConnectionModePhysical,
	})

	params := &AppHomeParams{
		BotUserID: "U0123456789",
		URL:       "https://chat-roulette-for-slack.com",
		UserID:    memberID,
	}

	// Mock Slack API call to /views.publish
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req *slack.PublishViewContextRequest

		err := json.NewDecoder(r.Body).Decode(&req)
		assert.Nil(t, err)
		assert.Len(t, req.View.Blocks.BlockSet, 20)

		// Assert that the response matches the right template
		template := appHomeTemplate{
			BotUserID: params.BotUserID,
			AppURL:    params.URL,
			Channels:  []models.Channel{*channel1, *channel2},
			IsAppUser: true,
		}

		content, err := renderTemplate(appHomeTemplateFilename, template)
		assert.Nil(t, err)

		var view slack.HomeTabViewRequest
		err = json.Unmarshal([]byte(content), &view)
		assert.Nil(t, err)
		assert.Equal(t, req.View.Blocks.BlockSet, view.Blocks.BlockSet)
		w.Write([]byte(`{}`))
	}))
	defer httpServer.Close()
	url := fmt.Sprintf("%s/", httpServer.URL)
	client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(url))

	err = HandleAppHomeEvent(context.Background(), client, db, params)
	r.NoError(err)
}

func Test_appHomeTemplate(t *testing.T) {
	r := require.New(t)
	g := goldie.New(t)

	nextRound := time.Date(2021, time.January, 4, 12, 0, 0, 0, time.UTC)

	type test struct {
		name       string
		goldenFile string
		data       appHomeTemplate
		blocks     int
		isErr      bool
	}

	tt := []test{
		{
			name:       "app user",
			goldenFile: "app_home.json",
			data: appHomeTemplate{
				BotUserID: "U0123456789",
				AppURL:    "https://chat-roulette-for-slack.com",
				Channels: []models.Channel{
					{
						ChannelID:      "C0123456789",
						Inviter:        "U0123456789",
						Interval:       models.Biweekly,
						Weekday:        time.Monday,
						ConnectionMode: models.ConnectionModeVirtual,
						NextRound:      nextRound,
					},
				},
				IsAppUser: true,
			},
			blocks: 19,
			isErr:  false,
		},
		{
			name:       "app user multiple channels",
			goldenFile: "app_home_multiple_channels.json",
			data: appHomeTemplate{
				BotUserID: "U0123456789",
				AppURL:    "https://chat-roulette-for-slack.com",
				Channels: []models.Channel{
					{
						ChannelID:      "C0123456789",
						Inviter:        "U0123456789",
						Interval:       models.Biweekly,
						Weekday:        time.Monday,
						ConnectionMode: models.ConnectionModeVirtual,
						NextRound:      nextRound,
					},
					{
						ChannelID:      "C9876543210",
						Inviter:        "U0123456789",
						Interval:       models.Monthly,
						Weekday:        time.Monday,
						ConnectionMode: models.ConnectionModeHybrid,
						NextRound:      nextRound,
					},
				},
				IsAppUser: true,
			},
			blocks: 20,
			isErr:  false,
		},
		{
			name:       "non app user",
			goldenFile: "app_home_non_user.json",
			data: appHomeTemplate{
				BotUserID: "U0123456789",
				AppURL:    "https://chat-roulette-for-slack.com",
				IsAppUser: false,
			},
			blocks: 8,
			isErr:  false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			content, err := renderTemplate(appHomeTemplateFilename, tc.data)
			if tc.isErr {
				r.Equal("foo", content)
				r.Error(err)
			}
			r.NoError(err)

			g.Assert(t, tc.goldenFile, []byte(content))

			var view slack.HomeTabViewRequest
			err = json.Unmarshal([]byte(content), &view)
			r.NoError(err)
			r.Equal(view.Type, slack.VTHomeTab)
			r.Len(view.Blocks.BlockSet, tc.blocks)
		})
	}
}
