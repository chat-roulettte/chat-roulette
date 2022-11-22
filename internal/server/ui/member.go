package ui

import (
	"context"
	"net/http"
	"time"

	"github.com/go-playground/tz"
	"github.com/gorilla/mux"
	"github.com/hashicorp/go-hclog"
	"go.opentelemetry.io/otel/trace"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
	"github.com/chat-roulettte/chat-roulette/internal/tzx"
)

// memberProfileParams are the parameters for the "member.html" template
type memberProfileParams struct {
	ID          string
	Image       string
	DisplayName string
	Title       string
	Workspace   string
	Channel     string
	Member      *models.Member
	Countries   []tz.Country
	Zones       []tz.Zone
}

// memberProfileHandler for displaying and updating a user's profile settings
//
// HTTP Method: GET
//
// HTTP Path: /profile/{channel_id}
func (s *implServer) memberProfileHandler(w http.ResponseWriter, r *http.Request) {
	// Identify the channel ID from the URL path
	channelID := mux.Vars(r)["channel_id"]

	logger := hclog.FromContext(r.Context()).With(attributes.SlackChannelID, channelID)
	span := trace.SpanFromContext(r.Context())
	cache := s.GetCache()
	slackClient := s.GetSlackClient()

	session, err := s.GetSession(r)
	if err != nil {
		span.RecordError(err)
		http.Redirect(w, r, "/503", http.StatusFound)
		return
	}

	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Redirect(w, r, "/401", http.StatusFound)
		return
	}

	slackUserID := session.Values["slack_user_id"].(string) //nolint:errcheck

	// Retrieve the info for the Slack user
	slackUser, err := lookupSlackUser(r.Context(), cache, slackClient, slackUserID)
	if err != nil {
		span.RecordError(err)
		logger.Error("failed to lookup Slack user", "error", err, attributes.SlackUserID, slackUserID)
		rend.HTML(w, http.StatusInternalServerError, "500", nil)
		return
	}

	// Retrieve the info for the Slack workspace
	teamInfo, err := lookupSlackWorkspace(r.Context(), cache, slackClient)
	if err != nil {
		span.RecordError(err)
		logger.Error("failed to lookup Slack workspace", "error", err)
		rend.HTML(w, http.StatusInternalServerError, "500", nil)
		return
	}

	// Retrieve the info for the Slack channel
	// If this errors, gracefully degrade by displaying the channel ID
	channelName := channelID

	channel, err := lookupSlackChannel(r.Context(), cache, slackClient, channelID)
	if err != nil {
		logger.Warn("failed to lookup Slack channel", "error", err)
	} else {
		channelName = channel.Name
	}

	// Retrieve the user's current settings from the DB
	db := s.GetDB()

	var member models.Member

	dbCtx, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
	defer cancel()

	result := db.WithContext(dbCtx).
		Where("user_id = ?", slackUserID).
		Where("channel_id = ?", channelID).
		First(&member)

	if result.Error != nil {
		span.RecordError(result.Error)
		logger.Error("failed to retrieve user from the database", "error", result.Error)
		rend.HTML(w, http.StatusInternalServerError, "500", nil)
		return
	}

	// Get the timezones for the user's country
	country, ok := tzx.GetCountryByName(member.Country.String())
	if !ok {
		logger.Warn("failed to lookup country")
		rend.HTML(w, http.StatusInternalServerError, "500", nil)
		return
	}

	zones := country.Zones

	// Render the template
	p := memberProfileParams{
		ID:          slackUserID,
		DisplayName: slackUser.Profile.DisplayName,
		Title:       slackUser.Profile.Title,
		Image:       slackUser.Profile.Image192,
		Workspace:   teamInfo.Name,
		Channel:     channelName,
		Member:      &member,
		Countries:   tz.GetCountries(),
		Zones:       zones,
	}

	w.Header().Set("Cache-Control", "no-cache")
	rend.HTML(w, http.StatusOK, "member", p)
}
