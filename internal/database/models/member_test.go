package models_test

import (
	"context"
	"testing"
	"time"

	"github.com/ory/dockertest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
)

var (
	channelID = "C1234567890"
)

type MemberSuite struct {
	suite.Suite
	db          *gorm.DB
	databaseURL string
	resource    *dockertest.Resource
}

func (s *MemberSuite) SetupSuite() {
	r := s.Require()

	resource, databaseURL, err := database.NewTestPostgresDB(false) // don't migrate yet
	r.NoError(err)
	s.resource = resource
	s.databaseURL = databaseURL
}

func (s *MemberSuite) SetupTest() {
	r := s.Require()

	// Create new gorm.DB to ensure fresh connections
	db, err := database.NewGormDB(s.databaseURL)
	r.NoError(err)
	s.db = db

	// Ensure fresh schema before every test
	r.NoError(database.Migrate(s.databaseURL))

	// Write channel to the database
	s.db.Create(&models.Channel{
		ChannelID:      channelID,
		Inviter:        "U9876543210",
		ConnectionMode: models.VirtualConnectionMode,
		Interval:       models.Biweekly,
		Weekday:        time.Friday,
		Hour:           12,
		NextRound:      time.Now().Add(72 * time.Hour), // 3 days in the future
	})
}

func (s *MemberSuite) TearDownTest() {
	require.NoError(s.T(), database.CleanPostgresDB(s.db))
}

func (s *MemberSuite) TearDownSuite() {
	s.resource.Close()
}

func (s *MemberSuite) TestGetMemberByUserID() {
	r := require.New(s.T())

	userID := "U1234567890"

	isActive := true
	hasGenderPref := false
	member := &models.Member{
		ChannelID:           channelID,
		UserID:              userID,
		Gender:              models.Male,
		IsActive:            &isActive,
		HasGenderPreference: &hasGenderPref,
	}
	err := s.db.Create(member).Error
	r.NoError(err)

	result, err := models.GetMemberByUserID(context.Background(), s.db, channelID, userID)
	r.NoError(err)
	r.NotNil(result)
	r.Equal(channelID, result.ChannelID)
	r.Equal(userID, result.UserID)
}

func (s *MemberSuite) TestGetMemberByUserID_NotFound() {
	r := require.New(s.T())
	channelID := "C0000000000"
	userID := "U0000000000"

	result, err := models.GetMemberByUserID(context.Background(), s.db, channelID, userID)
	r.Error(err)
	r.Nil(result)
	r.Contains(err.Error(), "failed to retrieve member by user_id")
}

func (s *MemberSuite) TestGetMemberByUserID_ContextCanceled() {
	r := require.New(s.T())

	userID := "U1234567890"

	isActive := true
	hasGenderPref := false
	member := &models.Member{
		ChannelID:           channelID,
		UserID:              userID,
		Gender:              models.Male,
		IsActive:            &isActive,
		HasGenderPreference: &hasGenderPref,
	}
	err := s.db.Create(member).Error
	r.NoError(err)

	// Create a context that is already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := models.GetMemberByUserID(ctx, s.db, channelID, userID)
	r.Error(err)
	r.Nil(result)
	r.Contains(err.Error(), "failed to retrieve member by user_id")
}

func TestMemberModelSuite(t *testing.T) {
	suite.Run(t, new(MemberSuite))
}
