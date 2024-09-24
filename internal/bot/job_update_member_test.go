package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bincyber/go-sqlcrypter"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
)

func Test_UpdateMember(t *testing.T) {
	r := require.New(t)

	logger, out := o11y.NewBufferedLogger()
	ctx := hclog.WithContext(context.Background(), logger)

	resource, databaseURL, err := database.NewTestPostgresDB(false)
	r.NoError(err)
	defer resource.Close()

	r.NoError(database.Migrate(databaseURL))

	db, err := database.NewGormDB(databaseURL)
	r.NoError(err)

	sqlcrypter.Init(database.NoOpCrypter{})

	channelID := "C9876543210"
	userID := "U0123456789"

	// Write channel to the database
	db.Create(&models.Channel{
		ChannelID:      channelID,
		Inviter:        "U9876543210",
		ConnectionMode: models.VirtualConnectionMode,
		Interval:       models.Biweekly,
		Weekday:        time.Friday,
		Hour:           12,
		NextRound:      time.Now().Add(72 * time.Hour),
	})

	// Write member to the database
	isActive := true
	hasGenderPreference := true

	db.Create(&models.Member{
		ChannelID:           channelID,
		UserID:              userID,
		Gender:              models.Male,
		Country:             sqlcrypter.NewEncryptedBytes("United States of America"),
		City:                sqlcrypter.NewEncryptedBytes("New York"),
		IsActive:            &isActive,
		HasGenderPreference: &hasGenderPreference,
	})

	// Update member
	isActive = false
	hasGenderPreference = false

	p := &UpdateMemberParams{
		ChannelID:           channelID,
		UserID:              userID,
		Gender:              models.Male.String(),
		City:                sqlcrypter.NewEncryptedBytes("Los Angeles"),
		IsActive:            &isActive,
		HasGenderPreference: &hasGenderPreference,
	}

	err = UpdateMember(ctx, db, nil, p)
	r.NoError(err)
	r.Contains(out.String(), "[INFO]")
	r.Contains(out.String(), "updated database row for the member")
	r.Contains(out.String(), fmt.Sprintf("slack_user_id=%s", userID))

	// Verify changes
	var member *models.Member
	result := db.Model(&models.Member{}).Where("user_id = ?", userID).Where("channel_id = ?", channelID).First(&member)
	r.NoError(result.Error)
	r.Equal(result.RowsAffected, int64(1))
	r.False(*member.IsActive)
	r.False(*member.HasGenderPreference)
	r.Equal(member.City.String(), "Los Angeles")
}

func Test_ExecUpdateMember(t *testing.T) {
	r := require.New(t)

	logger, out := o11y.NewBufferedLogger()
	ctx := hclog.WithContext(context.Background(), logger)

	db, mock := database.NewMockedGormDB()

	channelID := "C9876543210"
	userID := "U0123456789"

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "members" SET .* WHERE channel_id = (.+) AND user_id = (.+)`).
		WithArgs(
			userID,
			channelID,
			true,
			database.AnyTime(),
			channelID,
			userID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	isActive := true

	p := &UpdateMemberParams{
		ChannelID: channelID,
		UserID:    userID,
		IsActive:  &isActive,
	}

	data, _ := json.Marshal(p)
	job := models.NewJob(models.JobTypeUpdateMember, data)

	err := ExecJob(ctx, db, nil, job, UpdateMember)
	r.NoError(err)
	r.NoError(mock.ExpectationsWereMet())
	r.Contains(out.String(), "updated database row for the member")
}
