package bot

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
)

const (
	reportStatsTemplateFilename = "report_stats.json.tmpl"
)

// reportStatsTemplate is used with reportStatsTemplateFilename
type reportStatsTemplate struct {
	Matches float64
	Met     float64
	Percent float64
}

// ReportStatsParams are the parameters for the REPORT_STATS job.
type ReportStatsParams struct {
	ChannelID string    `json:"channel_id"`
	RoundID   int32     `json:"round_id"`
	NextRound time.Time `json:"next_round"`
}

// ReportStats messages a Slack channel with the stats for the last round of chat-roulette.
func ReportStats(ctx context.Context, db *gorm.DB, client *slack.Client, p *ReportStatsParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
		"round_id", p.RoundID,
	)

	// Retrieve the number of matches that were made and how many actually met
	type Matches struct {
		Total int64
		Met   int64
	}

	var matches Matches

	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	result := db.WithContext(dbCtx).
		Model(&models.Match{}).
		Select(
			`COUNT(*) as total`,
			`SUM(CASE WHEN has_met = true then 1 else 0 end) AS met`,
		).
		Where("round_id = ?", p.RoundID).
		Find(&matches)

	if result.Error != nil {
		message := "failed to retrieve match results"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	// Calculate matches percent
	percent := (float64(matches.Met) / float64(matches.Total)) * 100

	// Render template
	t := reportStatsTemplate{
		Matches: float64(matches.Total),
		Met:     float64(matches.Met),
		Percent: percent,
	}

	content, err := renderTemplate(reportStatsTemplateFilename, t)
	if err != nil {
		return errors.Wrap(err, "failed to render template")
	}

	// We can marshal the template into View as it contains Blocks
	var view slack.View
	if err := json.Unmarshal([]byte(content), &view); err != nil {
		return errors.Wrap(err, "failed to unmarshal JSON")
	}

	// Send the message to the Slack channel
	if _, _, err = client.PostMessageContext(
		ctx,
		p.ChannelID,
		slack.MsgOptionBlocks(view.Blocks.BlockSet...),
	); err != nil {
		logger.Error("failed to report stats to Slack channel", "error", err)
		return err
	}

	return nil
}

// QueueReportStatsJob adds a new REPORT_STATS job to the queue.
func QueueReportStatsJob(ctx context.Context, db *gorm.DB, p *ReportStatsParams) error {
	job := models.GenericJob[*ReportStatsParams]{
		JobType:  models.JobTypeReportStats,
		Priority: models.JobPriorityStandard,
		Params:   p,
		// This should execute before the start of the next chat-roulette round.
		ExecAt: p.NextRound.Add(-(1 * time.Hour)),
	}

	return QueueJob(ctx, db, job)
}
