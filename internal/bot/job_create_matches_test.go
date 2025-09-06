package bot

import (
	"bytes"
	"context"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/ory/dockertest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
)

type CreateMatchesSuite struct {
	suite.Suite
	ctx         context.Context
	db          *gorm.DB
	databaseURL string
	logger      hclog.Logger
	buffer      *bytes.Buffer
	resource    *dockertest.Resource
}

func (s *CreateMatchesSuite) SetupSuite() {
	r := s.Require()

	resource, databaseURL, err := database.NewTestPostgresDB(false) // don't migrate yet
	r.NoError(err)
	s.resource = resource
	s.databaseURL = databaseURL

	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)
}

func (s *CreateMatchesSuite) SetupTest() {
	r := s.Require()

	// Create new gorm.DB to ensure fresh connections
	db, err := database.NewGormDB(s.databaseURL)
	r.NoError(err)
	s.db = db

	// Ensure fresh schema before every test
	r.NoError(database.Migrate(s.databaseURL))
}

func (s *CreateMatchesSuite) TearDownTest() {
	require.NoError(s.T(), database.CleanPostgresDB(s.db))
}

func (s *CreateMatchesSuite) Test_CreateMatches() {
	r := require.New(s.T())

	channelID := "C0123456789"

	// Write channel to the database
	s.db.Create(&models.Channel{
		ChannelID:      channelID,
		Inviter:        "U9876543210",
		ConnectionMode: models.ConnectionModeVirtual,
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
		s.db.Create(&models.Member{
			ChannelID:           channelID,
			UserID:              member.userID,
			Gender:              member.gender,
			IsActive:            &member.isActive,
			HasGenderPreference: &member.hasGenderPreference,
		})
	}

	// Write a record in the rounds table
	s.db.Create(&models.Round{
		ChannelID: channelID,
	})

	// Test
	err := CreateMatches(s.ctx, s.db, nil, &CreateMatchesParams{
		ChannelID: channelID,
		RoundID:   1,
	})
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new match to the database")
	r.Contains(s.buffer.String(), "paired active participants for chat-roulette")
	r.Contains(s.buffer.String(), "participants=5")
	r.Contains(s.buffer.String(), "pairs=2")
	r.Contains(s.buffer.String(), "unpaired=1")

	// Verify matches
	var count int64
	result := s.db.Model(&models.Job{}).Where("job_type = ?", models.JobTypeCreatePair).Count(&count)
	r.NoError(result.Error)
	r.Equal(int64(2), count)

	// Verify unmatched participants were notified
	result = s.db.Model(&models.Job{}).Where("job_type = ?", models.JobTypeNotifyMember).Count(&count)
	r.NoError(result.Error)
	r.Equal(int64(1), count)

	// Verify REPORT_MATCHES job was queued
	result = s.db.Model(&models.Job{}).Where("job_type = ?", models.JobTypeReportMatches).Count(&count)
	r.NoError(result.Error)
	r.Equal(int64(1), count)
}

func (s *CreateMatchesSuite) Test_CreateMatches_ConnectionModes() {
	r := require.New(s.T())

	channelID := "C0123456789"

	// Write channel to the database
	s.db.Create(&models.Channel{
		ChannelID:      channelID,
		Inviter:        "U9876543210",
		ConnectionMode: models.ConnectionModeHybrid,
		Interval:       models.Monthly,
		Weekday:        time.Tuesday,
		Hour:           12,
		NextRound:      time.Now().Add(24 * time.Hour),
	})

	// Add members to the database
	members := []struct {
		userID              string
		gender              models.Gender
		connectionMode      models.ConnectionMode
		isActive            bool
		hasGenderPreference bool
	}{
		// These two are not active in this:
		{"U8765432109", models.Male, models.ConnectionModePhysical, false, true},
		{"U5647382910", models.Female, models.ConnectionModeHybrid, false, false},
		// These two will be matched:
		{"U3234567890", models.Female, models.ConnectionModeVirtual, true, false},
		{"U7812309456", models.Female, models.ConnectionModePhysical, true, true},
		// These two will be matched:
		{"U0487326159", models.Male, models.ConnectionModeVirtual, true, true},
		{"U0693126494", models.Male, models.ConnectionModeVirtual, true, true},
		// This one will be not be matched with anyone:
		{"U0123456789", models.Male, models.ConnectionModePhysical, true, false},
	}

	for _, member := range members {
		s.db.Create(&models.Member{
			ChannelID:           channelID,
			UserID:              member.userID,
			Gender:              member.gender,
			ConnectionMode:      member.connectionMode,
			IsActive:            &member.isActive,
			HasGenderPreference: &member.hasGenderPreference,
		})
	}

	// Write a record in the rounds table
	s.db.Create(&models.Round{
		ChannelID: channelID,
	})

	// Test
	err := CreateMatches(s.ctx, s.db, nil, &CreateMatchesParams{
		ChannelID: channelID,
		RoundID:   1,
	})
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new match to the database")
	r.Contains(s.buffer.String(), "paired active participants for chat-roulette")
	r.Contains(s.buffer.String(), "participants=5")
	r.Contains(s.buffer.String(), "pairs=2")
	r.Contains(s.buffer.String(), "unpaired=1")

	// Verify matches
	var count int64
	result := s.db.Model(&models.Job{}).Where("job_type = ?", models.JobTypeCreatePair).Count(&count)
	r.NoError(result.Error)
	r.Equal(int64(2), count)

	// Verify connection modes were respected
	pairs := [][2]string{
		{"U0487326159", "U0693126494"},
		{"U7812309456", "U3234567890"},
	}

	query := s.db.Model(&models.Job{}).
		Where("job_type = ?", models.JobTypeCreatePair)

	subQuery := s.db

	for _, pair := range pairs {
		a, b := pair[0], pair[1]

		// Each pair can match in either order
		condition := s.db.
			Where(datatypes.JSONQuery("data").Equals(a, "participant")).
			Where(datatypes.JSONQuery("data").Equals(b, "partner")).
			Or(
				s.db.
					Where(datatypes.JSONQuery("data").Equals(b, "participant")).
					Where(datatypes.JSONQuery("data").Equals(a, "partner")),
			)

		subQuery = subQuery.Or(condition)
	}
	result = query.Where(subQuery).Count(&count)
	r.NoError(result.Error)
	r.Equal(int64(2), count)
}

func (s *CreateMatchesSuite) Test_CreateMatches_SameGenderTwoParticipants() {
	r := require.New(s.T())

	channelID := "C0123456789"

	// Write channel to the database
	s.db.Create(&models.Channel{
		ChannelID:      channelID,
		Inviter:        "U9876543210",
		ConnectionMode: models.ConnectionModeVirtual,
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
		{"U8765432109", models.Male, true, true},
	}

	for _, member := range members {
		s.db.Create(&models.Member{
			ChannelID:           channelID,
			UserID:              member.userID,
			Gender:              member.gender,
			IsActive:            &member.isActive,
			HasGenderPreference: &member.hasGenderPreference,
		})
	}

	// Write a record in the rounds table
	s.db.Create(&models.Round{
		ChannelID: channelID,
	})

	// Test
	err := CreateMatches(s.ctx, s.db, nil, &CreateMatchesParams{
		ChannelID: channelID,
		RoundID:   1,
	})
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new match to the database")
	r.Contains(s.buffer.String(), "paired active participants for chat-roulette")
	r.Contains(s.buffer.String(), "participants=2")
	r.Contains(s.buffer.String(), "pairs=1")
	r.Contains(s.buffer.String(), "unpaired=0")

	// Verify matches
	var count int64
	result := s.db.Model(&models.Job{}).Where("job_type = ?", models.JobTypeCreatePair).Count(&count)
	r.NoError(result.Error)
	r.Equal(int64(1), count)
}

func (s *CreateMatchesSuite) Test_QueueCreateMatchesJob() {
	db, mock := database.NewMockedGormDB()

	r := require.New(s.T())

	p := &CreateMatchesParams{
		ChannelID: "C0123456789",
		RoundID:   1,
	}

	database.MockQueueJob(mock, p, models.JobTypeCreateMatches.String(), models.JobPriorityLow)

	err := QueueCreateMatchesJob(s.ctx, db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")
	r.NoError(mock.ExpectationsWereMet())
}

func Test_CreateMatches_suite(t *testing.T) {
	suite.Run(t, new(CreateMatchesSuite))
}
