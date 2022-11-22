package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/slack-go/slack"
	"go.opentelemetry.io/otel/trace"

	"github.com/chat-roulettte/chat-roulette/internal/iox"
	"github.com/chat-roulettte/chat-roulette/internal/tzx"
)

// slackOptionsHandler handles select options sent by the Slack Interactions API
// See: https://api.slack.com/reference/block-kit/block-elements#external_select
//
// HTTP Method: POST
//
// HTTP Path: /slack/options
func (s *implServer) slackOptionsHandler(w http.ResponseWriter, r *http.Request) {
	logger := hclog.FromContext(r.Context())
	span := trace.SpanFromContext(r.Context())

	// Verify that the request is sent from Slack by validating the X-Slack-Signature header.
	// See: https://api.slack.com/authentication/verifying-requests-from-slack
	//
	// To ease testing, skip verification if running in Dev mode.
	b, err := iox.ReadAndReset(&r.Body)
	if err != nil {
		span.RecordError(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !s.IsDevMode() {
		sv, err := slack.NewSecretsVerifier(r.Header, s.GetSlackSigningSecret())
		if err != nil {
			span.RecordError(err)
			logger.Error("failed to create new SecretsVerifier", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if _, err := sv.Write(b); err != nil {
			span.RecordError(err)
			logger.Error("failed to compute signature", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := sv.Ensure(); err != nil {
			span.RecordError(err)
			logger.Error("failed to verify request is from Slack", "error", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	// Verify POST body contains "payload"
	if err := r.ParseForm(); err != nil {
		span.RecordError(err)
		logger.Error("failed to parse HTTP request as form", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	payload := r.PostFormValue(SlackHTTPRequestParameter)
	if payload == "" {
		logger.Error("failed to read payload in HTTP request body")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Unmarshal the JSON in the payload
	var interaction slack.InteractionCallback

	if err := json.Unmarshal([]byte(payload), &interaction); err != nil {
		span.RecordError(err)
		logger.Error("failed to unmarshal Slack interaction", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if interaction.Type != slack.InteractionTypeBlockSuggestion {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch interaction.BlockID {
	case "onboarding-country":
		// Options for onboarding modal's external_select
		countries := tzx.GetCountriesWithPrefix(interaction.Value)

		response := slack.OptionsResponse{
			Options: make([]*slack.OptionBlockObject, 0),
		}

		for _, v := range countries {
			parts := strings.Split(v.Name, ",")
			countryName := parts[0]

			response.Options = append(response.Options, &slack.OptionBlockObject{
				Value: countryName,
				Text: &slack.TextBlockObject{
					Type:  slack.PlainTextType,
					Text:  fmt.Sprintf(":flag-%s: %s", v.Code, countryName),
					Emoji: true,
				},
			})
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(&response); err != nil {
			span.RecordError(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

	default:
		w.WriteHeader(http.StatusOK)
	}
}
