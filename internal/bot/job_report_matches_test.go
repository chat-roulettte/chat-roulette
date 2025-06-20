package bot

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
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

func Test_reportMatchesTemplate(t *testing.T) {
	g := goldie.New(t)

	data := reportMatchesTemplate{
		ChannelID: "C9876543210",
		UserID:    "U9876543210",
		NextRound: time.Date(2022, time.January, 3, 12, 0, 0, 0, time.UTC),
	}

	t.Run("admin", func(t *testing.T) {
		data.IsAdmin = true
		data.Participants = 51
		data.Men = 20
		data.Women = 30
		data.HasGenderPreference = 6
		data.Unpaired = 1
		data.Pairs = 25

		content, err := renderTemplate(reportMatchesTemplateFilename, data)
		assert.Nil(t, err)

		g.Assert(t, "report_matches_admin.json", []byte(content))
	})

	t.Run("admin zero participants", func(t *testing.T) {
		data.IsAdmin = true
		data.Participants = 0
		data.Men = 0
		data.Women = 0
		data.HasGenderPreference = 0
		data.Unpaired = 0
		data.Pairs = 0

		content, err := renderTemplate(reportMatchesTemplateFilename, data)
		assert.Nil(t, err)

		g.Assert(t, "report_matches_admin_zero.json", []byte(content))
	})

	t.Run("channel", func(t *testing.T) {
		data.IsAdmin = false
		data.Participants = 13
		data.Pairs = 6

		content, err := renderTemplate(reportMatchesTemplateFilename, data)
		assert.Nil(t, err)

		g.Assert(t, "report_matches_channel.json", []byte(content))
	})

	t.Run("channel no matches", func(t *testing.T) {
		data.IsAdmin = false
		data.Participants = 1
		data.Pairs = 0

		content, err := renderTemplate(reportMatchesTemplateFilename, data)
		assert.Nil(t, err)

		g.Assert(t, "report_matches_channel_zero.json", []byte(content))
	})
}

type ReportMatchesSuite struct {
	suite.Suite
	ctx    context.Context
	mock   sqlmock.Sqlmock
	db     *gorm.DB
	logger hclog.Logger
	buffer *bytes.Buffer
}

func (s *ReportMatchesSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)
	s.db, s.mock = database.NewMockedGormDB()
}

func (s *ReportMatchesSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *ReportMatchesSuite) Test_ReportMatches() {
	r := require.New(s.T())

	channelID := "C030DJ523N3"
	adminID := "U8967452301"
	roundID := 7

	// Mock Slack API calls
	mux := http.NewServeMux()
	mux.HandleFunc("/conversations.open", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(`{"ok":true,"channel":{"id":"D1111111111"}}`))
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

		section, ok := blocks.BlockSet[0].(*slack.SectionBlock)
		r.True(ok)

		switch {
		case strings.Contains(section.Text.Text, "Hi all :wave:"):
			r.Len(blocks.BlockSet, 8)
		case strings.Contains(section.Text.Text, "Hi <@U9876543210> :wave:"):
			r.Len(blocks.BlockSet, 10)
		}

		w.Write([]byte(`{
			"ok": true
		}`))
	})

	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	url := fmt.Sprintf("%s/", httpServer.URL)
	client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(url))

	// Mock channel lookup
	columns := []string{
		"channel_id",
		"inviter",
		"interval",
		"weekday",
		"hour",
		"next_round",
	}

	row := []driver.Value{
		channelID,
		adminID,
		models.Weekly,
		time.Sunday,
		12,
		time.Now(),
	}

	// Mock channel lookup
	s.mock.ExpectQuery(`SELECT \* FROM "channels" WHERE channel_id = (.+) ORDER BY`).
		WithArgs(
			channelID,
			1,
		).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(row...))

	// Mock stat lookups
	s.mock.ExpectQuery(`SELECT .* FROM "pairings" JOIN .* WHERE matches.round_id = (.+)`).
		WithArgs(
			roundID,
		).
		WillReturnRows(sqlmock.NewRows([]string{"men", "women", "has_gender_preference"}).AddRow([]driver.Value{5, 5, 0}...))

	p := &ReportMatchesParams{
		ChannelID:    channelID,
		RoundID:      int32(roundID),
		Participants: 10,
		Pairs:        5,
		Unpaired:     0,
	}

	err := ReportMatches(s.ctx, s.db, client, p)
	r.NoError(err)

}

func Test_ReportMatches_suite(t *testing.T) {
	suite.Run(t, new(ReportMatchesSuite))
}
