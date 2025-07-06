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
	Volunteer           string
	Participant         string
	Partner             string
	NoMessagesExchanged bool
	Icebreaker          string
}

// KickoffPairParams are the parameters for the KICKOFF_PAIR job.
type KickoffPairParams struct {
	ChannelID   string `json:"channel_id"`
	MatchID     int32  `json:"match_id"`
	Participant string `json:"participant"`
	Partner     string `json:"partner"`
}

// KickoffPair stimulates conversation for a pair of chat-roulette participants
// by sharing an icebreaker and picking a volunteer to answer first.
func KickoffPair(ctx context.Context, db *gorm.DB, client *slack.Client, p *KickoffPairParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
		attributes.MatchID, p.MatchID,
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
	var timestamp time.Time

	dbCtx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

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

	// Get a random icebreaker question from the database
	var icebreaker models.Icebreaker

	dbCtx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	if err := db.WithContext(dbCtx).Model(&models.Icebreaker{}).Order("RANDOM()").First(&icebreaker).Error; err != nil {
		message := "failed to retrieve random icebreaker from the DB"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	// Pick a volunteer and share the icebreaker
	t := kickoffPairTemplate{
		Volunteer:           selectRandomParticipant(p.Participant, p.Partner),
		Participant:         p.Participant,
		Partner:             p.Partner,
		NoMessagesExchanged: true,
		Icebreaker:          icebreaker.Question,
	}

	if len(history.Messages) > 1 {
		t.NoMessagesExchanged = false
	}

	content, err := renderTemplate(kickoffPairTemplateFilename, t)
	if err != nil {
		message := "failed to render template"
		logger.Error(message, "error", err, "template", kickoffPairTemplateFilename)
		return errors.Wrap(err, "failed to render template")
	}

	var msg slackMessage
	if err := json.Unmarshal([]byte(content), &msg); err != nil {
		message := "failed to unmarshal JSON template to slack.Message"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	slackCtx, cancel = context.WithTimeout(ctx, 3000*time.Millisecond)
	defer cancel()

	if _, _, err = client.PostMessageContext(
		slackCtx,
		match.MpimID,
		slack.MsgOptionBlocks(msg.Blocks.BlockSet...),
		slack.MsgOptionAttachments(msg.Attachments...),
		slack.MsgOptionDisableLinkUnfurl(),
		slack.MsgOptionDisableMediaUnfurl(),
	); err != nil {
		message := "failed to send Slack group DM"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	// Increment the counter for how many times this icebreaker has been used
	dbCtx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	err = db.WithContext(dbCtx).Model(&models.Icebreaker{}).Where("id = ?", icebreaker.ID).
		UpdateColumn("usage_count", gorm.Expr("usage_count + ?", 1)).Error
	if err != nil {
		message := "failed to increment random icebreaker from the DB"
		logger.Error(message, "error", err)
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
