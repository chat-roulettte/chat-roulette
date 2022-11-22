package ui

import (
	"net/http"
	"strings"

	"github.com/chat-roulettte/chat-roulette/internal/server"
)

type implServer struct {
	*server.Server
}

func cachingMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=600") // 10 minutes
		h.ServeHTTP(w, r)
	})
}

// RegisterRoutes registers UI routes on the given server
func RegisterRoutes(s *server.Server) {
	i := implServer{s}

	mux := s.GetMux()

	mux.PathPrefix("/static/").Handler(cachingMiddleware(http.FileServer(http.FS(embeddedFS))))
	mux.HandleFunc("/", i.indexHandler)
	mux.HandleFunc("/profile", i.profileHandler)
	mux.HandleFunc("/profile/{channel_id}", i.memberProfileHandler)
	mux.HandleFunc("/history/{channel_id}", i.historyHandler)
	mux.HandleFunc("/channel/{channel_id}", i.channelAdminHandler)
	mux.HandleFunc("/{[45]0[13]}", i.errorHandler)
}

// indexHandler for the index page
//
// HTTP Method: GET
//
// HTTP Path: /
func (s *implServer) indexHandler(w http.ResponseWriter, r *http.Request) {
	// Redirect to /profile if user is authenticated
	if session, err := s.GetSession(r); err == nil {
		if auth, ok := session.Values["authenticated"].(bool); ok && auth {
			http.Redirect(w, r, "/profile", http.StatusFound)
			return
		}
	}

	w.Header().Set("Cache-Control", "public, max-age=300") // 5 minutes
	rend.HTML(w, http.StatusOK, "index", nil)
}

// errorHandler is the handler for HTTP 400 and 500 errors
func (s *implServer) errorHandler(w http.ResponseWriter, r *http.Request) {
	statusCode := http.StatusOK

	path := strings.TrimPrefix(r.URL.Path, "/")

	switch path {
	case "401":
		statusCode = http.StatusUnauthorized
	case "403":
		statusCode = http.StatusForbidden
	case "500":
		statusCode = http.StatusInternalServerError
	case "503":
		statusCode = http.StatusServiceUnavailable
	}

	rend.HTML(w, statusCode, path, nil)
}
