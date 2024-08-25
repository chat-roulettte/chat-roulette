package bot

import (
	"context"
	"time"

	"github.com/bincyber/go-sqlcrypter"
	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y/attributes"
)

// UpdateMemberParams are the parameters for the UPDATE_MEMBER job.
type UpdateMemberParams struct {
	ChannelID           string                    `json:"channel_id"`
	UserID              string                    `json:"user_id"`
	Gender              string                    `json:"gender,omitempty"`
	Country             sqlcrypter.EncryptedBytes `json:"country,omitempty"`
	City                sqlcrypter.EncryptedBytes `json:"city,omitempty"`
	Timezone            sqlcrypter.EncryptedBytes `json:"timezone,omitempty"`
	ProfileType         sqlcrypter.EncryptedBytes `json:"profile_type,omitempty"`
	ProfileLink         sqlcrypter.EncryptedBytes `json:"profile_link,omitempty"`
	CalendlyLink        sqlcrypter.EncryptedBytes `json:"calendly_link,omitempty"`
	IsActive            bool                      `json:"is_active"`
	HasGenderPreference bool                      `json:"has_gender_preference"`
}

// UpdateMember updates the participation status for a member of a Slack channel.
func UpdateMember(ctx context.Context, db *gorm.DB, client *slack.Client, p *UpdateMemberParams) error {

	logger := hclog.FromContext(ctx).With(
		attributes.SlackChannelID, p.ChannelID,
		attributes.SlackUserID, p.UserID,
	)

	logger.Info("updating member")

	// Update the row for the member in the database
	member := models.Member{
		UserID:              p.UserID,
		ChannelID:           p.ChannelID,
		Country:             p.Country,
		City:                p.City,
		Timezone:            p.Timezone,
		ProfileType:         p.ProfileType,
		ProfileLink:         p.ProfileLink,
		CalendlyLink:        p.CalendlyLink,
		HasGenderPreference: &p.HasGenderPreference,
		IsActive:            &p.IsActive,
	}

	if p.Gender != "" {
		v, err := models.ParseGender(p.Gender)
		if err != nil {
			logger.Error("failed to parse gender", "error", err)
			return err
		}
		member.Gender = v
	}

	dbCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	result := db.WithContext(dbCtx).
		Model(&models.Member{}).
		Where("channel_id = ?", p.ChannelID).
		Where("user_id = ?", p.UserID).
		Updates(member)

	if result.Error != nil {
		message := "failed to update database row for the member"
		logger.Error(message, "error", result.Error)
		return errors.Wrap(result.Error, message)
	}

	logger.Info("updated database row for the member")

	return nil
}

// QueueUpdateMemberJob adds a new UPDATE_MEMBER job to the queue.
func QueueUpdateMemberJob(ctx context.Context, db *gorm.DB, p *UpdateMemberParams) error {
	job := models.GenericJob[*UpdateMemberParams]{
		JobType:  models.JobTypeUpdateMember,
		Priority: models.JobPriorityHigh,
		Params:   p,
	}

	return QueueJob(ctx, db, job)
}
