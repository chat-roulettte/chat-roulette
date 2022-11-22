package bot

import (
	"context"
	"sort"
	"time"

	"github.com/slack-go/slack"
	"go.opentelemetry.io/otel"
)

// chatRouletteChannel represents a Slack channel with chat-roulette enabled.
type chatRouletteChannel struct {
	// ChannelID is the id of the Slack channel
	ChannelID string

	// Invitor is the user who invited the chat-roulette bot
	// to the Slack channel or the creator of the Slack channel
	Invitor string

	// Create is set to true if the Slack channel exists in Slack
	// but not in the database, therefore it must be created.
	Create bool

	// Delete is set to true if the Slack channel exists in the database
	// but not in Slack, therefore it must be deleted.
	Delete bool
}

// chatRouletteMember represents a Slack user who is a member of
// a Slack channel with chat-roulette enabled.
type chatRouletteMember struct {
	// UserID is the id of the Slack user
	UserID string

	// Create is set to true if the Slack user is a member of the Slack channel,
	// but they are not in the database, therefore they must be added.
	Create bool

	// Delete is set to true if the Slack user exists in the database, but they
	// are not a member of the Slack channel, therefore they must be deleted.
	Delete bool
}

// getChannels uses Slack's users.conversations API method to retrieve all
// of the public and private channels that the chat-roulette bot is a member of.
//
// See: https://api.slack.com/methods/users.conversations
func getChannels(ctx context.Context, client *slack.Client, botUserID string) ([]chatRouletteChannel, error) {
	params := &slack.GetConversationsForUserParameters{
		UserID:          botUserID,
		Types:           []string{"public_channel", "private_channel"},
		Limit:           100,
		ExcludeArchived: true,
	}

	// Set deadline for Slack API call
	ctx, cancel := context.WithTimeout(ctx, 2000*time.Millisecond)
	defer cancel()

	// Skip pagination, assume bot is not a member of more than 100 channels
	resp, _, err := client.GetConversationsForUserContext(ctx, params)
	if err != nil {
		return nil, err
	}

	var slackChannels []chatRouletteChannel
	for _, i := range resp {
		channel := chatRouletteChannel{
			ChannelID: i.Conversation.ID,
			Invitor:   i.Creator,
		}
		slackChannels = append(slackChannels, channel)
	}

	return slackChannels, nil
}

// getChannelMembers uses Slack's conversation.members API method to retrieve
// the members of the given Slack channel using cursor-based pagination.
//
// See: https://api.slack.com/methods/conversations.members
func getChannelMembers(ctx context.Context, client *slack.Client, channelID string, limit int) ([]chatRouletteMember, error) {
	params := &slack.GetUsersInConversationParameters{
		ChannelID: channelID,
		Limit:     limit,
	}

	var channelMembers []chatRouletteMember
	for {
		// Set deadline for Slack API call
		ctx, cancel := context.WithTimeout(ctx, 2000*time.Millisecond)
		defer cancel()

		members, cursor, err := client.GetUsersInConversationContext(ctx, params)
		if err != nil {
			return nil, err
		}

		for _, i := range members {
			member := chatRouletteMember{
				UserID: i,
			}
			channelMembers = append(channelMembers, member)
		}

		if cursor != "" {
			params.Cursor = cursor
		} else {
			break
		}
	}

	return channelMembers, nil
}

// reconcileChannels identifies which Slack channels need to be
// created in the database and which ones need to be deleted.
func reconcileChannels(ctx context.Context, slackChannels, dbChannels []chatRouletteChannel) []chatRouletteChannel {
	// Start a new span
	tracer := otel.Tracer("")
	_, span := tracer.Start(ctx, "reconcile.channels")
	defer span.End()

	channels := make(map[string]chatRouletteChannel)

	for _, i := range slackChannels {
		i.Create = true
		channels[i.ChannelID] = i
	}

	for _, i := range dbChannels {
		v, ok := channels[i.ChannelID]

		switch ok {
		case true:
			// Channel already exists, so don't recreate
			v.Create = false
			channels[i.ChannelID] = v
		case false:
			// Channel exists, but shouldn't so mark for deletion
			i.Delete = true
			channels[i.ChannelID] = i
		}
	}

	// Sort the map to ease testing
	keys := make([]string, 0, len(channels))
	for k := range channels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var reconciledChannels []chatRouletteChannel
	for _, k := range keys {
		reconciledChannels = append(reconciledChannels, channels[k])
	}

	return reconciledChannels
}

// reconcileMembers identifies which members of a Slack channel
// need to be added to the database and which ones need to be deleted.
func reconcileMembers(ctx context.Context, slackMembers, dbMembers []chatRouletteMember) []chatRouletteMember {
	// Start a new span
	tracer := otel.Tracer("")
	_, span := tracer.Start(ctx, "reconcile.members")
	defer span.End()

	members := make(map[string]chatRouletteMember)

	for _, i := range slackMembers {
		i.Create = true
		members[i.UserID] = i
	}

	for _, i := range dbMembers {
		v, ok := members[i.UserID]

		switch ok {
		case true:
			// Member already exists, so don't recreate
			v.Create = false
			members[i.UserID] = v
		case false:
			// Member exists, but shouldn't so mark for deletion
			i.Delete = true
			members[i.UserID] = i
		}
	}

	// Sort the map to ease testing
	keys := make([]string, 0, len(members))
	for k := range members {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var reconciledMembers []chatRouletteMember
	for _, k := range keys {
		reconciledMembers = append(reconciledMembers, members[k])
	}

	return reconciledMembers
}
