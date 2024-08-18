package ui

import (
	"context"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/slack-go/slack"
)

const (
	// CacheKeySlackTeamInfo is the name of the key in the cache containing the Slack team info
	CacheKeySlackTeamInfo = "SLACK_TEAM_INFO"
)

// lookupSlackWorkspace looks up info on the Slack workspace.
// It tries the cache first, before hitting the Slack API.
func lookupSlackWorkspace(ctx context.Context, cache *ristretto.Cache, client *slack.Client) (*slack.TeamInfo, error) {
	v, ok := cache.Get(CacheKeySlackTeamInfo)
	if ok {
		return v.(*slack.TeamInfo), nil
	}

	ctx, cancel := context.WithTimeout(ctx, 2000*time.Millisecond)
	defer cancel()

	teamInfo, err := client.GetTeamInfoContext(ctx)
	if err != nil {
		return nil, err
	}

	// Cache for 8 hours
	cache.SetWithTTL(CacheKeySlackTeamInfo, teamInfo, 1, 4*time.Hour)

	return teamInfo, nil
}

// lookupSlackChannel looks up info on a Slack channel.
// It tries the cache first, before hitting the Slack API.
func lookupSlackChannel(ctx context.Context, cache *ristretto.Cache, client *slack.Client, channelID string) (*slack.Channel, error) {

	v, ok := cache.Get(channelID)
	if ok {
		return v.(*slack.Channel), nil
	}

	ctx, cancel := context.WithTimeout(ctx, 2000*time.Millisecond)
	defer cancel()

	p := &slack.GetConversationInfoInput{
		ChannelID:     channelID,
		IncludeLocale: false,
	}

	channel, err := client.GetConversationInfoContext(ctx, p)
	if err != nil {
		return nil, err
	}

	// Cache for 8 hours
	cache.SetWithTTL(channelID, channel, 1, 8*time.Hour)

	return channel, nil
}

// lookupSlackUser looks up info on a Slack user.
// It tries the cache first, before hitting the Slack API.
func lookupSlackUser(ctx context.Context, cache *ristretto.Cache, client *slack.Client, userID string) (*slack.User, error) {

	v, ok := cache.Get(userID)
	if ok {
		return v.(*slack.User), nil
	}

	ctx, cancel := context.WithTimeout(ctx, 2000*time.Millisecond)
	defer cancel()

	user, err := client.GetUserInfoContext(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Cache for 2 hours
	cache.SetWithTTL(userID, user, 1, 2*time.Hour)

	return user, nil
}
