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
	"github.com/chat-roulettte/chat-roulette/internal/timex"
	"github.com/chat-roulettte/chat-roulette/internal/tzx"
)

const (
	notifyPairTemplateFilename = "notify_pair.json.tmpl"
)

// notifyPairTemplate is used with templates/notify_pair.json.tmpl
type notifyPairTemplate struct {
	ChannelID           string
	Interval            string
	Participant         models.Member
	ParticipantTimezone string
	Partner             models.Member
	PartnerTimezone     string
	// Suggestion          string
	ConnectionMode string
}

// NotifyPairParams are the parameters for the NOTIFY_PAIR job.
type NotifyPairParams struct {
	ChannelID   string `json:"channel_id"`
	MatchID     int32  `json:"match_id"`
	Participant string `json:"participant"`
	Partner     string `json:"partner"`
}

// NotifyPair notifies a pair of chat-roulette participants that
// they have been matched for this round of chat-roulette.
func NotifyPair(ctx context.Context, db *gorm.DB, client *slack.Client, p *NotifyPairParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
		"match_id", p.MatchID,
	)

	logger.Info("notifying pair")

	// Retrieve channel metadata from the database
	var channel models.Channel

	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	if err := db.WithContext(dbCtx).Where("channel_id = ?", p.ChannelID).First(&channel).Error; err != nil {
		message := "failed to retrieve metadata for the Slack channel"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	// Create a Slack Group DM with the pair of participants and update the database
	childCtx, cancel := context.WithTimeout(ctx, 3000*time.Millisecond)
	defer cancel()

	response, _, _, err := client.OpenConversationContext(childCtx,
		&slack.OpenConversationParameters{
			ReturnIM: false,
			Users: []string{
				p.Participant,
				p.Partner,
			},
		})

	if err != nil {
		message := "failed to create Slack Group DM"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	mpimID := response.Conversation.ID

	dbCtx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	result := db.WithContext(childCtx).
		Model(&models.Match{}).
		Where("id = ?", p.MatchID).
		Update("mpim_id", mpimID)

	if result.Error != nil {
		message := "failed to update mpim_id for the match"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	logger.Info("updated mpim_id for the match")

	// Retrieve member info for template
	var participants []models.Member

	result = db.WithContext(dbCtx).
		Model(&models.Member{}).
		Where("channel_id = ?", p.ChannelID).
		Where("user_id IN ?", []string{p.Participant, p.Partner}).
		Scan(&participants)

	if result.Error != nil {
		message := "failed to retrieve members' info"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	participant := participants[0]
	partner := participants[1]

	// Template the message to send to the pair
	templateParams := notifyPairTemplate{
		ChannelID:           p.ChannelID,
		Interval:            channel.Interval.String(),
		Participant:         participant,
		ParticipantTimezone: tzx.GetAbbreviatedTimezone(participant.Timezone.String()),
		Partner:             partner,
		PartnerTimezone:     tzx.GetAbbreviatedTimezone(partner.Timezone.String()),
		ConnectionMode:      channel.ConnectionMode.String(),
	}

	content, err := renderTemplate(notifyPairTemplateFilename, templateParams)
	if err != nil {
		message := "failed to render template"
		logger.Error(message, "error", err, "template", notifyPairTemplateFilename)
		return errors.Wrap(err, message)
	}

	// We can marshal the template into View as it contains Blocks
	var view slack.View
	if err := json.Unmarshal([]byte(content), &view); err != nil {
		message := "failed to marshal JSON"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	// Send the Slack group message to the pair
	slackCtx, cancel := context.WithTimeout(ctx, 3000*time.Millisecond)
	defer cancel()

	if _, _, err = client.PostMessageContext(
		slackCtx,
		response.Conversation.ID,
		slack.MsgOptionBlocks(view.Blocks.BlockSet...),
		slack.MsgOptionDisableLinkUnfurl(),
		slack.MsgOptionDisableMediaUnfurl(),
	); err != nil {
		message := "failed to send Slack group message"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	// Update the was_notified column for the record in the matches table
	dbCtx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	result = db.WithContext(dbCtx).
		Model(&models.Match{}).
		Where("id = ?", p.MatchID).
		Update("was_notified", true)

	if result.Error != nil {
		message := "failed to update was_notified for the match"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	logger.Info("updated was_notified for the match")

	// Queue a KICKOFF_PAIR job for the middle of the round
	kickoffPairParams := &KickoffPairParams{
		ChannelID:   p.ChannelID,
		MatchID:     p.MatchID,
		Participant: p.Participant,
		Partner:     p.Partner,
	}

	dbCtx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	if err := QueueKickoffPairJob(dbCtx, db, kickoffPairParams); err != nil {
		message := "failed to add KICKOFF_PAIR job to the queue"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	logger.Info("queued KICKOFF_PAIR job for this match to run after 24 hours")

	// Queue a CHECK_PAIR job for the middle of the round
	params := &CheckPairParams{
		ChannelID:   p.ChannelID,
		MatchID:     p.MatchID,
		Participant: p.Participant,
		Partner:     p.Partner,
		NextRound:   channel.NextRound,
		MpimID:      mpimID,
	}

	midpoint, err := timex.MidPoint(time.Now().UTC(), channel.NextRound)
	if err != nil {
		message := "failed to add CHECK_PAIR job to the queue"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	params.IsMidRound = true

	dbCtx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	if err := QueueCheckPairJob(dbCtx, db, params, midpoint); err != nil {
		message := "failed to add CHECK_PAIR job to the queue"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	logger.Info("queued CHECK_PAIR job for this match to run in the middle of the round")

	// Queue a CHECK_PAIR job for the end of the round
	params.IsMidRound = false

	dbCtx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	timestamp := channel.NextRound.Add(-(12 * time.Hour)) // 12 hours before the round ends

	if err := QueueCheckPairJob(dbCtx, db, params, timestamp); err != nil {
		message := "failed to add CHECK_PAIR job to the queue"
		logger.Error(message, "error", err)
		return errors.Wrap(err, message)
	}

	logger.Info("queued CHECK_PAIR job for this match to run at the end of the round")

	return nil
}

// QueueNotifyPairJob adds a new NOTIFY_PAIR job to the queue.
func QueueNotifyPairJob(ctx context.Context, db *gorm.DB, p *NotifyPairParams) error {
	job := models.GenericJob[*NotifyPairParams]{
		JobType:  models.JobTypeNotifyPair,
		Priority: models.JobPriorityStandard,
		Params:   p,
	}

	return QueueJob(ctx, db, job)
}
