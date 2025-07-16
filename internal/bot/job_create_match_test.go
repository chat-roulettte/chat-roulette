package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/ory/dockertest"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
)

type CreateMatchSuite struct {
	suite.Suite
	ctx      context.Context
	db       *gorm.DB
	logger   hclog.Logger
	buffer   *bytes.Buffer
	resource *dockertest.Resource
}

func (s *CreateMatchSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)

	resource, databaseURL, err := database.NewTestPostgresDB(false)
	require.NoError(s.T(), err)
	s.resource = resource

	db, err := database.NewGormDB(databaseURL)
	require.NoError(s.T(), err)
	require.NoError(s.T(), database.Migrate(databaseURL))

	s.db = db
}

func (s *CreateMatchSuite) AfterTest(_, _ string) {
	s.resource.Close()
}

func (s *CreateMatchSuite) Test_CreateMatch() {
	r := require.New(s.T())

	channelID := "C0123456789"

	// Write channel to the database
	s.db.Create(&models.Channel{
		ChannelID:      channelID,
		Inviter:        "U9876543210",
		ConnectionMode: models.VirtualConnectionMode,
		Interval:       models.Biweekly,
		Weekday:        time.Friday,
		Hour:           12,
		NextRound:      time.Now().Add(336 * time.Hour), // 14 days in the future
	})

	// Add members to the database
	members := []struct {
		userID              string
		gender              models.Gender
		isActive            bool
		hasGenderPreference bool
	}{
		{"U0123456789", models.Male, true, false}, // match partner
		{"U8765432109", models.Male, false, true},
		{"U3234567890", models.Female, true, false},
		{"U7812309456", models.Female, true, true},
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

	// Write a record in the rounds table and create matches
	s.db.Create(&models.Round{
		ChannelID: channelID,
		HasEnded:  false,
	})

	err := CreateMatches(s.ctx, s.db, nil, &CreateMatchesParams{
		ChannelID: channelID,
		RoundID:   1,
	})
	r.NoError(err)

	// Add record for the future CREATE_ROUND job
	p := &CreateRoundParams{
		ChannelID: channelID,
		NextRound: time.Now().Add(336 * time.Hour),
		Interval:  models.Biweekly.String(),
	}

	data, _ := json.Marshal(p)

	s.db.Create(&models.Job{
		JobID:       ksuid.New(),
		JobType:     models.JobTypeCreateRound,
		Priority:    models.JobPriorityStandard,
		Status:      models.JobStatusSucceeded,
		IsCompleted: true,
		Data:        data,
		ExecAt:      time.Now().UTC().Add(-(24 * time.Hour)),
	})

	// Ensure pairings table is populated with rows
	var jobs []models.Job

	err = s.db.Model(&models.Job{}).
		Where("job_type = ?", models.JobTypeCreatePair.String()).
		Where("status = ?", models.JobStatusPending).
		Where("is_completed = false").
		Find(&jobs).Error
	r.NoError(err)

	for _, job := range jobs {
		err := ExecJob(s.ctx, s.db, nil, &job, CreatePair)
		r.NoError(err)
	}

	// Add the new participant who will be matched
	isActive := true
	hasGenderPreference := true
	newParticipant := &models.Member{
		ChannelID:           channelID,
		UserID:              "U9261442153",
		Gender:              models.Male,
		IsActive:            &isActive,
		HasGenderPreference: &hasGenderPreference,
	}

	s.db.Create(newParticipant)

	err = CreateMatch(s.ctx, s.db, nil, &CreateMatchParams{
		ChannelID:   newParticipant.ChannelID,
		Participant: newParticipant.UserID,
	})
	r.NoError(err)

	// Verify CREATE_PAIR job was queued
	var count int64
	result := s.db.Model(&models.Job{}).
		Where("job_type = ?", models.JobTypeCreatePair).
		Where(datatypes.JSONQuery("data").Equals(newParticipant.UserID, "participant")).
		Where(datatypes.JSONQuery("data").Equals("U0123456789", "partner")).
		Count(&count)
	r.NoError(result.Error)
	r.Equal(int64(1), count)
}

func (s *CreateMatchSuite) Test_CreateMatch_NoActiveRound() {
	r := require.New(s.T())

	channelID := "C0123456789"

	// Write channel to the database
	s.db.Create(&models.Channel{
		ChannelID:      channelID,
		Inviter:        "U9876543210",
		ConnectionMode: models.VirtualConnectionMode,
		Interval:       models.Biweekly,
		Weekday:        time.Friday,
		Hour:           12,
		NextRound:      time.Now().Add(1 * time.Hour),
	})

	// Round has ended
	s.db.Create(&models.Round{
		ChannelID: channelID,
		HasEnded:  true,
	})

	err := CreateMatch(s.ctx, s.db, nil, &CreateMatchParams{
		ChannelID:   channelID,
		Participant: "U3234567890",
	})
	r.NoError(err)
	r.Contains(s.buffer.String(), "unable to match participant: no active Chat Roulette round found")
}

func (s *CreateMatchSuite) Test_CreateMatch_ExceededMidPointInActiveRound() {
	r := require.New(s.T())

	channelID := "C0123456789"

	// Write channel to the database
	s.db.Create(&models.Channel{
		ChannelID:      channelID,
		Inviter:        "U9876543210",
		ConnectionMode: models.VirtualConnectionMode,
		Interval:       models.Biweekly,
		Weekday:        time.Friday,
		Hour:           12,
		NextRound:      time.Now().Add(24 * time.Hour),
	})

	// Round is in progress
	s.db.Create(&models.Round{
		ChannelID: channelID,
		HasEnded:  false,
	})

	// Add record for past CREATE_ROUND job
	p := &CreateRoundParams{
		ChannelID: channelID,
		NextRound: time.Now().Add(24 * time.Hour),
		Interval:  models.Biweekly.String(),
	}

	data, _ := json.Marshal(p)

	s.db.Create(&models.Job{
		JobID:       ksuid.New(),
		JobType:     models.JobTypeCreateRound,
		Priority:    models.JobPriorityStandard,
		Status:      models.JobStatusSucceeded,
		IsCompleted: true,
		Data:        data,
		ExecAt:      time.Now().Add(-(336 * time.Hour)), // 14 days ago
	})

	err := CreateMatch(s.ctx, s.db, nil, &CreateMatchParams{
		ChannelID:   channelID,
		Participant: "U3234567890",
	})
	r.NoError(err)
	r.Contains(s.buffer.String(), "unable to match participant: not enough time remaining in current Chat Roulette round")
}

func (s *CreateMatchSuite) Test_CreateMatch_HasGenderPreference() {
	r := require.New(s.T())

	channelID := "C0123456789"

	// Write channel to the database
	s.db.Create(&models.Channel{
		ChannelID:      channelID,
		Inviter:        "U9876543210",
		ConnectionMode: models.VirtualConnectionMode,
		Interval:       models.Biweekly,
		Weekday:        time.Friday,
		Hour:           12,
		NextRound:      time.Now().Add(504 * time.Hour), // 21 days in the future
	})

	// Add members to the database
	members := []struct {
		userID              string
		gender              models.Gender
		isActive            bool
		hasGenderPreference bool
	}{
		// This user has not been matched since they were inactive when the round started
		{"U8765432109", models.Female, false, false},
		// These users will all get matched
		{"U3234567890", models.Female, true, false},
		{"U7812309456", models.Female, true, true},
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

	// Write a record in the rounds table and create matches
	s.db.Create(&models.Round{
		ChannelID: channelID,
		HasEnded:  false,
	})

	err := CreateMatches(s.ctx, s.db, nil, &CreateMatchesParams{
		ChannelID: channelID,
		RoundID:   1,
	})
	r.NoError(err)

	// Add record for the CREATE_ROUND job
	createRoundParams := &CreateRoundParams{
		ChannelID: channelID,
		NextRound: time.Now().Add(336 * time.Hour),
		Interval:  models.Biweekly.String(),
	}

	data, _ := json.Marshal(createRoundParams)

	s.db.Create(&models.Job{
		JobID:       ksuid.New(),
		JobType:     models.JobTypeCreateRound,
		Priority:    models.JobPriorityStandard,
		Status:      models.JobStatusSucceeded,
		IsCompleted: true,
		Data:        data,
		ExecAt:      time.Now().UTC().Add(-(24 * time.Hour)),
	})

	// Ensure pairings table is populated with rows
	var jobs []models.Job

	err = s.db.Model(&models.Job{}).
		Where("job_type = ?", models.JobTypeCreatePair.String()).
		Where("status = ?", models.JobStatusPending).
		Where("is_completed = false").
		Find(&jobs).Error
	r.NoError(err)

	for _, job := range jobs {
		err := ExecJob(s.ctx, s.db, nil, &job, CreatePair)
		r.NoError(err)
	}

	// Set the inactive, standby participant to active
	err = s.db.Model(&models.Member{}).Where("user_id = ?", "U8765432109").Update("is_active", true).Update("has_gender_preference", true).Error
	r.NoError(err)

	// Test when the only other unmatched participant is not the same gender
	isActive := true
	hasGenderPreference := true
	newParticipant := &models.Member{
		ChannelID:           channelID,
		UserID:              "U9261442153",
		Gender:              models.Male,
		IsActive:            &isActive,
		HasGenderPreference: &hasGenderPreference,
	}

	s.db.Create(newParticipant)

	err = CreateMatch(s.ctx, s.db, nil, &CreateMatchParams{
		ChannelID:   newParticipant.ChannelID,
		Participant: newParticipant.UserID,
	})
	r.NoError(err)
	r.Contains(s.buffer.String(), "unable to match participant: no suitable partner found")
}

func (s *CreateMatchSuite) Test_CreateMatch_NoMatchFound() {
	r := require.New(s.T())

	channelID := "C0123456789"

	// Write channel to the database
	s.db.Create(&models.Channel{
		ChannelID:      channelID,
		Inviter:        "U9876543210",
		ConnectionMode: models.VirtualConnectionMode,
		Interval:       models.Biweekly,
		Weekday:        time.Friday,
		Hour:           12,
		NextRound:      time.Now().Add(504 * time.Hour), // 21 days in the future
	})

	// Add members to the database
	members := []struct {
		userID              string
		gender              models.Gender
		isActive            bool
		hasGenderPreference bool
	}{
		{"U8765432109", models.Male, false, false}, // targeted match partner
		{"U3234567890", models.Female, true, false},
		{"U7812309456", models.Female, true, true},
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

	// Write a record in the rounds table and create matches
	s.db.Create(&models.Round{
		ChannelID: channelID,
		HasEnded:  false,
	})

	err := CreateMatches(s.ctx, s.db, nil, &CreateMatchesParams{
		ChannelID: channelID,
		RoundID:   1,
	})
	r.NoError(err)

	// Add record for the CREATE_ROUND job
	createRoundParams := &CreateRoundParams{
		ChannelID: channelID,
		NextRound: time.Now().Add(336 * time.Hour),
		Interval:  models.Biweekly.String(),
	}

	data, _ := json.Marshal(createRoundParams)

	s.db.Create(&models.Job{
		JobID:       ksuid.New(),
		JobType:     models.JobTypeCreateRound,
		Priority:    models.JobPriorityStandard,
		Status:      models.JobStatusSucceeded,
		IsCompleted: true,
		Data:        data,
		ExecAt:      time.Now().UTC().Add(-(24 * time.Hour)),
	})

	// Ensure pairings table is populated with rows
	var jobs []models.Job

	err = s.db.Model(&models.Job{}).
		Where("job_type = ?", models.JobTypeCreatePair.String()).
		Where("status = ?", models.JobStatusPending).
		Where("is_completed = false").
		Find(&jobs).Error
	r.NoError(err)

	for _, job := range jobs {
		err := ExecJob(s.ctx, s.db, nil, &job, CreatePair)
		r.NoError(err)
	}

	// Test when there are no active participants to match with
	isActive := true
	hasGenderPreference := false
	newParticipant := &models.Member{
		ChannelID:           channelID,
		UserID:              "U9261442153",
		Gender:              models.Male,
		IsActive:            &isActive,
		HasGenderPreference: &hasGenderPreference,
	}

	s.db.Create(newParticipant)

	err = CreateMatch(s.ctx, s.db, nil, &CreateMatchParams{
		ChannelID:   newParticipant.ChannelID,
		Participant: newParticipant.UserID,
	})
	r.NoError(err)
	r.Contains(s.buffer.String(), "unable to match participant: no suitable partner found")

	// Test when the selected partner has already been matched
	err = s.db.Model(&models.Member{}).Where("user_id = ?", "U8765432109").Update("is_active", true).Error
	r.NoError(err)

	createPairParams := &CreatePairParams{
		ChannelID:   channelID,
		MatchID:     4,
		Participant: "U8765432109",
		Partner:     "U9261442153",
	}

	data, _ = json.Marshal(createPairParams)

	s.db.Create(&models.Job{
		JobID:       ksuid.New(),
		JobType:     models.JobTypeCreatePair,
		Priority:    models.JobPriorityStandard,
		Status:      models.JobStatusPending,
		IsCompleted: false,
		Data:        data,
		ExecAt:      time.Now().UTC().Add((30 * time.Second)),
	})

	err = CreateMatch(s.ctx, s.db, nil, &CreateMatchParams{
		ChannelID:   newParticipant.ChannelID,
		Participant: newParticipant.UserID,
	})
	r.NoError(err)
	r.Contains(s.buffer.String(), "unable to match participant: partner has already been matched")
}

func (s *CreateMatchSuite) Test_QueueCreateMatchJob() {
	r := require.New(s.T())

	p := &CreateMatchParams{
		ChannelID:   "C0123456789",
		Participant: "U9261442153",
	}

	db, mock := database.NewMockedGormDB()

	database.MockQueueJob(mock, p, models.JobTypeCreateMatch.String(), models.JobPriorityLow)

	err := QueueCreateMatchJob(s.ctx, db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")

	r.NoError(mock.ExpectationsWereMet())
}

func Test_CreateMatch_suite(t *testing.T) {
	suite.Run(t, new(CreateMatchSuite))
}
