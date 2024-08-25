package bot

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
)

type DeleteMemberSuite struct {
	suite.Suite
	ctx    context.Context
	mock   sqlmock.Sqlmock
	db     *gorm.DB
	logger hclog.Logger
	buffer *bytes.Buffer
}

func (s *DeleteMemberSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)
	s.db, s.mock = database.NewMockedGormDB()
}

func (s *DeleteMemberSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *DeleteMemberSuite) Test_DeleteMember() {
	r := require.New(s.T())

	resource, databaseURL, err := database.NewTestPostgresDB(false)
	r.NoError(err)
	defer resource.Close()

	if err := database.Migrate(databaseURL); err != nil {
		r.NoError(err)
	}

	db, err := database.NewGormDB(databaseURL)
	if err != nil {
		r.NoError(err)
	}

	channelID := "C0123456789"

	// Write channel to the database
	db.Create(&models.Channel{
		ChannelID:      channelID,
		Inviter:        "U9876543210",
		ConnectionMode: models.VirtualConnectionMode,
		Interval:       models.Biweekly,
		Weekday:        time.Friday,
		Hour:           12,
		NextRound:      time.Now().Add(24 * time.Hour),
	})

	// Write two members to the database, verify only first member gets deleted
	isActive := false

	firstUserID := "U0123456789"
	db.Create(&models.Member{
		ChannelID:           channelID,
		UserID:              firstUserID,
		Gender:              models.Female,
		IsActive:            &isActive,
		HasGenderPreference: new(bool),
	})

	secondUserID := "U5555666778"
	db.Create(&models.Member{
		ChannelID:           channelID,
		UserID:              secondUserID,
		Gender:              models.Male,
		IsActive:            &isActive,
		HasGenderPreference: new(bool),
	})

	err = DeleteMember(s.ctx, db, nil, &DeleteMemberParams{
		ChannelID: channelID,
		UserID:    firstUserID,
	})
	r.NoError(err)

	var q models.Member
	db.First(&q)

	r.Equal(q.UserID, secondUserID)
}

func (s *DeleteMemberSuite) Test_QueueDeleteMemberJob() {
	r := require.New(s.T())

	p := &DeleteMemberParams{
		ChannelID: "C0123456789",
		UserID:    "U1111111111",
	}

	database.MockQueueJob(
		s.mock,
		p,
		models.JobTypeDeleteMember.String(),
		models.JobPriorityHigh,
	)

	err := QueueDeleteMemberJob(s.ctx, s.db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")
}

func Test_DeleteMember_suite(t *testing.T) {
	suite.Run(t, new(DeleteMemberSuite))
}
