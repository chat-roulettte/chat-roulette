package bot

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	rand "math/rand/v2"

	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
)

const (
	kickoffPairTemplateFilename = "kickoff_pair.json.tmpl"
)

// kickoffPairTemplate is used with templates/kickoff_pair.json.tmpl
type kickoffPairTemplate struct {
	Volunteer   string
	Participant string `json:"participant"`
	Partner     string `json:"partner"`
}

// KickoffPairParams are the parameters for the KICKOFF_PAIR job.
type KickoffPairParams struct {
	ChannelID   string `json:"channel_id"`
	MatchID     int32  `json:"match_id"`
	Participant string `json:"participant"`
	Partner     string `json:"partner"`
}

// KickoffPair notifies a pair of chat-roulette participants that
// they have been matched for this round of chat-roulette.
func KickoffPair(ctx context.Context, db *gorm.DB, client *slack.Client, p *KickoffPairParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
		"match_id", p.MatchID,
	)

	// Retrieve match metadata from the database
	var match models.Match

	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	if err := db.WithContext(dbCtx).Model(&models.Match{}).Where("id = ?", p.MatchID).First(&match).Error; err != nil {
		message := "failed to retrieve metadata for the match"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	// Retrieve the timestamp of NOTIFY_PAIR job to determine when the bot sent message to pair
	dbCtx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	var timestamp time.Time

	result := db.WithContext(dbCtx).
		Model(&models.Job{}).
		Select("exec_at").
		Where("status = ?", models.JobStatusSucceeded).
		Where("is_completed = true").
		Where("job_type = ?", models.JobTypeNotifyPair.String()).
		Where(datatypes.JSONQuery("data").Equals(p.MatchID, "match_id")).
		Take(&timestamp)

	if result.Error != nil {
		message := "failed to retrieve timestamp of last NOTIFY_PAIR job"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	// Check if the pair have already started a discussion
	slackCtx, cancel := context.WithTimeout(ctx, 3000*time.Millisecond)
	defer cancel()

	history, err := client.GetConversationHistoryContext(slackCtx, &slack.GetConversationHistoryParameters{
		ChannelID: match.MpimID,
		Oldest:    strconv.FormatInt(timestamp.Unix(), 10),
		Limit:     10,
		Inclusive: true,
	})

	if err != nil {
		message := "failed to retrieve chat history from the Slack group DM"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	logger.Debug("retrieved chat history from the Slack Group DM", "messages", len(history.Messages))

	if len(history.Messages) > 1 {
		logger.Debug("skipping sending message to the pair to kickstart a conversation")
		return nil
	}

	// Pick a volunteer and kick start the conversation
	t := kickoffPairTemplate{
		Volunteer:   selectRandomParticipant(p.Participant, p.Partner),
		Participant: p.Participant,
		Partner:     p.Partner,
	}

	content, err := renderTemplate(kickoffPairTemplateFilename, t)
	if err != nil {
		message := "failed to render template"
		logger.Error(message, "error", err, "template", kickoffPairTemplateFilename)
		return errors.Wrap(err, "failed to render template")
	}

	var view slack.View
	if err := json.Unmarshal([]byte(content), &view); err != nil {
		message := "failed to unmarshal JSON template to slack.View"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	slackCtx, cancel = context.WithTimeout(ctx, 3000*time.Millisecond)
	defer cancel()

	if _, _, err = client.PostMessageContext(
		slackCtx,
		match.MpimID,
		slack.MsgOptionBlocks(view.Blocks.BlockSet...),
		slack.MsgOptionDisableLinkUnfurl(),
		slack.MsgOptionDisableMediaUnfurl(),
	); err != nil {
		message := "failed to send Slack group DM"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	return nil
}

// QueueKickoffPairJob adds a new KICKOFF_PAIR job to the queue.
func QueueKickoffPairJob(ctx context.Context, db *gorm.DB, p *KickoffPairParams) error {
	job := models.GenericJob[*KickoffPairParams]{
		JobType:  models.JobTypeKickoffPair,
		Priority: models.JobPriorityLow,
		Params:   p,
		ExecAt:   time.Now().UTC().Add(24 * time.Hour),
	}

	return QueueJob(ctx, db, job)
}

func selectRandomParticipant(p1, p2 string) string {
	pair := []string{p1, p2}
	randomIndex := rand.IntN(len(pair)) //nolint:gosec
	return pair[randomIndex]
}
