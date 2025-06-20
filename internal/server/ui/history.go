package ui

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sync"
	"time"

	sqlcrypter "github.com/bincyber/go-sqlcrypter"
	"github.com/gorilla/mux"
	"github.com/hashicorp/go-hclog"
	"go.opentelemetry.io/otel/trace"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
)

// matchHistory represents a historical chat roulette match.
// The User represented here is the other participant in the match.
type matchHistory struct {
	User      string
	Image     string
	SlackLink template.URL
	City      sqlcrypter.EncryptedBytes
	Country   sqlcrypter.EncryptedBytes
	Social    sqlcrypter.EncryptedBytes
	IntroDate time.Time
	HasMet    bool
}

func (m *matchHistory) Location() string {
	return fmt.Sprintf("%s, %s", m.City, m.Country)
}

// historyParams are the parameters for the "history.html" template
type historyParams struct {
	ID          string
	Image       string
	DisplayName string
	Title       string
	Workspace   string
	Channel     string
	History     []matchHistory
}

// historyHandler for the chat-roulette history page
//
// HTTP Method: GET
//
// HTTP Path: /history/{CHANNEL-ID}
func (s *implServer) historyHandler(w http.ResponseWriter, r *http.Request) {
	// Identify the channel ID from the URL path
	channelID := mux.Vars(r)["channel_id"]

	logger := hclog.FromContext(r.Context()).With(attributes.SlackChannelID, channelID)
	span := trace.SpanFromContext(r.Context())
	cache := s.GetCache()
	slackClient := s.GetSlackClient()

	// Verify authentication
	session, err := s.GetSession(r)
	if err != nil {
		span.RecordError(err)
		rend.HTML(w, http.StatusInternalServerError, "500", nil)
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

	// Retrieve chat-roulette history for the user for this Slack channel
	db := s.GetDB()

	var history []matchHistory

	dbCtx, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
	defer cancel()

	subQuery := db.Model(&models.Pairing{}).
		Select("match_id").
		Joins("LEFT JOIN members ON members.id = pairings.member_id").
		Where("user_id = ?", slackUserID)

	result := db.WithContext(dbCtx).Model(&models.Pairing{}).
		Select(`
			members.user_id AS user,
			members.channel_id AS channel,
			matches.created_at AS intro_date,
			members.city,
			members.country,
			members.profile_link AS social,
			matches.has_met`,
		).
		Joins("LEFT JOIN members ON members.id = pairings.member_id").
		Joins("LEFT JOIN matches ON matches.id = pairings.match_id").
		Where("pairings.match_id IN (?)", subQuery).
		Where("members.user_id <> ?", slackUserID).
		Where("members.channel_id = ?", channelID).
		Scan(&history)

	if result.Error != nil {
		span.RecordError(result.Error)
		logger.Error("failed to lookup Slack user's chat-roulette history", "error", result.Error)
		rend.HTML(w, http.StatusInternalServerError, "500", nil)
		return
	}

	// Map the Slack channel ID to channel name
	// If this errors, gracefully degrade by displaying the channel ID
	channelName := channelID

	channel, err := lookupSlackChannel(r.Context(), cache, slackClient, channelID)
	if err != nil {
		logger.Warn("failed to lookup Slack channel", "error", err, "slack_channel_id", channelID)
	} else {
		channelName = channel.Name
	}

	// Lookup the Slack users
	var (
		wg       sync.WaitGroup
		errCount int
	)

	for i, e := range history {
		wg.Add(1)

		go func(i int, e matchHistory) {
			defer wg.Done()

			// Set the Slack link before converting userID to display name
			//
			// This is a safe link as the data comes from Slack and not the user.
			history[i].SlackLink = template.URL(generateSlackLink(teamInfo.ID, e.User)) //nolint:gosec

			// Map the Slack user ID to display name
			match, err := lookupSlackUser(r.Context(), cache, slackClient, e.User)
			if err != nil {
				logger.Error("failed to lookup Slack user", "error", err, "slack_user_id", channelID)
				errCount++
			} else {
				name := match.Profile.DisplayName
				if name == "" {
					name = match.Profile.RealName
				}

				history[i].User = name
				history[i].Image = match.Profile.Image192
			}

			// Ensure social links include the scheme
			u, _ := url.Parse(history[i].Social.String()) // We already know it's a valid URL
			u.Scheme = "https"
			history[i].Social = sqlcrypter.NewEncryptedBytes(u.String())
		}(i, e)
	}

	wg.Wait()

	// Raise HTTP 500 if too many errors encountered
	if errCount > 0 && errCount == len(history) {
		span.RecordError(fmt.Errorf("failed to lookup Slack channels or users"))
		logger.Error("failed to lookup Slack channels or users", "error", "too many errors calling Slack")
		rend.HTML(w, http.StatusInternalServerError, "500", nil)
		return
	}

	// Render the HTML page
	p := historyParams{
		ID:          slackUserID,
		DisplayName: slackUser.Profile.DisplayName,
		Title:       slackUser.Profile.Title,
		Image:       slackUser.Profile.Image192,
		Workspace:   teamInfo.Name,
		Channel:     channelName,
		History:     history,
	}

	w.Header().Set("Cache-Control", "no-cache")
	rend.HTML(w, http.StatusOK, "history", p)
}

// generateSlackLink generates a deep link to a Slack user's profile
func generateSlackLink(teamID, userID string) string {
	return fmt.Sprintf("slack://user?team=%s&id=%s", teamID, userID)
}
