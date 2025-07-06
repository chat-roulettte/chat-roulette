package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
)

const (
	markInactiveTemplateFilename = "mark_inactive.json.tmpl"
)

// markInactiveTemplate is used with templates/mark_inactive.json.tmpl
type markInactiveTemplate struct {
	ChannelID string
	UserID    string
	NextRound time.Time
	AppHome   string
}

// MarkInactiveParams are the parameters for the MARK_INACTIVE job.
type MarkInactiveParams struct {
	ChannelID    string    `json:"channel_id"`
	NextRound    time.Time `json:"next_round"`
	MatchID      int32     `json:"match_id"`
	Participants []string  `json:"participants"`
}

// MarkInactive marks a user as inactive if they do not send any messages in the group chat.
func MarkInactive(ctx context.Context, db *gorm.DB, client *slack.Client, p *MarkInactiveParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
		attributes.MatchID, p.MatchID,
	)

	// Retrieve the Group DM ID
	var match models.Match

	dbCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	result := db.WithContext(dbCtx).Model(&models.Match{}).
		Where("id = ?", p.MatchID).
		Find(&match)

	if result.Error != nil {
		message := "failed to retrieve Group DM ID"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	// Retrieve messages from the group chat
	slackCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	history, err := client.GetConversationHistoryContext(
		slackCtx,
		&slack.GetConversationHistoryParameters{
			ChannelID: match.MpimID,
			Oldest:    fmt.Sprintf("%d", match.CreatedAt.Unix()),
			Inclusive: true,
		},
	)

	if err != nil {
		message := "failed to retrieve message history from Group DM"
		logger.Error(message, "error", err, "mpim_id", match.MpimID)
		return errors.Wrap(err, message)
	}

	// Generate the deep link to the app's AppHome
	teamID, appID, err := GetBotTeamAppIDs(ctx, client)
	if err != nil {
		message := "failed to retrieve Slack team ID and app ID"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	appHome := generateAppHomeDeepLink(teamID, appID)

	// Check if both members have sent a message in the group chat
	// and/or clicked on the CHECK_PAIR buttons.
	senders := make(map[string]struct{})

	var builder strings.Builder

	for _, message := range history.Messages {
		senders[message.User] = struct{}{}

		builder.WriteString(message.Text)
		builder.WriteString(" ") // Space to avoid word merging
	}

	re := regexp.MustCompile("Did you get a chance to connect")
	found := re.FindAllStringIndex(builder.String(), -1)

	switch len(found) {
	case 1:
		logger.Debug("1 CHECK_PAIR button has not been clicked")
	case 2:
		logger.Debug("2 CHECK_PAIR buttons have not been clicked")
	}

	// Mark the users as inactive if they have not sent any messages
	// in the group chat or responded to the CHECK_PAIR job(s).
	var marked int

	for _, memberID := range p.Participants {
		if _, ok := senders[memberID]; ok && len(found) < 2 {
			// Skip the member since they have sent at least 1 message
			// in the group chat and clicked on at least 1 CHECK_PAIR button.
			continue
		}

		logger = logger.With(attributes.SlackUserID, memberID)

		// Mark member as inactive in the database
		if err := db.Model(&models.Member{}).Where("channel_id = ? AND user_id = ?", p.ChannelID, memberID).Update("is_active", false).Error; err != nil {
			message := "failed to mark Slack user as inactive in the database"
			logger.Error(message, "error", err)
			return errors.Wrap(err, message)
		}
		logger.Debug("marked Slack user as inactive in the database")
		marked++

		// Increment the counter of users marked as inactive
		if err := db.Model(&models.Round{}).Where("id = ?", match.RoundID).UpdateColumn("inactive_users", gorm.Expr("inactive_users + ?", 1)).Error; err != nil {
			message := "failed to increment the count of users marked inactive"
			logger.Error(message, "error", err)
			return errors.Wrap(err, message)
		}

		// Render the template for the Slack message
		t := markInactiveTemplate{
			ChannelID: p.ChannelID,
			UserID:    memberID,
			NextRound: p.NextRound,
			AppHome:   appHome,
		}

		content, err := renderTemplate("mark_inactive.json.tmpl", t)
		if err != nil {
			return errors.Wrap(err, "failed to render template")
		}

		var view slack.View
		if err := json.Unmarshal([]byte(content), &view); err != nil {
			message := "failed to unmarshal JSON template to slack.View"
			logger.Error(message, "error", err)
			return errors.Wrap(err, message)
		}

		// Open a Slack DM with the user
		slackCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		response, _, _, err := client.OpenConversationContext(
			slackCtx,
			&slack.OpenConversationParameters{
				ReturnIM: false,
				Users: []string{
					memberID,
				},
			})

		if err != nil {
			message := "failed to open Slack DM with user"
			logger.Error(message, "error", err)
			return errors.Wrap(err, message)
		}

		// Send the Slack direct message to the user
		slackCtx, cancel = context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		if _, _, err = client.PostMessageContext(
			slackCtx,
			response.ID,
			slack.MsgOptionBlocks(view.Blocks.BlockSet...),
		); err != nil {
			message := "failed to send Slack direct message to user"
			logger.Error(message, "error", err)
			return errors.Wrap(err, message)
		}
	}

	logger.Debug("marked Slack user(s) as inactive in the database", "count", marked)

	return nil
}

// QueueMarkInactiveJob adds a new MARK_INACTIVE job to the queue.
func QueueMarkInactiveJob(ctx context.Context, db *gorm.DB, p *MarkInactiveParams, timestamp time.Time) error {
	job := models.GenericJob[*MarkInactiveParams]{
		JobType:  models.JobTypeMarkInactive,
		Priority: models.JobPriorityHigh,
		Params:   p,
		ExecAt:   timestamp,
	}

	return QueueJob(ctx, db, job)
}
