package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hashicorp/go-hclog"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slacktest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/config"
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

	inviter := "U9876543210"

	addChannel := "C5555555555"
	deleteChannel := "C9999999999"
	syncChannel := "C0123456789"

	slackChannels := []string{
		fmt.Sprintf("%s,%s", syncChannel, inviter),
		fmt.Sprintf("%s,%s", addChannel, inviter),
	}

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

	// Mock DB call to retrieve the list of Slack channels
	dbChannels := append([]string{}, slackChannels[0])
	dbChannels = append(dbChannels, fmt.Sprintf("%s,%s", deleteChannel, inviter))

	s.mock.ExpectQuery(`SELECT "channel_id","inviter" FROM "channels"`).
		WillReturnRows(sqlmock.NewRows([]string{"channel_id", "inviter"}).FromCSVString(strings.Join(dbChannels, "\n")))

	// Mock the SYNC_MEMBERS job
	database.MockQueueJob(
		s.mock,
		&SyncMembersParams{
			ChannelID: syncChannel,
		},
		models.JobTypeSyncMembers.String(),
		models.JobPriorityHigh,
	)

	// Mock the ADD_CHANNEL job
	nextRound := FirstChatRouletteRound(time.Now().UTC(), "Monday", 12)

	database.MockQueueJob(
		s.mock,
		&AddChannelParams{
			ChannelID: addChannel,
			Invitor:   inviter,
			Interval:  "weekly",
			Weekday:   "Monday",
			Hour:      12,
			NextRound: nextRound,
		},
		models.JobTypeAddChannel.String(),
		models.JobPriorityHighest,
	)

	// Mock the DELETE_CHANNEL job
	database.MockQueueJob(
		s.mock,
		&DeleteChannelParams{
			ChannelID: deleteChannel,
		},
		models.JobTypeDeleteChannel.String(),
		models.JobPriorityHighest,
	)

	p := &SyncChannelsParams{
		BotUserID: "U1111111111",
		ChatRouletteConfig: config.ChatRouletteConfig{
			Interval: "weekly",
			Weekday:  "Monday",
			Hour:     12,
		},
	}

	err := SyncChannels(s.ctx, s.db, client, p)
	r.NoError(err)
}

func (s *SyncChannelsSuite) Test_SyncChannelsJob() {
	r := require.New(s.T())

	p := &SyncChannelsParams{
		BotUserID: "U1111111111",
		ChatRouletteConfig: config.ChatRouletteConfig{
			Interval: "weekly",
			Weekday:  "Monday",
			Hour:     12,
		},
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
