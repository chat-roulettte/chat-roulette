package bot

import (
	"context"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
)

type WaitOnMemberJobsSuite struct {
	suite.Suite
	channelID string
	mock      sqlmock.Sqlmock
	db        *gorm.DB
}

func (s *WaitOnMemberJobsSuite) SetupTest() {
	s.db, s.mock = database.NewMockedGormDB()

	s.channelID = "C0123456789"
}

func (s *WaitOnMemberJobsSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *WaitOnMemberJobsSuite) Test_ContextCancellation() {
	r := require.New(s.T())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitOnMemberJobs(ctx, s.db, s.channelID)
	r.NotNil(err)
	r.True(errors.Is(err, context.Canceled))
}

func (s *WaitOnMemberJobsSuite) Test_NoPendingJobs() {
	r := require.New(s.T())

	s.mock.ExpectQuery(`SELECT "job_type" FROM "jobs" WHERE data->>'channel_id' = (.+) AND is_completed = false AND job_type IN (.+)`).
		WithArgs(
			s.channelID,
			models.JobTypeAddMember.String(),
			models.JobTypeDeleteMember.String(),
			models.JobTypeUpdateMatch.String(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"job_type"})) // 0 rows

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := waitOnMemberJobs(ctx, s.db, s.channelID)
	r.NoError(err)
}

func (s *WaitOnMemberJobsSuite) Test_OnePendingJob() {
	r := require.New(s.T())

	// Mock to return 1 row
	s.mock.ExpectQuery(`SELECT "job_type" FROM "jobs" WHERE data->>'channel_id' = (.+) AND is_completed = false AND job_type IN (.+)`).
		WithArgs(
			s.channelID,
			models.JobTypeAddMember.String(),
			models.JobTypeDeleteMember.String(),
			models.JobTypeUpdateMatch.String(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"job_type"}).FromCSVString("ADD_MEMBER"))

	// Mock to return 0 rows
	s.mock.ExpectQuery(`SELECT "job_type" FROM "jobs"`).
		WithArgs(
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"job_type"})) // 0 rows

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := waitOnMemberJobs(ctx, s.db, s.channelID)
	r.NoError(err)
}

func (s *WaitOnMemberJobsSuite) Test_QueryError() {
	r := require.New(s.T())

	s.mock.ExpectQuery(`SELECT "job_type" FROM "jobs" WHERE data->>'channel_id' = (.+) AND is_completed = false AND job_type IN (.+)`).
		WithArgs(
			s.channelID,
			models.JobTypeAddMember.String(),
			models.JobTypeDeleteMember.String(),
			models.JobTypeUpdateMatch.String(),
		).
		WillReturnError(fmt.Errorf("dial error"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := waitOnMemberJobs(ctx, s.db, s.channelID)
	r.NotNil(err)
	r.Contains(err.Error(), "failed to query pending member jobs")
}

func Test_WaitOnMemberJobs_suite(t *testing.T) {
	suite.Run(t, new(WaitOnMemberJobsSuite))
}
