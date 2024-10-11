package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
)

const (
	checkPairTemplateFilename         = "check_pair.json.tmpl"
	checkPairResponseTemplateFilename = "check_pair_response.json.tmpl"
)

// checkPairTemplate is used with templates/check_pair.json.tmpl and templates/check_pair_response.json.tmpl
type checkPairTemplate struct {
	Participant string `json:"participant"`
	Partner     string `json:"partner"`
	MatchID     int32  `json:"match_id"`
	Responder   string `json:"responder"`
	HasMet      bool   `json:"has_met"`
	IsMidRound  bool   `json:"is_mid_round"`
}

// CheckPairParams are the parameters for the CHECK_PAIR job.
type CheckPairParams struct {
	ChannelID   string    `json:"channel_id"`
	NextRound   time.Time `json:"next_round"`
	MatchID     int32     `json:"match_id"`
	Participant string    `json:"participant"`
	Partner     string    `json:"partner"`
	MpimID      string    `json:"mpim_id"`
	IsMidRound  bool      `json:"is_mid_round"`
}

// CheckPair sends a private group message to a chat-roulette pair
// to check if they have had a chance to meet during this round of chat-roulette.
func CheckPair(ctx context.Context, db *gorm.DB, client *slack.Client, p *CheckPairParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
		"match_id", p.MatchID,
	)

	logger.Info("checking if pair has met for chat-roulette")

	// Template the message to send to the pair
	templateParams := checkPairTemplate{
		Participant: p.Participant,
		Partner:     p.Partner,
		MatchID:     p.MatchID,
		IsMidRound:  p.IsMidRound,
	}

	content, err := renderTemplate(checkPairTemplateFilename, templateParams)
	if err != nil {
		message := "failed to render template"
		logger.Error(message, "error", err, "template", checkPairTemplateFilename)
		return errors.Wrap(err, message)
	}

	// We can marshal the json template into View as it contains Blocks
	var view slack.View
	if err := json.Unmarshal([]byte(content), &view); err != nil {
		return errors.Wrap(err, "failed to unmarshal JSON")
	}

	// Send an interactive Slack group message to the pair
	//
	// The interaction will be received by our /interactions endpoint
	// and handled outside of this job.
	slackCtx, cancel := context.WithTimeout(ctx, 3000*time.Millisecond)
	defer cancel()

	if _, _, err = client.PostMessageContext(
		slackCtx,
		p.MpimID,
		slack.MsgOptionBlocks(view.Blocks.BlockSet...),
	); err != nil {
		logger.Error("failed to send Slack group message", "error", err)
		return err
	}

	return nil
}

// QueueCheckPairJob adds a new CHECK_PAIR job to the queue.
func QueueCheckPairJob(ctx context.Context, db *gorm.DB, p *CheckPairParams, timestamp time.Time) error {
	job := models.GenericJob[*CheckPairParams]{
		JobType:  models.JobTypeCheckPair,
		Priority: models.JobPriorityStandard,
		Params:   p,
		ExecAt:   timestamp,
	}

	return QueueJob(ctx, db, job)
}

type checkPairButtonValue struct {
	Participant string `json:"participant"`
	Partner     string `json:"partner"`
	MatchID     int32  `json:"match_id"`
	HasMet      bool   `json:"has_met"`
	IsMidRound  bool   `json:"is_mid_round"`
}

func (v *checkPairButtonValue) Encode() string {
	s, err := json.Marshal(&v)
	if err != nil {
		panic(err)
	}

	return string(s)
}

func (v *checkPairButtonValue) Decode(s string) {
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		panic(err)
	}
}

// HandleCheckPairButtons processes the webhook sent by Slack when a user clicks on
// the button in the message sent by the CHECK_PAIR job confirming if they have had a chance
// to meet the participant that they were paired with in this round of chat roulette.
// A response is sent overwriting the button in the original message, so that it cannot
// be clicked multiple times. This interaction contains multiple buttons, so we do need
// to parse the action. An UPDATE_MATCH job is then queued to modify the "has_met" column
// for the match in the database.
func HandleCheckPairButtons(ctx context.Context, client *http.Client, db *gorm.DB, interaction *slack.InteractionCallback) error {
	if len(interaction.Message.Blocks.BlockSet) > 0 && len(interaction.ActionCallback.BlockActions) > 0 {
		var value checkPairButtonValue
		value.Decode(interaction.ActionCallback.BlockActions[0].Value)

		// Template the confirmation message
		t := checkPairTemplate{
			Responder:   interaction.User.ID,
			HasMet:      value.HasMet,
			Participant: value.Participant,
			Partner:     value.Partner,
			IsMidRound:  value.IsMidRound,
		}

		content, err := renderTemplate(checkPairResponseTemplateFilename, t)
		if err != nil {
			return errors.Wrap(err, "failed to render template")
		}

		// We can marshal the json template into View as it contains Blocks
		var view slack.View
		if err := json.Unmarshal([]byte(content), &view); err != nil {
			return errors.Wrap(err, "failed to unmarshal JSON")
		}

		webhookMessage := &slack.WebhookMessage{
			Blocks:          &view.Blocks,
			ReplaceOriginal: true,
		}

		if t.HasMet && value.IsMidRound {
			// Cancel the end of round check-in since the pair already met
			dbCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			defer cancel()

			result := db.WithContext(dbCtx).
				Model(&models.Job{}).
				Where(datatypes.JSONQuery("data").Equals(value.MatchID, "match_id")).
				Where("status = ?", models.JobStatusPending).
				Where("is_completed = false").
				Where("job_type = ?", models.JobTypeCheckPair.String()).
				Updates(&models.Job{IsCompleted: true, Status: models.JobStatusCanceled})

			if result.Error != nil {
				return errors.Wrap(result.Error, "failed to cancel pending CHECK_PAIR job")
			}
		}

		// Queue an UPDATE_MATCH job to update the "has_met" column for the match
		if db != nil {
			params := &UpdateMatchParams{
				MatchID: value.MatchID,
				HasMet:  value.HasMet,
			}

			dbCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			defer cancel()

			if err := QueueUpdateMatchJob(dbCtx, db, params); err != nil {
				return errors.Wrap(err, "failed to add UPDATE_MATCH job to the queue")
			}
		}

		// Send HTTP response for the webhook
		if err := slack.PostWebhookCustomHTTPContext(ctx, interaction.ResponseURL, client, webhookMessage); err != nil {
			return errors.Wrap(err, "failed to send Slack webhook")
		}
	}

	return nil
}
