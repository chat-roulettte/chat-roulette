package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/hashicorp/go-hclog"
	"go.opentelemetry.io/otel/trace"

	"github.com/chat-roulettte/chat-roulette/internal/bot"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/isx"
)

// updateChannelHandler handles updating a channel's settings (interval, weekday, hour)
//
// HTTP Method: POST
//
// HTTP Path: /channel
func (s *implServer) updateChannelHandler(w http.ResponseWriter, r *http.Request) {
	logger := hclog.FromContext(r.Context())
	span := trace.SpanFromContext(r.Context())

	// Verify that the user is authenticated
	session, err := s.GetSession(r)
	if err != nil {
		span.RecordError(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Unmarshal request body to JSON
	var p *bot.UpdateChannelParams
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		span.RecordError(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Validate the request
	if err := validation.ValidateStruct(p,
		validation.Field(&p.ChannelID, validation.Required, is.Alphanumeric),
		validation.Field(&p.Interval, validation.Required, validation.By(isx.Interval)),
		validation.Field(&p.Weekday, validation.Required, validation.By(isx.Weekday)),
		validation.Field(&p.Hour, validation.Min(0), validation.Max(23)),
		validation.Field(&p.NextRound, validation.Required, validation.By(isx.NextRoundDate)),
	); err != nil {
		span.RecordError(err)

		response := ErrResponse{
			Error: fmt.Sprintf("Validation failed: %s", err),
		}

		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response) //nolint:errcheck
		return
	}

	// Verify that the user is authorized to modify the chat-roulette channel
	slackUserID, ok := session.Values["slack_user_id"].(string)
	if !ok {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	db := s.GetDB()

	dbCtx, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
	defer cancel()

	var inviter string
	result := db.WithContext(dbCtx).
		Model(&models.Channel{}).
		Select("inviter").
		Where("channel_id = ?", p.ChannelID).
		First(&inviter)

	if result.Error != nil {
		message := "failed to retrieve inviter from the database"
		logger.Error(message, "error", result.Error)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if inviter != slackUserID {
		span.RecordError(ErrAuthzFailed)
		logger.Error("failed to update channel settings", "error", "user is not authorized to modify the chat-roulette channel")

		response := ErrResponse{
			Error: ErrAuthzFailed.Error(),
		}

		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(response) //nolint:errcheck
		return
	}

	// Schedule an UPDATE_CHANNEL job to update the channel's settings (interval, weekday, hour)
	// for chat-roulette. bot.UpdateChannel() could be directly called here,
	// however scheduling a background job will ensure it is reliably executed.
	if err := bot.QueueUpdateChannelJob(r.Context(), db, p); err != nil {
		span.RecordError(err)
		logger.Error("failed to add job to the queue", "error", "job", models.JobTypeUpdateChannel.String())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
