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

type CreateMatchesSuite struct {
	suite.Suite
	ctx    context.Context
	mock   sqlmock.Sqlmock
	db     *gorm.DB
	logger hclog.Logger
	buffer *bytes.Buffer
}

func (s *CreateMatchesSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)
	s.db, s.mock = database.NewMockedGormDB()
}

func (s *CreateMatchesSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *CreateMatchesSuite) Test_CreateMatches() {
	r := require.New(s.T())

	resource, databaseURL, err := database.NewTestPostgresDB(false)
	r.NoError(err)
	defer resource.Close()

	r.NoError(database.Migrate(databaseURL))

	db, err := database.NewGormDB(databaseURL)
	r.NoError(err)

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

	// Add members to the database
	members := []struct {
		userID              string
		gender              models.Gender
		isActive            bool
		hasGenderPreference bool
	}{
		{"U0123456789", models.Male, true, false},
		{"U3234567890", models.Female, true, false},
		{"U7812309456", models.Female, true, true},
		{"U8765432109", models.Male, false, true},
		{"U5647382910", models.Female, false, false},
		{"U0487326159", models.Male, true, true},
		{"U0693126494", models.Male, true, true},
	}

	for _, member := range members {
		db.Create(&models.Member{
			ChannelID:           channelID,
			UserID:              member.userID,
			Gender:              member.gender,
			IsActive:            &member.isActive,
			HasGenderPreference: &member.hasGenderPreference,
		})
	}

	// Write a record in the rounds table
	db.Create(&models.Round{
		ChannelID: channelID,
	})

	err = CreateMatches(s.ctx, db, nil, &CreateMatchesParams{
		ChannelID: channelID,
		RoundID:   1,
	})
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new match to the database")
	r.Contains(s.buffer.String(), "paired active participants for chat-roulette")

	// Verify matches
	var count int64
	result := db.Model(&models.Job{}).Where("job_type = ?", models.JobTypeCreatePair).Count(&count)
	r.NoError(result.Error)
	r.Equal(int64(2), count)
}

func (s *CreateMatchesSuite) Test_QueueCreateMatchesJob() {
	r := require.New(s.T())

	p := &CreateMatchesParams{
		ChannelID: "C0123456789",
		RoundID:   1,
	}

	database.MockQueueJob(s.mock, p, models.JobTypeCreateMatches.String(), models.JobPriorityLow)

	err := QueueCreateMatchesJob(s.ctx, s.db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")
}

func Test_CreateMatches_suite(t *testing.T) {
	suite.Run(t, new(CreateMatchesSuite))
}
