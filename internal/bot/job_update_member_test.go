package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
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

	p := &UpdateMemberParams{
		ChannelID: channelID,
		UserID:    userID,
		IsActive:  true,
	}

	err := UpdateMember(ctx, db, nil, p)
	r.NoError(err)
	r.NoError(mock.ExpectationsWereMet())
	r.Contains(out.String(), "[INFO]")
	r.Contains(out.String(), "updated database row for the member")
	r.Contains(out.String(), fmt.Sprintf("slack_user_id=%s", userID))
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

	p := &UpdateMemberParams{
		ChannelID: channelID,
		UserID:    userID,
		IsActive:  true,
	}

	data, _ := json.Marshal(p)
	job := models.NewJob(models.JobTypeUpdateMember, data)

	err := ExecJob(ctx, db, nil, job, UpdateMember)
	r.NoError(err)
	r.NoError(mock.ExpectationsWereMet())
	r.Contains(out.String(), "updated database row for the member")
}
