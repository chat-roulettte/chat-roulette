package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hashicorp/go-hclog"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slacktest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
)

type SyncChannelsSuite struct {
	suite.Suite
	ctx    context.Context
	mock   sqlmock.Sqlmock
	db     *gorm.DB
	logger hclog.Logger
	buffer *bytes.Buffer
}

func (s *SyncChannelsSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)
	s.db, s.mock = database.NewMockedGormDB()
}

func (s *SyncChannelsSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *SyncChannelsSuite) Test_SyncChannels() {
	r := require.New(s.T())

	resource, databaseURL, err := database.NewTestPostgresDB(false)
	r.NoError(err)
	defer resource.Close()

	r.NoError(database.Migrate(databaseURL))

	db, err := database.NewGormDB(databaseURL)
	r.NoError(err)

	inviter := "U9876543210"

	addChannel := "C5555555555"      // Does not exist
	awaitingChannel := "C1928374560" // Does not exist in DB, awaiting admin onboarding
	deleteChannel := "C9999999999"   // Exists in DB, but needs to be deleted
	syncChannel := "C0123456789"     // Exists in DB

	// Mock Slack API call
	handlerFn := func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"ok": true,
			"channels": []slack.Channel{
				{
					GroupConversation: slack.GroupConversation{
						Conversation: slack.Conversation{
							ID: syncChannel,
						},
						Creator: inviter,
					},
				},
				{
					GroupConversation: slack.GroupConversation{
						Conversation: slack.Conversation{
							ID: addChannel,
						},
						Creator: inviter,
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}

	slackServer := slacktest.NewTestServer()
	slackServer.Handle("/users.conversations", handlerFn)
	go slackServer.Start()
	defer slackServer.Stop()

	client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(slackServer.GetAPIURL()))

	// Write any existing channels to the database
	db.Create(&models.Channel{
		ChannelID:      deleteChannel,
		Inviter:        inviter,
		ConnectionMode: models.ConnectionModeVirtual,
		Interval:       models.Biweekly,
		Weekday:        time.Friday,
		Hour:           12,
		NextRound:      time.Now().Add(24 * time.Hour),
	})

	// Add job record for GREET_ADMIN to show admin onboarding hasnt been completed
	QueueGreetAdminJob(context.Background(), db, &GreetAdminParams{
		ChannelID: awaitingChannel,
		Inviter:   inviter,
	})

	p := &SyncChannelsParams{
		BotUserID: "U1111111111",
	}

	err = SyncChannels(s.ctx, db, client, p)
	r.NoError(err)

	// Verify new jobs were queued
	var count int64
	err = db.Model(&models.Job{}).
		Where("job_type = ?", models.JobTypeGreetAdmin).
		Or("job_type = ?", models.JobTypeDeleteChannel).
		Or("job_type = ?", models.JobTypeSyncMembers).
		Count(&count).Error
	r.NoError(err)
	r.Equal(int64(4), count)
}

func (s *SyncChannelsSuite) Test_SyncChannelsJob() {
	r := require.New(s.T())

	p := &SyncChannelsParams{
		BotUserID: "U1111111111",
	}

	database.MockQueueJob(
		s.mock,
		p,
		models.JobTypeSyncChannels.String(),
		models.JobPriorityHighest,
	)

	err := QueueSyncChannelsJob(s.ctx, s.db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")
}

func Test_SyncChannels_suite(t *testing.T) {
	suite.Run(t, new(SyncChannelsSuite))
}
