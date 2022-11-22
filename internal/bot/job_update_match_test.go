package bot

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
)

func Test_UpdateMatch(t *testing.T) {
	r := require.New(t)

	logger, out := o11y.NewBufferedLogger()
	ctx := hclog.WithContext(context.Background(), logger)

	db, mock := database.NewMockedGormDB()

	matchID := 99
	hasMet := true

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE \"matches\" SET").
		WithArgs(
			hasMet,
			database.AnyTime(),
			matchID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	p := &UpdateMatchParams{
		MatchID: int32(matchID),
		HasMet:  hasMet,
	}

	err := UpdateMatch(ctx, db, nil, p)
	r.NoError(err)
	r.Contains(out.String(), "[INFO]")
	r.Contains(out.String(), "updated met status for the match")
}
