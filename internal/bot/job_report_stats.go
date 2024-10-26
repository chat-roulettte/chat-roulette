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
	Pairs   float64
	Met     float64
	Percent float64
}

type roundStats struct {
	Total int
	Met   int
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

	// Retrieve the number of pairs that were made and how many actually met
	var stats roundStats

	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	result := db.WithContext(dbCtx).
		Model(&models.Match{}).
		Select(`
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE has_met) AS met
		`).
		Where("round_id = ?", p.RoundID).
		Find(&stats)

	if result.Error != nil {
		message := "failed to retrieve match results"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	// Calculate matches percent
	percent := (float64(stats.Met) / float64(stats.Total)) * 100

	// Render template
	t := reportStatsTemplate{
		Pairs:   float64(stats.Total),
		Met:     float64(stats.Met),
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
		Priority: models.JobPriorityLow,
		Params:   p,
		ExecAt:   p.NextRound.Add(-(4 * time.Hour)), // 4 hours before the start of the next round
	}

	return QueueJob(ctx, db, job)
}
