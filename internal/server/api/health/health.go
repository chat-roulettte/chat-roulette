package health

import (
	"github.com/chat-roulettte/chat-roulette/internal/server"
	"github.com/chat-roulettte/chat-roulette/internal/server/api"
)

type implServer struct {
	*server.Server
}

const healthRoutesPrefix = "/-/"

// RegisterRoutes registers health routes on the given server
func RegisterRoutes(s *server.Server) {
	i := implServer{s}

	routes := []api.Route{
		{Path: "healthy", Methods: []string{"GET"}, Func: i.healthHandler},
		{Path: "ready", Methods: []string{"GET"}, Func: i.readinessHandler},
	}

	api.RegisterRoutes(s.GetMux(), healthRoutesPrefix, routes)
}
