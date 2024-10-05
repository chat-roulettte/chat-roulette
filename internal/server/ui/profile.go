package ui

import (
	"context"
	"net/http"
	"time"

	"github.com/bincyber/go-sqlcrypter"
	"github.com/hashicorp/go-hclog"
	"go.opentelemetry.io/otel/trace"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
)

// profileChannel represents a chat-roulette channel
// that the Slack user is a member of.
type profileChannel struct {
	ChannelID      string
	ChannelName    string
	Inviter        string
	ConnectionMode string
	Interval       models.IntervalEnum
	Weekday        time.Weekday
	NextRound      time.Time
	Participants   int32

	// ProfileType is used to determine if the user has completed onboarding,
	// since collecting profile_type is the last required step of onboarding.
	ProfileType sqlcrypter.EncryptedBytes

	// Admin is used to limit displaying the channel edit button only
	// for inviters/admins.
	Admin bool
}

// profileParams are the parameters for the "profile.html" template
type profileParams struct {
	ID          string
	Image       string
	DisplayName string
	Title       string
	Workspace   string
	Channels    []profileChannel
}

// profileHandler for the profile page
//
// HTTP Method: GET
//
// HTTP Path: /profile
func (s *implServer) profileHandler(w http.ResponseWriter, r *http.Request) {
	logger := hclog.FromContext(r.Context())
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

	// Retrieve the Slack user's chat roulette channels
	db := s.GetDB()

	var profileChannels []profileChannel

	dbCtx, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
	defer cancel()

	subquery := db.Model(&models.Member{}).
		Select("COUNT(is_active)").
		Where("channel_id = channels.channel_id AND is_active")

	result := db.WithContext(dbCtx).
		Model(&models.Channel{}).
		Select("channels.channel_id, channels.inviter, channels.interval, channels.weekday, channels.next_round, channels.connection_mode, members.profile_type, (?) AS participants", subquery).
		Joins("LEFT JOIN members on channels.channel_id = members.channel_id").
		Where("user_id = ?", slackUserID).
		Scan(&profileChannels)

	if result.Error != nil {
		span.RecordError(result.Error)
		logger.Error("failed to lookup Slack user's chat roulette channels", "error", result.Error)
		http.Redirect(w, r, "/503", http.StatusFound)
		return
	}

	// Map the Slack channel ID to channel name and set admin flag if inviter
	for i, c := range profileChannels {
		channel, err := lookupSlackChannel(r.Context(), cache, slackClient, c.ChannelID)
		if err != nil {
			span.RecordError(err)
			logger.Error("failed to lookup Slack channel", "error", err, attributes.SlackChannelID, c.ChannelID)
			rend.HTML(w, http.StatusInternalServerError, "500", nil)
			return
		}

		profileChannels[i].ChannelName = channel.Name

		if c.Inviter == slackUserID {
			profileChannels[i].Admin = true
		}
	}

	p := profileParams{
		ID:          slackUserID,
		DisplayName: slackUser.Profile.DisplayName,
		Title:       slackUser.Profile.Title,
		Image:       slackUser.Profile.Image192,
		Workspace:   teamInfo.Name,
		Channels:    profileChannels,
	}

	w.Header().Set("Cache-Control", "no-cache")
	rend.HTML(w, http.StatusOK, "profile", p)
}
