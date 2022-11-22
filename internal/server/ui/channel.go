package ui

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/hashicorp/go-hclog"
	"go.opentelemetry.io/otel/trace"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
)

// channelAdminParams are the parameters for the "admin.html" template
type channelAdminParams struct {
	ID          string
	Image       string
	DisplayName string
	Title       string
	Workspace   string
	ChannelName string
	Channel     *models.Channel
	MinDate     time.Time
}

// channelAdminHandler for the channel admin page
//
// HTTP Method: GET
//
// HTTP Path: /channel/<CHANNEL-ID>
func (s *implServer) channelAdminHandler(w http.ResponseWriter, r *http.Request) {
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

	// Validate that the user in the request is the inviter/admin for the Slack channel
	db := s.GetDB()

	dbCtx, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
	defer cancel()

	var channel models.Channel
	result := db.WithContext(dbCtx).
		Where("channel_id = ?", channelID).
		First(&channel)

	if result.Error != nil {
		message := "failed to retrieve inviter from the database"
		span.RecordError(result.Error)
		logger.Error(message, "error", result.Error)
		http.Redirect(w, r, "/500", http.StatusFound)
		return
	}

	if channel.Inviter != slackUserID {
		logger.Error("Slack user is not authorized to modify settings for chat-roulette channel")
		http.Redirect(w, r, "/403", http.StatusFound)
		return
	}

	// Retrieve the info for the Slack channel
	// If this errors, gracefully degrade by displaying the channel ID
	channelName := channelID

	slackChannel, err := lookupSlackChannel(r.Context(), cache, slackClient, channelID)
	if err != nil {
		logger.Warn("failed to lookup Slack channel", "error", err)
	} else {
		channelName = slackChannel.Name
	}

	// Render the template
	p := channelAdminParams{
		ID:          slackUserID,
		DisplayName: slackUser.Profile.DisplayName,
		Title:       slackUser.Profile.Title,
		Image:       slackUser.Profile.Image192,
		Workspace:   teamInfo.Name,
		Channel:     &channel,
		ChannelName: channelName,
		MinDate:     time.Now().Add(-(24 * time.Hour)),
	}

	w.Header().Set("Cache-Control", "no-cache")
	rend.HTML(w, http.StatusOK, "channel", p)
}
