package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slacktest"
	"github.com/stretchr/testify/assert"
)

func Test_GetBotUserID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		slackServer := slacktest.NewTestServer()
		go slackServer.Start()
		defer slackServer.Stop()

		client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(slackServer.GetAPIURL()))

		userID, err := GetBotUserID(context.Background(), client)
		assert.Equal(t, userID, "W012A3CDE")
		assert.Nil(t, err)
	})

	t.Run("failure", func(t *testing.T) {
		client := slack.New("xoxb-invalid-slack-authtoken")

		userID, err := GetBotUserID(context.Background(), client)
		assert.Equal(t, userID, "")
		assert.NotNil(t, err)
	})
}

func Test_IsUserABot(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		slackServer := slacktest.NewTestServer()
		go slackServer.Start()
		defer slackServer.Stop()

		client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(slackServer.GetAPIURL()))

		boolean, err := isUserASlackBot(context.Background(), client, "W012A3CDE")
		assert.Nil(t, err)
		assert.False(t, boolean)
	})

	t.Run("failure", func(t *testing.T) {
		client := slack.New("xoxb-invalid-slack-authtoken")

		_, err := isUserASlackBot(context.Background(), client, "W012A3CDE")
		assert.NotNil(t, err)
	})
}

func Test_isBotChannelMember(t *testing.T) {
	channelID := "C0123456789"
	botUserID := "U0000000000"

	var response map[string]interface{}

	handlerFn := func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(response)
	}

	slackServer := slacktest.NewTestServer()
	slackServer.Handle("/users.conversations", handlerFn)
	go slackServer.Start()
	defer slackServer.Stop()

	client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(slackServer.GetAPIURL()))

	t.Run("true", func(t *testing.T) {
		response = map[string]interface{}{
			"ok": true,
			"channels": []slack.Channel{
				{
					GroupConversation: slack.GroupConversation{
						Conversation: slack.Conversation{
							ID: channelID,
						},
						Creator: "U0123456789",
					},
				},
				{
					GroupConversation: slack.GroupConversation{
						Conversation: slack.Conversation{
							ID: "G9182736450",
						},
						Creator: "U987654321",
					},
				},
			},
		}

		isMember, err := isBotAChannelMember(context.Background(), client, botUserID, channelID)

		assert.True(t, isMember)
		assert.Nil(t, err)
	})

	t.Run("false", func(t *testing.T) {
		response = map[string]interface{}{
			"ok":       true,
			"channels": []slack.Channel{},
		}

		isMember, err := isBotAChannelMember(context.Background(), client, botUserID, channelID)

		assert.False(t, isMember)
		assert.Nil(t, err)
	})

	t.Run("error", func(t *testing.T) {
		response = map[string]interface{}{
			"ok":    false,
			"error": "invalid_auth",
		}

		isMember, err := isBotAChannelMember(context.Background(), client, botUserID, channelID)

		assert.False(t, isMember)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "failed to check if bot is a member of the Slack channel")
	})
}

func Test_generateAppHomeDeepLink(t *testing.T) {
	actual := generateAppHomeDeepLink("T1234567890", "A1234567890")
	expected := "slack://app?id=A1234567890&tab=home&team=T1234567890"
	assert.Equal(t, expected, actual)
}

func Test_GetBotTeamAppIDs(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		slackServer := slacktest.NewTestServer()

		go slackServer.Start()
		defer slackServer.Stop()

		client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(slackServer.GetAPIURL()))

		actualTeamID, actualAppID, err := GetBotTeamAppIDs(context.Background(), client)
		assert.Equal(t, "T024BE7LD", actualTeamID)
		assert.Equal(t, "A4H1JB4AZ", actualAppID)
		assert.Nil(t, err)
	})

	t.Run("auth failure", func(t *testing.T) {
		client := slack.New("xoxb-invalid-slack-authtoken")
		teamID, appID, err := GetBotTeamAppIDs(context.Background(), client)
		assert.Empty(t, teamID)
		assert.Empty(t, appID)
		assert.NotNil(t, err)
	})
}
