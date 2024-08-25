package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/bincyber/go-sqlcrypter"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"go.opentelemetry.io/otel/trace"

	"github.com/chat-roulettte/chat-roulette/internal/bot"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/isx"
)

// TODO: convert received json to bot.UpdateMemberParams struct
type updateMemberRequest struct {
	ChannelID           string `json:"channel_id"`
	UserID              string `json:"user_id"`
	Country             string `json:"country,omitempty"`
	City                string `json:"city,omitempty"`
	Timezone            string `json:"timezone,omitempty"`
	ProfileType         string `json:"profile_type,omitempty"`
	ProfileLink         string `json:"profile_link,omitempty"`
	CalendlyLink        string `json:"calendly_link,omitempty"`
	IsActive            bool   `json:"is_active"`
	HasGenderPreference bool   `json:"has_gender_preference"`
}

// updateMemberHandler handles updating a member's profile settings
//
// HTTP Method: POST
//
// HTTP Path: /member
func (s *implServer) updateMemberHandler(w http.ResponseWriter, r *http.Request) {
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
	var req *updateMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.RecordError(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Validate the request
	var result *multierror.Error

	if err := validation.ValidateStruct(req,
		validation.Field(&req.ChannelID, validation.Required, is.Alphanumeric),
		validation.Field(&req.UserID, validation.Required, is.Alphanumeric),
		validation.Field(&req.Country, validation.Required, validation.By(isx.Country)),
		validation.Field(&req.City, validation.Required),
		validation.Field(&req.Timezone, validation.Required),
		validation.Field(&req.ProfileType, validation.Required, validation.By(isx.ProfileType)),
		validation.Field(&req.ProfileLink, validation.Required, is.URL),
		validation.Field(&req.CalendlyLink, validation.By(isx.CalendlyLink)),
	); err != nil {
		result = multierror.Append(result, err)
	}

	if err := isx.ValidProfileLink(req.ProfileType, req.ProfileLink); err != nil {
		result = multierror.Append(result, err)
	}

	if result.ErrorOrNil() != nil {
		result.ErrorFormat = func(errs []error) string {
			s := make([]string, len(errs))

			for i, err := range errs {
				s[i] = fmt.Sprintf("%s", err)
			}

			return strings.Join(s, ",")
		}

		span.RecordError(result)

		response := ErrResponse{
			Error: fmt.Sprintf("Validation failed: %s", result),
		}

		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response) //nolint:errcheck
		return
	}

	// Verify that the user is only updating their own settings
	slackUserID, ok := session.Values["slack_user_id"].(string)
	if !ok || req.UserID != slackUserID {
		span.RecordError(ErrAuthzFailed)
		logger.Error("failed to update profile settings", "error", "user is not authorized to modify others' settings")

		response := ErrResponse{
			Error: ErrAuthzFailed.Error(),
		}

		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(response) //nolint:errcheck
		return
	}

	// Convert updateMemberRequest struct to bot.UpdateMemberParams struct
	p := &bot.UpdateMemberParams{
		ChannelID:           req.ChannelID,
		UserID:              req.UserID,
		IsActive:            req.IsActive,
		HasGenderPreference: req.HasGenderPreference,
	}

	if req.Country != "" {
		p.Country = sqlcrypter.NewEncryptedBytes(req.Country)
	}

	if req.City != "" {
		p.City = sqlcrypter.NewEncryptedBytes(req.City)
	}

	if req.Timezone != "" {
		p.Timezone = sqlcrypter.NewEncryptedBytes(req.Timezone)
	}

	if req.ProfileType != "" {
		p.ProfileType = sqlcrypter.NewEncryptedBytes(req.ProfileType)
	}

	if req.ProfileLink != "" {
		p.ProfileLink = sqlcrypter.NewEncryptedBytes(req.ProfileLink)
	}

	if req.CalendlyLink != "" {
		p.CalendlyLink = sqlcrypter.NewEncryptedBytes(req.CalendlyLink)
	}

	// Schedule an UPDATE_MEMBER job to update the member's participation status
	// for chat roulette. bot.UpdateMember() could be directly called here,
	// however scheduling a background job will ensure it is reliably executed.
	if err := bot.QueueUpdateMemberJob(r.Context(), s.GetDB(), p); err != nil {
		logger.Error("failed to add job to the queue", "error", err, "job", models.JobTypeUpdateMember.String())
		span.RecordError(err)

		response := ErrResponse{
			Error: "Something went wrong. Please retry your request",
		}

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response) //nolint:errcheck
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
