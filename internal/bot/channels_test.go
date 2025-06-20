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

func Test_getChannels(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handlerFn := func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"ok": true,
				"channels": []slack.Channel{
					{
						GroupConversation: slack.GroupConversation{
							Conversation: slack.Conversation{
								ID: "C012AB3CD",
							},
							Creator: "U012A3CDE",
						},
					},
					{
						GroupConversation: slack.GroupConversation{
							Conversation: slack.Conversation{
								ID: "G01L6TE6MCK",
							},
							Creator: "U061F7AUR",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}

		slackServer := slacktest.NewTestServer()
		slackServer.Handle("/users.conversations", handlerFn)
		go slackServer.Start()
		defer slackServer.Stop()

		client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(slackServer.GetAPIURL()))

		channels, err := getChannels(context.Background(), client, "U023BECGF")
		assert.Len(t, channels, 2)
		assert.Nil(t, err)
		assert.Equal(t, channels[0].ChannelID, "C012AB3CD")
		assert.Equal(t, channels[0].Inviter, "U012A3CDE")
		assert.Equal(t, channels[1].ChannelID, "G01L6TE6MCK")
		assert.Equal(t, channels[1].Inviter, "U061F7AUR")
	})

	t.Run("mocked failure", func(t *testing.T) {
		handlerFn := func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"ok":    false,
				"error": "invalid_auth",
			}
			json.NewEncoder(w).Encode(response)
		}

		slackServer := slacktest.NewTestServer()
		slackServer.Handle("/users.conversations", handlerFn)
		go slackServer.Start()
		defer slackServer.Stop()

		client := slack.New("xoxb-invalid-slack-authtoken", slack.OptionAPIURL(slackServer.GetAPIURL()))

		channels, err := getChannels(context.Background(), client, "U023BECGF")
		assert.Nil(t, channels)
		assert.NotNil(t, err)
		assert.Contains(t, "invalid_auth", err.Error())
	})

	t.Run("failure", func(t *testing.T) {
		client := slack.New("xoxb-invalid-slack-authtoken")

		channels, err := getChannels(context.Background(), client, "U023BECGF")
		assert.Nil(t, channels)
		assert.NotNil(t, err)
		assert.Contains(t, "invalid_auth", err.Error())
	})
}

func Test_getChannelMembers(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handlerFn := func(w http.ResponseWriter, r *http.Request) {
			r.ParseForm()

			cursor := r.FormValue("cursor")

			var response map[string]interface{}

			if cursor == "" {
				response = map[string]interface{}{
					"ok": true,
					"members": []string{
						"U024BECGF",
						"U061F7AUR",
						"U062F7BUR",
						"U025BCEGF",
						"U071DC4GF",
					},
					"response_metadata": map[string]string{
						"next_cursor": "dXNlcjpVMDYxTkZUVDI=",
					},
				}
			} else {
				response = map[string]interface{}{
					"ok": true,
					"members": []string{
						"U02FF3BCE",
						"U068F7AUR",
						"U067AU1FR",
						"U0673BECR",
						"U09E67BVR",
					},
				}
			}

			json.NewEncoder(w).Encode(response)
		}

		slackServer := slacktest.NewTestServer()
		slackServer.Handle("/conversations.members", handlerFn)
		go slackServer.Start()
		defer slackServer.Stop()

		client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(slackServer.GetAPIURL()))

		members, err := getChannelMembers(context.Background(), client, "U023BECGF", 5)
		assert.Len(t, members, 10)
		assert.Nil(t, err)
	})

	t.Run("mocked failure", func(t *testing.T) {
		handlerFn := func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"ok":    false,
				"error": "invalid_auth",
			}
			json.NewEncoder(w).Encode(response)
		}

		slackServer := slacktest.NewTestServer()
		slackServer.Handle("/conversations.members", handlerFn)
		go slackServer.Start()
		defer slackServer.Stop()

		client := slack.New("xoxb-invalid-slack-authtoken", slack.OptionAPIURL(slackServer.GetAPIURL()))

		members, err := getChannelMembers(context.Background(), client, "U023BECGF", 10)
		assert.Nil(t, members)
		assert.NotNil(t, err)
		assert.Contains(t, "invalid_auth", err.Error())
	})

	t.Run("failure", func(t *testing.T) {
		client := slack.New("xoxb-invalid-slack-authtoken")

		members, err := getChannelMembers(context.Background(), client, "U023BECGF", 100)
		assert.Nil(t, members)
		assert.NotNil(t, err)
		assert.Contains(t, "invalid_auth", err.Error())
	})
}

func Test_reconcileChannels(t *testing.T) {
	slackChannels := []chatRouletteChannel{
		{
			ChannelID: "C0123456789",
		},
		{
			ChannelID: "G9876543210",
		},
		{
			ChannelID: "G1928374650",
		},
	}

	dbChannels := []chatRouletteChannel{
		{
			ChannelID: "C0123456789",
		},
		{
			ChannelID: "C0543219876",
		},
	}

	expected := []chatRouletteChannel{
		{
			ChannelID: "C0123456789",
			Create:    false,
			Delete:    false,
		},
		{
			ChannelID: "G9876543210",
			Create:    true,
			Delete:    false,
		},
		{
			ChannelID: "C0543219876",
			Create:    false,
			Delete:    true,
		},
		{
			ChannelID: "G1928374650",
			Create:    true,
			Delete:    false,
		},
	}

	actual := reconcileChannels(context.Background(), slackChannels, dbChannels)

	assert.Len(t, actual, 4)
	assert.ElementsMatch(t, expected, actual)
}

func Test_reconcileChannelMembers(t *testing.T) {
	slackMembers := []chatRouletteMember{
		{
			UserID: "U0123456789",
		},
		{
			UserID: "U9876543210",
		},
		{
			UserID: "U1928374650",
		},
	}

	dbMembers := []chatRouletteMember{
		{
			UserID: "U0123456789",
		},
		{
			UserID: "U0543219876",
		},
	}

	expected := []chatRouletteMember{
		{
			UserID: "U0123456789",
			Create: false,
			Delete: false,
		},
		{
			UserID: "U9876543210",
			Create: true,
			Delete: false,
		},
		{
			UserID: "U0543219876",
			Create: false,
			Delete: true,
		},
		{
			UserID: "U1928374650",
			Create: true,
			Delete: false,
		},
	}

	actual := reconcileMembers(context.Background(), slackMembers, dbMembers)

	assert.Len(t, actual, 4)
	assert.ElementsMatch(t, expected, actual)
}
