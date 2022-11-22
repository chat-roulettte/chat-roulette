package health

import (
	"context"
	"net/http"
	"time"

	"github.com/chat-roulettte/chat-roulette/internal/bot"
	"github.com/chat-roulettte/chat-roulette/internal/database"
)

type check struct {
	completed bool
	err       error
}

// readinessHandler reports if the Server is ready to start receiving requests
//
// HTTP Method: GET
//
// HTTP Path: /ready
func (s *implServer) readinessHandler(w http.ResponseWriter, r *http.Request) {
	dbCheck := &check{}
	slackCheck := &check{}

	ctx, cancel := context.WithTimeout(r.Context(), 1000*time.Millisecond)
	defer cancel()

	// Check the connection to the database
	go func() {
		dbCheck.err = database.Ping(ctx, s.GetDB())
		dbCheck.completed = true
	}()

	// Check the connection to Slack
	go func() {
		_, slackCheck.err = bot.GetBotUserID(ctx, s.GetSlackClient())
		slackCheck.completed = true
	}()

	// Use a ticker to control the frequency of the loop
	ticker := time.NewTicker(time.Millisecond)

	for {
		select {
		case <-ctx.Done():
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("not ready"))
			return

		case <-ticker.C:
			if !dbCheck.completed || !slackCheck.completed {
				break
			}

			if dbCheck.err != nil || slackCheck.err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("not ready"))
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ready"))
			return
		}
	}
}
