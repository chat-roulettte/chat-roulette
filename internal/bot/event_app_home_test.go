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

	data := appHomeTemplate{
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
	}

	content, err := renderTemplate(appHomeTemplateFilename, data)
	r.NoError(err)

	g.Assert(t, "app_home.json", []byte(content))

	var view slack.HomeTabViewRequest
	err = json.Unmarshal([]byte(content), &view)
	r.NoError(err)
	r.Equal(view.Type, slack.VTHomeTab)
	r.Len(view.Blocks.BlockSet, 14)
}
