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

		r.Len(blocks.BlockSet, 5)

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
	t.Run("all met", func(t *testing.T) {
		g := goldie.New(t)

		p := reportStatsTemplate{
			Matches: 10,
			Met:     10,
			Percent: 100,
		}

		content, err := renderTemplate(reportStatsTemplateFilename, p)
		require.NoError(t, err)

		g.Assert(t, strings.TrimRight(reportStatsTemplateFilename, ".tmpl"), []byte(content))
	})

	t.Run("some met", func(t *testing.T) {
		p := reportStatsTemplate{
			Matches: 10,
			Met:     5,
			Percent: 50,
		}

		content, err := renderTemplate(reportStatsTemplateFilename, p)
		require.NoError(t, err)

		assert.Contains(t, content, "*5* groups met")
		assert.Contains(t, content, "*50%* of the *10* intros made")
	})

	t.Run("no matches", func(t *testing.T) {
		p := reportStatsTemplate{
			Matches: 0,
			Met:     0,
			Percent: 0,
		}

		content, err := renderTemplate(reportStatsTemplateFilename, p)
		require.NoError(t, err)

		assert.Contains(t, content, "No matches were made in the last round of Chat Roulette! :cry:")
	})
}
