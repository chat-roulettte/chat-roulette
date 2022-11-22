package oidc

import (
	"net/http"

	"github.com/hashicorp/go-hclog"
	"go.opentelemetry.io/otel/trace"
)

// loginHandler handles logins for Single Sign On with Slack
//
// HTTP Method: GET
//
// HTTP Path: /login
func (s *implServer) loginHandler(w http.ResponseWriter, r *http.Request) {
	span := trace.SpanFromContext(r.Context())

	state, err := generateRandomURLSafeString()
	if err != nil {
		span.RecordError(err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	nonce, err := generateRandomURLSafeString()
	if err != nil {
		span.RecordError(err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	setCallbackCookies(w, state, nonce)

	url := s.GenerateAuthCodeURL(state, nonce)

	http.Redirect(w, r, url, http.StatusFound)
}

// logoutHandler handles logging out of the UI
//
// HTTP Method: GET
//
// HTTP Path: /logout
func (s *implServer) logoutHandler(w http.ResponseWriter, r *http.Request) {
	logger := hclog.FromContext(r.Context())
	span := trace.SpanFromContext(r.Context())
	cache := s.GetCache()

	session, err := s.GetSession(r)
	if err != nil {
		span.RecordError(err)
		http.Redirect(w, r, "/503", http.StatusInternalServerError)
		return
	}

	if !session.IsNew {
		slackUserID := session.Values["slack_user_id"].(string) //nolint:errcheck

		session.Options.MaxAge = -1
		session.Values["authenticated"] = false

		if err := session.Save(r, w); err != nil {
			span.RecordError(err)
			logger.Error("failed to save session", "error", err)
			http.Redirect(w, r, "/503", http.StatusFound)
			return
		}

		cache.Del(slackUserID)
		logger.Debug("deleted key from cache", "key", slackUserID)
	}

	http.Redirect(w, r, "/", http.StatusFound)
}
