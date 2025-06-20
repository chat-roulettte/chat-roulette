package bot

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
)

const (
	reportMatchesTemplateFilename = "report_matches.json.tmpl"
)

// reportMatchesTemplate is used with reportMatchesTemplateFilename
type reportMatchesTemplate struct {
	IsAdmin             bool
	UserID              string
	ChannelID           string
	NextRound           time.Time
	Participants        int
	Men                 int
	Women               int
	HasGenderPreference int
	Pairs               int
	Unpaired            int
}

// ReportMatchesParams are the parameters for the REPORT_MATCHES job.
type ReportMatchesParams struct {
	ChannelID    string `json:"channel_id"`
	RoundID      int32  `json:"round_id"`
	Participants int    `json:"participants"`
	Pairs        int    `json:"pairs"`
	Unpaired     int    `json:"unpaired"`
}

type matchStats struct {
	Men                 int
	Women               int
	HasGenderPreference int
}

// ReportMatches sends a report of matches for the current round to the chat-roulette admin and channel.
func ReportMatches(ctx context.Context, db *gorm.DB, client *slack.Client, p *ReportMatchesParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
		attributes.RoundID, p.RoundID,
	)

	// Lookup the channel
	var channel models.Channel

	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	result := db.WithContext(dbCtx).
		Model(&models.Channel{}).
		Where("channel_id = ?", p.ChannelID).
		First(&channel)

	if result.Error != nil {
		message := "failed to lookup channel in the database"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	// Lookup how many men, women, has_gender_preference in this round
	var stats matchStats

	dbCtx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	result = db.WithContext(dbCtx).
		Table("pairings").
		Select(`
		COUNT(*) FILTER (WHERE members.gender = 'male') AS men,
		COUNT(*) FILTER (WHERE members.gender = 'female') AS women,
		COUNT(*) FILTER (WHERE members.has_gender_preference) AS has_gender_preference`).
		Joins("JOIN matches ON matches.id = pairings.match_id").
		Joins("JOIN members ON pairings.member_id = members.id").
		Where("matches.round_id = ?", p.RoundID).
		Scan(&stats)

	if result.Error != nil {
		message := "failed to lookup match stats in the database"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	// Send message to channel and admin concurrently
	t := reportMatchesTemplate{
		UserID:              channel.Inviter,
		ChannelID:           p.ChannelID,
		NextRound:           channel.NextRound,
		Participants:        p.Participants,
		Pairs:               p.Pairs,
		Unpaired:            p.Unpaired,
		Men:                 stats.Men,
		Women:               stats.Women,
		HasGenderPreference: stats.HasGenderPreference,
	}

	var multiErr error

	wg := new(sync.WaitGroup)

	wg.Add(1)
	go func() {
		defer wg.Done()

		// Render template
		template := t
		template.IsAdmin = true

		content, err := renderTemplate(reportMatchesTemplateFilename, template)
		if err != nil {
			logger.Error("failed to render template", "error", err, "template", reportMatchesTemplateFilename)
			multiErr = errors.Wrap(err, "failed to render template")
		}

		// We can marshal the template into View as it contains Blocks
		var view slack.View
		if err := json.Unmarshal([]byte(content), &view); err != nil {
			logger.Error("failed to unmarshal JSON", "error", err)
			multiErr = errors.Wrap(err, "failed to unmarshal JSON")
		}

		// Open a Slack DM with the chat-roulette admin
		slackCtx, cancel := context.WithTimeout(ctx, 3000*time.Millisecond)
		defer cancel()

		response, _, _, err := client.OpenConversationContext(
			slackCtx,
			&slack.OpenConversationParameters{
				ReturnIM: false,
				Users: []string{
					channel.Inviter,
				},
			})

		if err != nil {
			logger.Error("failed to open Slack DM to channel admin", "error", err)
			multiErr = err
		}

		slackCtx, cancel = context.WithTimeout(ctx, 3000*time.Millisecond)
		defer cancel()

		// Send a private message to the admin
		if _, _, err = client.PostMessageContext(
			slackCtx,
			response.ID,
			slack.MsgOptionBlocks(view.Blocks.BlockSet...),
		); err != nil {
			logger.Error("failed to report match stats to chat-roulette admin", "error", err)
			multiErr = err
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		// Render template
		content, err := renderTemplate(reportMatchesTemplateFilename, t)
		if err != nil {
			logger.Error("failed to render template", "error", err, "template", reportMatchesTemplateFilename)
			multiErr = errors.Wrap(err, "failed to render template")
		}

		// We can marshal the template into View as it contains Blocks
		var view slack.View
		if err := json.Unmarshal([]byte(content), &view); err != nil {
			logger.Error("failed to unmarshal JSON", "error", err)
			multiErr = errors.Wrap(err, "failed to unmarshal JSON")
		}

		// Send a  message to the channel admin
		slackCtx, cancel := context.WithTimeout(ctx, 3000*time.Millisecond)
		defer cancel()

		if _, _, err = client.PostMessageContext(
			slackCtx,
			p.ChannelID,
			slack.MsgOptionBlocks(view.Blocks.BlockSet...),
		); err != nil {
			logger.Error("failed to report match stats to chat-roulette channel", "error", err)
			multiErr = err
		}
	}()

	wg.Wait()

	if multiErr != nil {
		return multiErr
	}

	return nil
}

// QueueReportMatchesJob adds a new REPORT_MATCHES job to the queue.
func QueueReportMatchesJob(ctx context.Context, db *gorm.DB, p *ReportMatchesParams) error {
	job := models.GenericJob[*ReportMatchesParams]{
		JobType:  models.JobTypeReportMatches,
		Priority: models.JobPriorityLowest,
		Params:   p,
		ExecAt:   time.Now().UTC().Add(5 * time.Minute),
	}

	return QueueJob(ctx, db, job)
}
