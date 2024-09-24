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
}

// HandleAppHomeEvent handles the app_home_opened event and publishes the view for the App Home.
func HandleAppHomeEvent(ctx context.Context, client *slack.Client, db *gorm.DB, p *AppHomeParams) error {
	// Retrieve chat-roulette enabled Slack channels from the database
	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	var channels []models.Channel
	if err := db.WithContext(dbCtx).Find(&channels).Error; err != nil {
		return errors.Wrap(err, "failed to retrieve chat roulette channels")
	}

	// Render template
	t := appHomeTemplate{
		BotUserID: p.BotUserID,
		AppURL:    p.URL,
		Channels:  channels,
	}

	content, err := renderTemplate(appHomeTemplateFilename, t)
	if err != nil {
		return errors.Wrap(err, "failed to render template")
	}

	var view slack.HomeTabViewRequest

	if err := json.Unmarshal([]byte(content), &view); err != nil {
		return errors.Wrap(err, "failed to unmarshal JSON")
	}

	if _, err = client.PublishViewContext(ctx, p.UserID, view, ""); err != nil {
		return errors.Wrap(err, "failed to publish AppHome view")
	}

	return nil
}
