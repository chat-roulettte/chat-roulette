package bot

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
)

const (
	appHomeTemplateFilename = "app_home.json.tmpl"
)

// AppHomeParams is the parameters for handling app_home_opened events
type AppHomeParams struct {
	BotUserID string
	URL       string
	UserID    string
	View      slack.View
}

type appHomeTemplate struct {
	BotUserID string
	AppURL    string
	Channels  []models.Channel
	IsAppUser bool
}

// HandleAppHomeEvent handles the app_home_opened event and publishes the view for the App Home.
func HandleAppHomeEvent(ctx context.Context, client *slack.Client, db *gorm.DB, p *AppHomeParams) error {
	// Retrieve chat-roulette enabled Slack channels from the database
	dbCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	var channels []models.Channel
	if err := db.WithContext(dbCtx).Find(&channels).Error; err != nil {
		return errors.Wrap(err, "failed to retrieve chat roulette channels")
	}

	// Check if the user exists in the database (ie, user is a member of a Chat Roulette channel). Ignore errors
	var count int64
	_ = db.WithContext(dbCtx).Model(&models.Member{}).Where("user_id = ?", p.UserID).Count(&count)

	// Render template
	t := appHomeTemplate{
		BotUserID: p.BotUserID,
		AppURL:    p.URL,
		Channels:  channels,
		IsAppUser: count == 1,
	}

	content, err := renderTemplate(appHomeTemplateFilename, t)
	if err != nil {
		return errors.Wrap(err, "failed to render template")
	}

	var view slack.HomeTabViewRequest
	if err := json.Unmarshal([]byte(content), &view); err != nil {
		return errors.Wrap(err, "failed to unmarshal JSON")
	}

	req := slack.PublishViewContextRequest{
		UserID: p.UserID,
		View:   view,
	}

	if _, err = client.PublishViewContext(ctx, req); err != nil {
		return errors.Wrap(err, "failed to publish AppHome view")
	}

	return nil
}
