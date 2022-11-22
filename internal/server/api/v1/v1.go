package v1

import (
	"github.com/chat-roulettte/chat-roulette/internal/server"
	"github.com/chat-roulettte/chat-roulette/internal/server/api"
)

const (
	// APIVersion is the version for the APIs in this package
	APIVersion = "v1"

	// RoutesPrefix is the route prefix for APIs in this package
	RoutesPrefix = "/v1"
)

type implServer struct {
	*server.Server
}

// RegisterRoutes registers routes from this package on the given server
func RegisterRoutes(s *server.Server) {
	i := implServer{s}

	routes := []api.Route{
		{Path: "slack/event", Methods: []string{"POST"}, Func: i.slackEventHandler},
		{Path: "slack/interaction", Methods: []string{"POST"}, Func: i.slackInteractionHandler},
		{Path: "slack/options", Methods: []string{"POST"}, Func: i.slackOptionsHandler},
		{Path: "member", Methods: []string{"POST"}, Func: i.updateMemberHandler},
		{Path: "channel", Methods: []string{"POST"}, Func: i.updateChannelHandler},
		{Path: "timezones/{country}", Methods: []string{"GET"}, Func: i.timezonesHandler},
	}

	api.RegisterRoutes(s.GetMux(), RoutesPrefix, routes)
}
