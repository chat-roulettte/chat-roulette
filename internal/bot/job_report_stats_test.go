package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hashicorp/go-hclog"
	"github.com/sebdah/goldie/v2"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/o11y"
)

type ReportStatsSuite struct {
	suite.Suite
	ctx    context.Context
	mock   sqlmock.Sqlmock
	db     *gorm.DB
	logger hclog.Logger
	buffer *bytes.Buffer
}

func (s *ReportStatsSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)
	s.db, s.mock = database.NewMockedGormDB()
}

func (s *ReportStatsSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *ReportStatsSuite) Test_ReportStats() {
	r := require.New(s.T())

	channelID := "C030DJ523N3"
	roundID := 5

	p := &ReportStatsParams{
		ChannelID: channelID,
		RoundID:   int32(roundID),
	}

	// httptest.NewServer() is used instead of slacktest.NewTestServer() because
	// the latter fails to handle '%' characters in its test chat.postMessage handler
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
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

		r.Len(blocks.BlockSet, 9)

		w.Write([]byte(`{
			"ok": true,
			"channel": "C030DJ523N3"
		}`))
	}))
	defer httpServer.Close()

	url := fmt.Sprintf("%s/", httpServer.URL)
	client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(url))

	// Mock DB call
	s.mock.ExpectQuery(`SELECT .* FROM "matches" WHERE round_id = (.+)`).
		WithArgs(
			p.RoundID,
		).
		WillReturnRows(sqlmock.NewRows([]string{"total", "met"}).FromCSVString("10,5"))

	err := ReportStats(s.ctx, s.db, client, p)
	r.NoError(err)
}

func Test_ReportStats_suite(t *testing.T) {
	suite.Run(t, new(ReportStatsSuite))
}

func Test_reportStatsTemplate(t *testing.T) {

	testCases := []struct {
		name     string
		pairs    float64
		met      float64
		percent  float64
		contains []string
	}{
		{"all met", 10, 10, 100, []string{
			"Congratulations to everyone for achieving *100%* :tada:",
		}},
		{"half met", 10, 5, 50, []string{
			"This round had *20* participants",
			"*5* groups met :partying_face:",
			"*50%* of the *10* intros made :confetti_ball:",
		}},
		{"one group met", 4, 1, 25, []string{
			"*1* group met",
			"*25%* of the *4* intros made",
			"Can you get to *100%* next round?",
		}},
		{"none met", 20, 0, 0, []string{
			"This round had *40* participants",
			"*0* groups met :smiling_face_with_tear:",
			"*0%* of the *20* intros made",
		}},
		{"no pairs", 0, 0, 0, []string{
			"No intros were made in the last round :sob:",
			"To ensure intros can be made in the next round, you must opt-in to Chat Roulette.",
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			p := reportStatsTemplate{
				Pairs:   tc.pairs,
				Met:     tc.met,
				Percent: tc.percent,
			}

			content, err := renderTemplate(reportStatsTemplateFilename, p)
			require.NoError(t, err)

			if tc.name == "all met" {
				goldie.New(t).Assert(t, strings.TrimRight(reportStatsTemplateFilename, ".tmpl"), []byte(content))
			}

			for _, substring := range tc.contains {
				assert.Contains(t, content, substring)
			}
		})
	}
}
