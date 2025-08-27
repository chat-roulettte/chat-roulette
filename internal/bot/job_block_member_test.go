package bot

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/ory/dockertest"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slacktest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
)

var (
	channelID = "C1234567890"
	botUser   = "U023BECGF"
)

type BlockMemberSuite struct {
	suite.Suite
	ctx         context.Context
	db          *gorm.DB
	databaseURL string
	client      *slack.Client
	testServer  *slacktest.Server
	logger      hclog.Logger
	buffer      *bytes.Buffer
	resource    *dockertest.Resource
}

func (s *BlockMemberSuite) SetupSuite() {
	r := s.Require()

	resource, databaseURL, err := database.NewTestPostgresDB(false) // don't migrate yet
	r.NoError(err)
	s.resource = resource
	s.databaseURL = databaseURL

	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)

	// Create Slack test server with custom /users.info endpoint
	s.testServer = slacktest.NewTestServer(func(c slacktest.Customize) {
		c.Handle("/users.info", func(w http.ResponseWriter, r *http.Request) {
			r.ParseForm()
			userID := r.PostFormValue("user")

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			isBot := userID == botUser
			fmt.Fprintf(w, `{
				"ok": true,
				"user": {
					"id": "%s",
					"name": "test",
					"is_bot": %t
				}
			}`, userID, isBot)
		})
	})
	go s.testServer.Start()

	s.client = slack.New("xoxb-test-token-here", slack.OptionAPIURL(s.testServer.GetAPIURL()))
}

func (s *BlockMemberSuite) SetupTest() {
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
}

func (s *BlockMemberSuite) TearDownTest() {
	require.NoError(s.T(), database.CleanPostgresDB(s.db))
}

func (s *BlockMemberSuite) TearDownSuite() {
	s.testServer.Stop()
	s.resource.Close()
}

func (s *BlockMemberSuite) Test_BlockMember() {
	r := require.New(s.T())

	userID := "U0123456789"
	memberID := "U8765432109"

	// Write members to the database
	members := []struct {
		userID string
	}{
		{userID},
		{memberID},
	}

	isActive := true
	hasGenderPreference := false
	for _, member := range members {
		s.db.Create(&models.Member{
			ChannelID:           channelID,
			UserID:              member.userID,
			Gender:              models.Male,
			IsActive:            &isActive,
			HasGenderPreference: &hasGenderPreference,
		})
	}

	// Block member
	p := &BlockMemberParams{
		UserID:   userID,
		MemberID: memberID,
	}

	err := BlockMember(s.ctx, s.db, s.client, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "blocked Slack member from matching with this user")
	r.Contains(s.buffer.String(), fmt.Sprintf("slack_user_id=%s", userID))

	// Verify block was created
	var blockedMember *models.BlockedMember
	result := s.db.Model(&models.BlockedMember{}).
		Where("channel_id = ?", channelID).
		Where("user_id = ?", userID).
		Where("member_id = ?", memberID).
		First(&blockedMember)
	r.NoError(result.Error)
	r.Equal(result.RowsAffected, int64(1))
}

func (s *BlockMemberSuite) Test_BlockMember_AlreadyBlocked() {
	r := require.New(s.T())

	userID := "U0123456789"
	memberID := "U8765432109"

	// Write members to the database
	members := []struct {
		userID string
	}{
		{userID},
		{memberID},
	}

	isActive := true
	hasGenderPreference := false
	for _, member := range members {
		s.db.Create(&models.Member{
			ChannelID:           channelID,
			UserID:              member.userID,
			Gender:              models.Male,
			IsActive:            &isActive,
			HasGenderPreference: &hasGenderPreference,
		})
	}

	// Create existing block
	s.db.Create(&models.BlockedMember{
		ChannelID: channelID,
		UserID:    userID,
		MemberID:  memberID,
	})

	// Try to block the same member again
	p := &BlockMemberParams{
		UserID:   userID,
		MemberID: memberID,
	}

	err := BlockMember(s.ctx, s.db, s.client, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "Slack member is already blocked by this user")

	// Verify only one block exists
	var count int64
	result := s.db.Model(&models.BlockedMember{}).
		Where("channel_id = ?", channelID).
		Where("user_id = ?", userID).
		Where("member_id = ?", memberID).
		Count(&count)
	r.NoError(result.Error)
	r.Equal(int64(1), count)
}

func (s *BlockMemberSuite) Test_BlockMember_SelfBlock() {
	r := require.New(s.T())

	userID := "U0123456789"

	// Write member to the database
	isActive := true
	hasGenderPreference := false
	s.db.Create(&models.Member{
		ChannelID:           channelID,
		UserID:              userID,
		Gender:              models.Male,
		IsActive:            &isActive,
		HasGenderPreference: &hasGenderPreference,
	})

	// Try to block self
	p := &BlockMemberParams{
		UserID:   userID,
		MemberID: userID,
	}

	err := BlockMember(s.ctx, s.db, s.client, p)
	r.Error(err)
	r.Contains(err.Error(), "failed to validate job parameters")

	// Verify no block was created
	var count int64
	result := s.db.Model(&models.BlockedMember{}).
		Where("channel_id = ?", channelID).
		Where("user_id = ?", userID).
		Count(&count)
	r.NoError(result.Error)
	r.Equal(int64(0), count)
}

func (s *BlockMemberSuite) Test_BlockMember_BlockBot() {
	r := require.New(s.T())

	userID := "U0123456789"

	// Write member to the database
	isActive := true
	hasGenderPreference := false
	s.db.Create(&models.Member{
		ChannelID:           channelID,
		UserID:              userID,
		Gender:              models.Male,
		IsActive:            &isActive,
		HasGenderPreference: &hasGenderPreference,
	})

	// Try to block the bot
	p := &BlockMemberParams{
		UserID:   userID,
		MemberID: "U023BECGF", // Slack bot
	}

	err := BlockMember(s.ctx, s.db, s.client, p)
	r.Nil(err)
	r.Contains(s.buffer.String(), "skipping because this Slack user is a bot")

	// Verify no block was created
	var count int64
	result := s.db.Model(&models.BlockedMember{}).
		Where("channel_id = ?", channelID).
		Where("user_id = ?", userID).
		Count(&count)
	r.NoError(result.Error)
	r.Equal(int64(0), count)
}

func (s *BlockMemberSuite) Test_BlockMember_FailedValidation() {
	r := require.New(s.T())

	p := &BlockMemberParams{
		UserID:   "U0123456789",
		MemberID: "A B C D E F",
	}

	err := BlockMember(s.ctx, s.db, s.client, p)
	r.Error(err)
	r.Equal(err, models.ErrJobParamsFailedValidation)
	r.Contains(s.buffer.String(), "failed to validate job parameters")
}

func (s *BlockMemberSuite) Test_QueueBlockMemberJob() {
	r := require.New(s.T())

	p := &BlockMemberParams{
		UserID:   "U0123456789",
		MemberID: "U8765432109",
	}

	db, mock := database.NewMockedGormDB()

	database.MockQueueJob(mock, p, models.JobTypeBlockMember.String(), models.JobPriorityHigh)

	err := QueueBlockMemberJob(s.ctx, db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")

	r.NoError(mock.ExpectationsWereMet())
}

func Test_BlockMember_suite(t *testing.T) {
	suite.Run(t, new(BlockMemberSuite))
}
