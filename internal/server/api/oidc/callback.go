package oidc

import (
	"net/http"

	"github.com/gorilla/schema"
	"github.com/hashicorp/go-hclog"
	"go.opentelemetry.io/otel/trace"
)

var (
	decoder = schema.NewDecoder()
)

type callbackParams struct {
	State string
	Code  string
}

// callbackHandler handles OIDC callbacks for Single Sign On with Slack
//
// HTTP Method: GET
//
// HTTP Path: /callback
func (s *implServer) callbackHandler(w http.ResponseWriter, r *http.Request) {
	logger := hclog.FromContext(r.Context())
	span := trace.SpanFromContext(r.Context())

	state, err := r.Cookie("state")
	if err != nil {
		span.RecordError(err)
		logger.Warn("failed to retrieve state from cookie", "error", err)
		http.Error(w, "state cookie not found", http.StatusBadRequest)
		return
	}

	nonce, err := r.Cookie("nonce")
	if err != nil {
		span.RecordError(err)
		logger.Warn("failed to retrieve nonce from cookie", "error", err)
		http.Error(w, "nonce not found", http.StatusBadRequest)
		return
	}

	var p callbackParams
	if err := decoder.Decode(&p, r.URL.Query()); err != nil {
		span.RecordError(err)
		logger.Error("invalid callback query parameters", "error", err)
		http.Error(w, "invalid callback query parameters", http.StatusBadRequest)
		return
	}

	if p.State != state.Value {
		logger.Error("state in query parameter does not match state from cookie", "error", err)
		http.Error(w, "state mismatch detected. Please retry the OAuth2 login flow", http.StatusBadRequest)
		return
	}

	// Exchange the code for an ID token
	oauth2Token, err := s.FetchIDToken(r.Context(), p.Code)
	if err != nil {
		span.RecordError(err)
		logger.Error("failed to exchange OAuth2 code for ID token", "error", err)
		http.Error(w, "Failed to exchange OAuth2 code for ID token. Please retry the OAuth2 login flow", http.StatusInternalServerError)
		return
	}

	token, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		span.RecordError(err)
		logger.Error("failed to find id_token in OAuth2 token", "error", err)
		http.Error(w, "id_token is missing in OAuth2 token. Please retry the OAuth2 login flow", http.StatusInternalServerError)
		return
	}

	// Verify the ID token
	idToken, err := s.VerifyIDToken(r.Context(), token)
	if err != nil {
		span.RecordError(err)
		logger.Error("failed to verify id_token", "error", err)
		http.Error(w, "Failed to verify ID Token. Please retry the OAuth2 login flow", http.StatusInternalServerError)
		return
	}

	if idToken.Nonce != nonce.Value {
		span.RecordError(err)
		logger.Error("nonce in id_token did not match nonce from cookie")
		http.Error(w, "nonce mismatch detected. Please retry the OAuth2 login flow", http.StatusBadRequest)
		return
	}

	session, err := s.GetSession(r)
	if err != nil {
		span.RecordError(err)
		http.Redirect(w, r, "/503", http.StatusInternalServerError)
		return
	}

	session.Values["authenticated"] = true
	session.Values["slack_user_id"] = idToken.Subject

	if err := session.Save(r, w); err != nil {
		span.RecordError(err)
		logger.Error("failed to save session", "error", err)
		http.Redirect(w, r, "/503", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile", http.StatusFound)
}
