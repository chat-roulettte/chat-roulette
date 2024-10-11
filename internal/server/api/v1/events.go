package v1

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/chat-roulettte/chat-roulette/internal/bot"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
)

// slackEventHandler handles events sent by the Slack Events API
//
// HTTP Method: POST
//
// HTTP Path: /slack/event
func (s *implServer) slackEventHandler(w http.ResponseWriter, r *http.Request) {
	logger := hclog.FromContext(r.Context())
	span := trace.SpanFromContext(r.Context())

	body, err := io.ReadAll(r.Body)
	if err != nil {
		span.RecordError(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Skip verification of signing secret if running in Dev mode, to ease testing
	if !s.IsDevMode() {
		sv, err := slack.NewSecretsVerifier(r.Header, s.GetSlackSigningSecret())
		if err != nil {
			span.RecordError(err)
			logger.Error("failed to create new SecretsVerifier", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if _, err := sv.Write(body); err != nil {
			span.RecordError(err)
			logger.Error("failed to compute signature", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := sv.Ensure(); err != nil {
			span.RecordError(err)
			logger.Error("failed to verify request from Slack", "error", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		span.RecordError(err)
		logger.Error("failed to parse Slack event", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String(attributes.SlackAPIEvent, eventsAPIEvent.Type),
	)

	switch eventsAPIEvent.Type {
	// Handle url_verification events
	// See: https://api.slack.com/events/url_verification
	case slackevents.URLVerification:
		var resp *slackevents.ChallengeResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			span.RecordError(err)
			logger.Error("failed to unmarshal Slack url_verification event", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(resp.Challenge))
		return

	case slackevents.CallbackEvent:
		innerEvent := eventsAPIEvent.InnerEvent

		span.SetAttributes(
			attribute.String(attributes.SlackEvent, innerEvent.Type),
		)

		switch ev := innerEvent.Data.(type) {

		// Handle member_joined_channel events
		// See: https://api.slack.com/events/member_joined_channel
		case *slackevents.MemberJoinedChannelEvent:
			db := s.GetDB()

			// Onboard Slack channel for chat-roulette when the bot is invited to a channel
			if s.GetSlackBotUserID() == ev.User {

				// Inviter is blank if the bot is added by default to the channel
				if ev.Inviter == "" {
					p := &bot.SyncChannelsParams{
						BotUserID: ev.User,
					}

					if err := bot.QueueSyncChannelsJob(r.Context(), db, p); err != nil {
						span.RecordError(err)
						logger.Error("failed to add job to the queue", "error", err, "job", models.JobTypeSyncChannels.String())
						// Return HTTP 503 so that Slack marks the event as failed to deliver and retries up to 3 times.
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
				} else {
					// Queue a GREET_ADMIN job for the new Slack channel
					p := &bot.GreetAdminParams{
						ChannelID: ev.Channel,
						Inviter:   ev.Inviter,
					}

					if err := bot.QueueGreetAdminJob(r.Context(), db, p); err != nil {
						span.RecordError(err)
						logger.Error("failed to add job to the queue", "error", err, "job", models.JobTypeGreetAdmin.String())
						// Return HTTP 503 so that Slack marks the event as failed to deliver and retries up to 3 times.
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
				}

				w.WriteHeader(http.StatusOK)
				return
			}

			// Verify that the Slack channel is enrolled for chat-roulette
			dbCtx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
			defer cancel()

			var channel models.Channel

			result := db.WithContext(dbCtx).
				Model(&models.Channel{}).
				Select("channel_id").
				Where("channel_id = ?", ev.Channel).First(&channel)

			if result.Error != nil {
				logger.Warn("Slack channel does not exist")
				return
			}

			p := &bot.AddMemberParams{
				ChannelID: ev.Channel,
				UserID:    ev.User,
			}

			if err := bot.QueueAddMemberJob(r.Context(), db, p); err != nil {
				span.RecordError(err)
				logger.Error("failed to add job to the queue", "error", err, "job", models.JobTypeAddMember)
				// Return HTTP 503 so that Slack marks the event as failed to deliver and retries up to 3 times.
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}

		// Handle member_left_channel events
		// See: https://api.slack.com/events/member_left_channel
		case *slackevents.MemberLeftChannelEvent:
			db := s.GetDB()

			// Verify that the Slack channel is enrolled for chat roulette
			dbCtx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
			defer cancel()

			var channel models.Channel

			result := db.WithContext(dbCtx).
				Model(&models.Channel{}).
				Select("channel_id").
				Where("channel_id = ?", ev.Channel).
				First(&channel)

			if result.Error != nil {
				logger.Warn("Slack channel does not exist")
				return
			}

			p := &bot.DeleteMemberParams{
				ChannelID: ev.Channel,
				UserID:    ev.User,
			}

			if err := bot.QueueDeleteMemberJob(r.Context(), db, p); err != nil {
				span.RecordError(err)
				logger.Error("failed to add job to the queue", "error", err, "job", models.JobTypeDeleteMember)
				// Return HTTP 503 so that Slack marks the event as failed to deliver and retries up to 3 times.
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}

		// Handle app_home_opened events
		// See: https://api.slack.com/events/app_home_opened
		case *slackevents.AppHomeOpenedEvent:
			if ev.Tab == "home" {
				p := &bot.AppHomeParams{
					UserID:    ev.User,
					BotUserID: s.GetSlackBotUserID(),
					URL:       s.GetBaseURL(),
				}

				if err := bot.HandleAppHomeEvent(r.Context(), s.GetSlackClient(), s.GetDB(), p); err != nil {
					span.RecordError(err)
					logger.Error("failed to handle app_home_opened event", "error", err)

					// Return HTTP 500 so that Slack marks the event as failed
					// to deliver and retries up to 3 times.
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}

			w.WriteHeader(http.StatusOK)
			return

		default:
			w.WriteHeader(http.StatusOK)
			return
		}
	}
}
