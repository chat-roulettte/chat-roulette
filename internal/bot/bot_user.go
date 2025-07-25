package bot

import (
	"context"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/slack-go/slack"
)

// GetBotUserID uses Slack's auth.test API method
// to retrieve the UserID of the chat-roulette bot
func GetBotUserID(ctx context.Context, client *slack.Client) (string, error) {
	resp, err := client.AuthTestContext(ctx)
	if err != nil {
		return "", err
	}

	return resp.UserID, nil
}

func GetBotTeamAppIDs(ctx context.Context, client *slack.Client) (string, string, error) {
	var teamID, appID string

	// Retrieve TeamID
	slackCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	rAuthTest, err := client.AuthTestContext(slackCtx)
	if err != nil {
		return teamID, appID, errors.Wrap(err, "failed to call Slack's auth.test API endpoint")
	}
	teamID = rAuthTest.TeamID

	// Retrieve AppID
	slackCtx, cancel = context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	rBotInfo, err := client.GetBotInfoContext(slackCtx, slack.GetBotInfoParameters{
		Bot: rAuthTest.BotID,
	})
	if err != nil {
		return teamID, appID, errors.Wrap(err, "failed to call Slack's bots.info API endpoint")
	}
	appID = rBotInfo.AppID

	return teamID, appID, nil
}

// isUserASlackBot uses Slack's users.info API method to check if the given user is actually a bot.
func isUserASlackBot(ctx context.Context, client *slack.Client, userID string) (bool, error) {
	resp, err := client.GetUserInfoContext(ctx, userID)
	if err != nil {
		return false, err
	}

	return resp.IsBot, nil
}

// isBotAChannelMember checks if the chat roulette bot is a member of a Slack channel.
func isBotAChannelMember(ctx context.Context, client *slack.Client, botUserID, channelID string) (bool, error) {
	slackChannels, err := getChannels(ctx, client, botUserID)
	if err != nil {
		return false, errors.Wrap(err, "failed to check if bot is a member of the Slack channel")
	}

	var isMember bool
	for _, i := range slackChannels {
		if i.ChannelID == channelID {
			isMember = true
			break
		}
	}

	return isMember, nil
}

// generateAppHomeDeepLink generates a deep link to the Chat Roulette app's App Home page
func generateAppHomeDeepLink(teamID, appID string) string {
	q := make(url.Values)

	q.Set("tab", "home")
	q.Set("team", teamID)
	q.Set("id", appID)

	u := url.URL{
		Scheme:   "slack",
		Host:     "app",
		Path:     "",
		RawQuery: q.Encode(),
	}

	return u.String()
}
