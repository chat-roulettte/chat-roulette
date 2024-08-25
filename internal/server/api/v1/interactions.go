package v1

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/slack-go/slack"
	"go.opentelemetry.io/otel/trace"

	"github.com/chat-roulettte/chat-roulette/internal/bot"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/iox"
)

const (
	SlackHTTPRequestParameter = "payload"
)

// slackInteractionHandler handles interactions sent by the Slack Interactions API
//
// HTTP Method: POST
//
// HTTP Path: /slack/interaction
func (s *implServer) slackInteractionHandler(w http.ResponseWriter, r *http.Request) {
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

	switch interaction.Type {
	case slack.InteractionTypeViewSubmission:

		switch interaction.View.CallbackID {
		case "onboarding-modal":
			// Respond to the HTTP request with the new view
			body, err := bot.RenderOnboardingLocationView(r.Context(), &interaction, s.GetBaseURL())
			if err != nil {
				span.RecordError(err)
				logger.Error("failed to load onboarding location template", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(body)
			return

		case "onboarding-location":
			// Parse the contents of the view and queue UPDATE_MEMBER job
			if err := bot.UpsertMemberLocationInfo(r.Context(), s.GetDB(), &interaction); err != nil {
				span.RecordError(err)
				logger.Error("failed to upsert Slack member's location info", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// Respond to the HTTP request with the new view
			body, err := bot.RenderOnboardingTimezoneView(r.Context(), &interaction, s.GetBaseURL())
			if err != nil {
				span.RecordError(err)
				logger.Error("failed to load onboarding timezone template", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(body)
			return

		case "onboarding-timezone":
			// Parse the contents of the view and queue UPDATE_MEMBER job
			if err := bot.UpsertMemberTimezoneInfo(r.Context(), s.GetDB(), &interaction); err != nil {
				span.RecordError(err)
				logger.Error("failed to upsert Slack member's timezone info", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// Respond to the HTTP request with the new view
			body, err := bot.RenderOnboardingGenderView(r.Context(), &interaction, s.GetBaseURL())
			if err != nil {
				span.RecordError(err)
				logger.Error("failed to load onboarding gender template", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(body)
			return

		case "onboarding-gender":
			// Parse the contents of the view and queue UPDATE_MEMBER job
			if err := bot.UpsertMemberGenderInfo(r.Context(), s.GetDB(), &interaction); err != nil {
				span.RecordError(err)
				logger.Error("failed to upsert Slack member's gender info", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// Respond to the HTTP request with the new view
			body, err := bot.RenderOnboardingProfileView(r.Context(), &interaction, s.GetBaseURL())
			if err != nil {
				span.RecordError(err)
				logger.Error("failed to load onboarding profile template", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(body)
			return

		case "onboarding-profile":
			// Validate the Slack member's profile data
			if err := bot.ValidateMemberProfileInfo(r.Context(), &interaction); err != nil {
				span.RecordError(err)

				response := &slack.ViewSubmissionResponse{
					ResponseAction: slack.RAErrors,
					Errors: map[string]string{
						"onboarding-profile-link": err.Error(),
					},
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(response); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				return
			}

			// Parse the contents of the view and queue UPDATE_MEMBER job
			if err := bot.UpsertMemberProfileInfo(r.Context(), s.GetDB(), &interaction); err != nil {
				span.RecordError(err)
				logger.Error("failed to upsert Slack member's profile info", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// Respond to the HTTP request with the new view
			body, err := bot.RenderOnboardingCalendlyView(r.Context(), &interaction, s.GetBaseURL())
			if err != nil {
				span.RecordError(err)
				logger.Error("failed to load onboarding profile template", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(body)
			return

		case "onboarding-calendly":
			// Extract the Calendly link from the view state
			calendlyLink := strings.ToLower(interaction.View.State.Values["onboarding-calendly"]["onboarding-calendly"].Value)

			if calendlyLink != "" {
				// Validate the Slack user's calendly link, if provided
				if err := bot.ValidateMemberCalendlyLink(r.Context(), calendlyLink); err != nil {
					span.RecordError(err)

					response := &slack.ViewSubmissionResponse{
						ResponseAction: slack.RAErrors,
						Errors: map[string]string{
							"onboarding-calendly": err.Error(),
						},
					}

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if err := json.NewEncoder(w).Encode(response); err != nil {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					return
				}

				// Parse the contents of the view and queue UPDATE_MEMBER job
				if err := bot.UpsertMemberCalendlyLink(r.Context(), s.GetDB(), &interaction); err != nil {
					span.RecordError(err)
					logger.Error("failed to upsert Slack member's Calendly link", "error", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}

			// Update the original GREET_MEMBER message to remove the Opt-In button
			if err := bot.RespondGreetMemberWebhook(r.Context(), s.GetHTTPClient(), &interaction); err != nil {
				span.RecordError(err)
				logger.Error("failed to respond to GREET_MEMBER webhook", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// Close the modal
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"response_action": "clear"}`))
			return
		}

	case slack.InteractionTypeBlockActions:
		if len(interaction.ActionCallback.BlockActions) > 0 {
			actionID := interaction.ActionCallback.BlockActions[0].ActionID

			// Don't handle link buttons
			// See: https://github.com/slackapi/node-slack-sdk/issues/869
			if actionID == "link" {
				w.WriteHeader(http.StatusOK)
				return
			}

			jobType, err := models.ExtractJobFromActionID(actionID)
			if err != nil {
				span.RecordError(err)
				logger.Error("invalid job type in interactivity event", "error", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			switch jobType { //nolint:gocritic
			case models.JobTypeGreetMember:
				// Handle GREET_MEMBER button
				if err := bot.HandleGreetMemberButton(r.Context(), s.GetSlackClient(), &interaction); err != nil {
					span.RecordError(err)
					logger.Error("failed to handle greet member button", "error", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

			case models.JobTypeCheckPair:
				// handle CHECK_PAIR buttons
				if err := bot.HandleCheckPairButtons(r.Context(), s.GetHTTPClient(), s.GetDB(), &interaction); err != nil {
					span.RecordError(err)
					logger.Error("something went wrong", "error", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
		}
	}

	// Return response to Slack
	// This has a 3 second SLO
	w.WriteHeader(http.StatusOK)
}
