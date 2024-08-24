package bot

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bincyber/go-sqlcrypter"
	"github.com/bincyber/go-sqlcrypter/providers/aesgcm"
	"github.com/hashicorp/go-hclog"
	"github.com/sebdah/goldie/v2"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
)

func Test_notifyPairTemplate(t *testing.T) {
	g := goldie.New(t)

	testCases := []struct {
		name           string
		connectionMode string
		goldenFile     string
	}{
		{"virtual", models.VirtualConnectionMode.String(), "notify_pair_virtual.json"},
		{"physical", models.PhysicalConnectionMode.String(), "notify_pair_physical.json"},
		{"hybrid", models.HybridConnectionMode.String(), "notify_pair_hybrid.json"},
	}

	p := notifyPairTemplate{
		ChannelID: "C0123456789",
		Interval:  "biweekly",
		Participant: models.Member{
			UserID:       "U0123456789",
			Country:      sqlcrypter.NewEncryptedBytes("Kenya"),
			City:         sqlcrypter.NewEncryptedBytes("Nairobi"),
			ProfileType:  sqlcrypter.NewEncryptedBytes("Github"),
			ProfileLink:  sqlcrypter.NewEncryptedBytes("https://github.com/AhmedARmohamed"),
			CalendlyLink: sqlcrypter.NewEncryptedBytes("https://calendly.com/AhmedARmohamed"),
		},
		ParticipantTimezone: "EAT (UTC+03:00)",
		Partner: models.Member{
			UserID:      "U9876543210",
			Country:     sqlcrypter.NewEncryptedBytes("United States"),
			City:        sqlcrypter.NewEncryptedBytes("Phoenix"),
			ProfileType: sqlcrypter.NewEncryptedBytes("Github"),
			ProfileLink: sqlcrypter.NewEncryptedBytes("https://github.com/bincyber"),
		},
		PartnerTimezone: "MST (UTC-07:00)",
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p.ConnectionMode = tc.connectionMode

			content, err := renderTemplate(notifyPairTemplateFilename, p)
			assert.Nil(t, err)

			g.Assert(t, tc.goldenFile, []byte(content))
		})
	}
}

type NotifyPairSuite struct {
	suite.Suite
	ctx    context.Context
	mock   sqlmock.Sqlmock
	db     *gorm.DB
	logger hclog.Logger
	buffer *bytes.Buffer
}

func (s *NotifyPairSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)
	s.db, s.mock = database.NewMockedGormDB()
}

func (s *NotifyPairSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *NotifyPairSuite) Test_NotifyPair() {
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

	key, err := hex.DecodeString("fb7f69d3f824045c2685ad859593470df11e45256480802517cb20fc19b0d15e")
	r.NoError(err)

	aesCrypter, err := aesgcm.New(key, nil)
	r.NoError(err)

	sqlcrypter.Init(aesCrypter)

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

	// Write two members to the database
	isActive := true

	firstUserID := "U0123456789"
	db.Create(&models.Member{
		ChannelID: channelID,
		UserID:    firstUserID,
		IsActive:  &isActive,
		Country:   sqlcrypter.NewEncryptedBytes("Canada"),
		City:      sqlcrypter.NewEncryptedBytes("Toronto"),
	})

	secondUserID := "U5555666778"
	db.Create(&models.Member{
		ChannelID: channelID,
		UserID:    secondUserID,
		IsActive:  &isActive,
		Country:   sqlcrypter.NewEncryptedBytes("United Kingdom"),
		City:      sqlcrypter.NewEncryptedBytes("Manchester"),
	})

	// Write records in the rounds and matches table
	db.Create(&models.Round{
		ChannelID: channelID,
	})

	db.Create(&models.Match{
		RoundID: 1,
	})

	// Mock Slack API calls
	mux := http.NewServeMux()
	mux.HandleFunc("/conversations.open", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(`{"ok":true,"channel":{"id":"G1111111111", "is_mpim":true}}`))
	})

	mux.HandleFunc("/chat.postMessage", func(w http.ResponseWriter, req *http.Request) {
		req.ParseForm()

		b := req.FormValue("blocks")

		if b == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var blocks slack.Blocks
		if err := json.Unmarshal([]byte(b), &blocks); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"ok":false}`))
		}

		r.Len(blocks.BlockSet, 8)

		participantSection, ok := blocks.BlockSet[5].(*slack.SectionBlock)
		r.True(ok)
		r.Contains(participantSection.Text.Text, "*Name:* <@U0123456789>\n*Location:* Toronto, Canada")

		partnerSection, ok := blocks.BlockSet[7].(*slack.SectionBlock)
		r.True(ok)
		r.Contains(partnerSection.Text.Text, "*Name:* <@U5555666778>\n*Location:* Manchester, United Kingdom")

		w.Write([]byte(`{
			"ok": true,
			"channel": "D1111111111"
		}`))
	})

	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	url := fmt.Sprintf("%s/", httpServer.URL)
	client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(url))

	// Test
	err = NotifyPair(s.ctx, db, client, &NotifyPairParams{
		ChannelID:   channelID,
		MatchID:     1,
		Participant: firstUserID,
		Partner:     secondUserID,
	})
	r.NoError(err)
}

func (s *NotifyPairSuite) Test_QueueNotifyPairJob() {
	r := require.New(s.T())

	p := &NotifyPairParams{
		ChannelID:   "C0123456789",
		MatchID:     1,
		Participant: "U0111111111",
		Partner:     "U2022222222",
	}

	database.MockQueueJob(
		s.mock,
		p,
		models.JobTypeNotifyPair.String(),
		models.JobPriorityStandard,
	)

	err := QueueNotifyPairJob(s.ctx, s.db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")
}

func Test_NotifyPair_suite(t *testing.T) {
	suite.Run(t, new(NotifyPairSuite))
}
