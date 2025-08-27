package bot

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/ory/dockertest"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
)

type UnblockMemberSuite struct {
	suite.Suite
	ctx         context.Context
	db          *gorm.DB
	databaseURL string
	client      *slack.Client
	logger      hclog.Logger
	buffer      *bytes.Buffer
	resource    *dockertest.Resource
	channelID   string
	userID      string
}

func (s *UnblockMemberSuite) SetupSuite() {
	r := s.Require()

	resource, databaseURL, err := database.NewTestPostgresDB(false) // don't migrate yet
	r.NoError(err)
	s.resource = resource
	s.databaseURL = databaseURL

	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)

	s.channelID = "C1234567890"
	s.userID = "U0123456789"
}

func (s *UnblockMemberSuite) SetupTest() {
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
		ConnectionMode: models.ConnectionModeVirtual,
		Interval:       models.Biweekly,
		Weekday:        time.Friday,
		Hour:           12,
		NextRound:      time.Now().Add(72 * time.Hour), // 3 days in the future
	})

	// Write member to the database
	userID := "U0123456789"

	isActive := true
	hasGenderPreference := false
	s.db.Create(&models.Member{
		ChannelID:           channelID,
		UserID:              userID,
		Gender:              models.Male,
		IsActive:            &isActive,
		HasGenderPreference: &hasGenderPreference,
	})
}

func (s *UnblockMemberSuite) TearDownTest() {
	require.NoError(s.T(), database.CleanPostgresDB(s.db))
}

func (s *UnblockMemberSuite) TearDownSuite() {
	s.resource.Close()
}

func (s *UnblockMemberSuite) Test_UnblockMember() {
	r := require.New(s.T())

	// Block this member
	memberID := "U8765432109"

	s.db.Create(&models.BlockedMember{
		ChannelID: s.channelID,
		UserID:    s.userID,
		MemberID:  memberID,
	})

	// Unblock member
	p := &UnblockMemberParams{
		UserID:   s.userID,
		MemberID: memberID,
	}

	err := UnblockMember(s.ctx, s.db, s.client, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "successfully unblocked Slack member for this user")
	r.Contains(s.buffer.String(), fmt.Sprintf("slack_user_id=%s", s.userID))

	// Verify unblock
	var blockedMember *models.BlockedMember
	result := s.db.Model(&models.BlockedMember{}).
		Where("channel_id = ?", s.channelID).
		Where("user_id = ?", s.userID).
		Where("member_id = ?", memberID).
		First(&blockedMember)
	r.Error(result.Error)
	r.Equal(result.RowsAffected, int64(0))
}

func (s *UnblockMemberSuite) Test_UnblockMember_NotBlocked() {
	r := require.New(s.T())

	// Unblock member
	memberID := "U8765432109"

	p := &UnblockMemberParams{
		UserID:   s.userID,
		MemberID: memberID,
	}

	err := UnblockMember(s.ctx, s.db, s.client, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "no action taken: Slack member was not blocked for this user")
	r.Contains(s.buffer.String(), fmt.Sprintf("slack_user_id=%s", s.userID))
}

func (s *UnblockMemberSuite) Test_UnblockMember_FailedValidation() {
	r := require.New(s.T())

	p := &UnblockMemberParams{
		UserID:   s.userID,
		MemberID: "A B C D E F",
	}

	err := UnblockMember(s.ctx, s.db, s.client, p)
	r.Error(err)
	r.Equal(err, models.ErrJobParamsFailedValidation)
	r.Contains(s.buffer.String(), "failed to validate job parameters")
}

func (s *UnblockMemberSuite) Test_QueueUnblockMemberJob() {
	r := require.New(s.T())

	p := &UnblockMemberParams{
		UserID:   "U0123456789",
		MemberID: "U8765432109",
	}

	db, mock := database.NewMockedGormDB()

	database.MockQueueJob(mock, p, models.JobTypeUnblockMember.String(), models.JobPriorityHigh)

	err := QueueUnblockMemberJob(s.ctx, db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")

	r.NoError(mock.ExpectationsWereMet())
}

func Test_UnblockMember_suite(t *testing.T) {
	suite.Run(t, new(UnblockMemberSuite))
}
