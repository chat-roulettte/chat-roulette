package oidc

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"

	"github.com/chat-roulettte/chat-roulette/internal/server"
	"github.com/chat-roulettte/chat-roulette/internal/server/api"
)

type implServer struct {
	*server.Server
}

const RoutesPrefix = "/oidc/"

// RegisterRoutes registers health routes on the given server
func RegisterRoutes(s *server.Server) {
	i := implServer{s}

	routes := []api.Route{
		{Path: "callback", Methods: []string{"GET"}, Func: i.callbackHandler},
		{Path: "login", Methods: []string{"GET"}, Func: i.loginHandler},
		{Path: "logout", Methods: []string{"GET"}, Func: i.logoutHandler},
	}

	api.RegisterRoutes(s.GetMux(), RoutesPrefix, routes)
}

// generateRandomURLSafeString generates a random 16-byte url safe string.
func generateRandomURLSafeString() (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// setCallbackCookies sets the state and nonce cookies for the OAuth2 callback
func setCallbackCookies(w http.ResponseWriter, state, nonce string) {

	cookies := map[string]string{
		"state": state,
		"nonce": nonce,
	}

	for k, v := range cookies {
		c := &http.Cookie{
			Name:     k,
			Value:    v,
			Path:     "/",
			MaxAge:   300, // 5 minutes is sufficient time to complete auth
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		}

		http.SetCookie(w, c)
	}
}
