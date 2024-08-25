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

type CreatePairSuite struct {
	suite.Suite
	ctx    context.Context
	mock   sqlmock.Sqlmock
	db     *gorm.DB
	logger hclog.Logger
	buffer *bytes.Buffer
}

func (s *CreatePairSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)
	s.db, s.mock = database.NewMockedGormDB()
}

func (s *CreatePairSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *CreatePairSuite) Test_CreatePair() {
	r := require.New(s.T())

	resource, databaseURL, err := database.NewTestPostgresDB(true)
	r.NoError(err)
	defer resource.Close()

	db, err := database.NewGormDB(databaseURL)
	if err != nil {
		r.NoError(err)
	}

	channelID := "C0123456789"

	// Write channel to the database
	db.Create(&models.Channel{
		ChannelID:      channelID,
		Inviter:        "U9876543210",
		ConnectionMode: models.HybridConnectionMode,
		Interval:       models.Biweekly,
		Weekday:        time.Friday,
		Hour:           12,
		NextRound:      time.Now().Add(24 * time.Hour),
	})

	// Write two members to the database
	isActive := true
	hasGenderPreference := true

	firstUserID := "U0123456789"
	db.Create(&models.Member{
		ChannelID:           channelID,
		UserID:              firstUserID,
		Gender:              models.Male,
		IsActive:            &isActive,
		HasGenderPreference: &hasGenderPreference,
	})

	secondUserID := "U5555666778"
	db.Create(&models.Member{
		ChannelID:           channelID,
		UserID:              secondUserID,
		Gender:              models.Male,
		IsActive:            &isActive,
		HasGenderPreference: &hasGenderPreference,
	})

	// Write a record in the rounds and matches table
	db.Create(&models.Round{
		ChannelID: channelID,
	})

	db.Create(&models.Match{
		RoundID: 1,
	})

	// Test
	err = CreatePair(s.ctx, db, nil, &CreatePairParams{
		ChannelID:   channelID,
		MatchID:     1,
		Participant: firstUserID,
		Partner:     secondUserID,
	})
	r.NoError(err)
	r.Contains(s.buffer.String(), "created chat-roulette pair")

	var count int64
	result := db.Model(&models.Job{}).Where("job_type = ?", models.JobTypeNotifyPair).Where("data->>'match_id' = '1'").Count(&count)
	r.NoError(result.Error)
	r.Equal(int64(1), count)

	result = db.Model(&models.Pairing{}).Where("match_id = 1").Count(&count)
	r.NoError(result.Error)
	r.Equal(int64(2), count)
}

func (s *CreatePairSuite) Test_QueueCreatePairJob() {
	r := require.New(s.T())

	p := &CreatePairParams{
		ChannelID:   "C0123456789",
		MatchID:     1,
		Participant: "U0111111111",
		Partner:     "U2022222222",
	}

	database.MockQueueJob(
		s.mock,
		p,
		models.JobTypeCreatePair.String(),
		models.JobPriorityStandard,
	)

	err := QueueCreatePairJob(s.ctx, s.db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")
}

func Test_CreatePair_suite(t *testing.T) {
	suite.Run(t, new(CreatePairSuite))
}
