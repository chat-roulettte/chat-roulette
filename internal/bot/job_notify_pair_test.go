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
	"gorm.io/datatypes"
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
		{"virtual", models.ConnectionModeVirtual.String(), "notify_pair_virtual.json"},
		{"physical", models.ConnectionModePhysical.String(), "notify_pair_physical.json"},
		{"hybrid", models.ConnectionModeHybrid.String(), "notify_pair_hybrid.json"},
	}

	p := notifyPairTemplate{
		ChannelID: "C0123456789",
		Interval:  "biweekly",
		Participant: models.Member{
			UserID:       "U0123456789",
			Country:      sqlcrypter.NewEncryptedBytes("Kenya"),
			City:         sqlcrypter.NewEncryptedBytes("Nairobi"),
			ProfileType:  sqlcrypter.NewEncryptedBytes("GitHub"),
			ProfileLink:  sqlcrypter.NewEncryptedBytes("https://github.com/AhmedARmohamed"),
			CalendlyLink: sqlcrypter.NewEncryptedBytes("https://calendly.com/AhmedARmohamed"),
		},
		ParticipantTimezone: "EAT (UTC+03:00)",
		Partner: models.Member{
			UserID:      "U9876543210",
			Country:     sqlcrypter.NewEncryptedBytes("United States"),
			City:        sqlcrypter.NewEncryptedBytes("Phoenix"),
			ProfileType: sqlcrypter.NewEncryptedBytes("GitHub"),
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
		ConnectionMode: models.ConnectionModePhysical,
		Interval:       models.Biweekly,
		Weekday:        time.Friday,
		Hour:           12,
		NextRound:      time.Now().Add(168 * time.Hour),
	})

	// Write two members to the database
	isActive := true

	firstUserID := "U0123456789"
	db.Create(&models.Member{
		ChannelID:           channelID,
		UserID:              firstUserID,
		Gender:              models.Male,
		IsActive:            &isActive,
		HasGenderPreference: new(bool),
		Country:             sqlcrypter.NewEncryptedBytes("Canada"),
		City:                sqlcrypter.NewEncryptedBytes("Toronto"),
		ProfileType:         sqlcrypter.NewEncryptedBytes("GitHub"),
		ProfileLink:         sqlcrypter.NewEncryptedBytes("github.com/user1"),
		CalendlyLink:        sqlcrypter.NewEncryptedBytes("https://calendly.com/example"),
	})

	secondUserID := "U5555666778"
	db.Create(&models.Member{
		ChannelID:           channelID,
		UserID:              secondUserID,
		Gender:              models.Female,
		IsActive:            &isActive,
		HasGenderPreference: new(bool),
		Country:             sqlcrypter.NewEncryptedBytes("United Kingdom"),
		City:                sqlcrypter.NewEncryptedBytes("Manchester"),
		ProfileType:         sqlcrypter.NewEncryptedBytes("Twitter"),
		ProfileLink:         sqlcrypter.NewEncryptedBytes("twitter.com/user2"),
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

		r.Len(blocks.BlockSet, 12)

		// Verify
		participantSection, ok := blocks.BlockSet[4].(*slack.SectionBlock)
		r.True(ok)
		r.Contains(participantSection.Text.Text, fmt.Sprintf(":identification_card: *Name:* <@%s>", firstUserID))

		partnerSection, ok := blocks.BlockSet[8].(*slack.SectionBlock)
		r.True(ok)
		r.Contains(partnerSection.Text.Text, fmt.Sprintf(":identification_card: *Name:* <@%s>", secondUserID))

		w.Write([]byte(`{
			"ok": true,
			"channel": "D1111111111"
		}`))
	})

	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	url := fmt.Sprintf("%s/", httpServer.URL)
	client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(url))

	p := &NotifyPairParams{
		ChannelID:   channelID,
		MatchID:     1,
		Participant: firstUserID,
		Partner:     secondUserID,
	}

	err = NotifyPair(s.ctx, db, client, p)
	r.NoError(err)

	var checkPairJobs []models.Job
	result := db.Model(&models.Job{}).
		Where("job_type = ?", models.JobTypeCheckPair).
		Where(datatypes.JSONQuery("data").Equals(p.MatchID, "match_id")).
		Where(datatypes.JSONQuery("data").Equals(p.Participant, "participant")).
		Where(datatypes.JSONQuery("data").Equals(p.Partner, "partner")).
		Find(&checkPairJobs)
	r.NoError(result.Error)
	r.Len(checkPairJobs, 2)
	r.Greater(checkPairJobs[1].ExecAt, checkPairJobs[0].ExecAt)
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
