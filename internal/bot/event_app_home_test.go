package bot

import (
	"encoding/json"
	"testing"
	"time"

	goldie "github.com/sebdah/goldie/v2"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/require"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
)

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
						ConnectionMode: models.VirtualConnectionMode,
						NextRound:      nextRound,
					},
				},
				IsAppUser: true,
			},
			blocks: 19,
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
