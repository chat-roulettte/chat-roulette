package bot

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

type NotifyMemberSuite struct {
	suite.Suite
	ctx    context.Context
	mock   sqlmock.Sqlmock
	db     *gorm.DB
	logger hclog.Logger
	buffer *bytes.Buffer
}

func (s *NotifyMemberSuite) SetupTest() {
	s.logger, s.buffer = o11y.NewBufferedLogger()
	s.ctx = hclog.WithContext(context.Background(), s.logger)
	s.db, s.mock = database.NewMockedGormDB()
}

func (s *NotifyMemberSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *NotifyMemberSuite) Test_NotifyMember() {
	r := require.New(s.T())

	p := &NotifyMemberParams{
		ChannelID: "C9876543210",
		UserID:    "U0123456789",
	}

	columns := []string{
		"channel_id",
		"inviter",
		"interval",
		"weekday",
		"hour",
		"next_round",
	}

	row := []driver.Value{
		p.ChannelID,
		"U8967452301",
		models.Weekly,
		time.Sunday,
		12,
		time.Now(),
	}

	// Mock retrieving channel metadata
	s.mock.ExpectQuery(`SELECT \* FROM "channels" WHERE channel_id = (.+) ORDER BY`).
		WithArgs(
			p.ChannelID,
			1,
		).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(row...))

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

		r.Len(blocks.BlockSet, 4)

		w.Write([]byte(`{
			"ok": true,
			"channel": "D1111111111"
		}`))
	})

	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	url := fmt.Sprintf("%s/", httpServer.URL)
	client := slack.New("xoxb-test-token-here", slack.OptionAPIURL(url))

	err := NotifyMember(s.ctx, s.db, client, p)
	r.NoError(err)
}

func (s *NotifyMemberSuite) Test_NotifyMemberJob() {
	r := require.New(s.T())

	p := &NotifyMemberParams{
		ChannelID: "C0123456789",
		UserID:    "U1111111111",
	}

	database.MockQueueJob(
		s.mock,
		p,
		models.JobTypeNotifyMember.String(),
		models.JobPriorityStandard,
	)

	err := QueueNotifyMemberJob(s.ctx, s.db, p)
	r.NoError(err)
	r.Contains(s.buffer.String(), "added new job to the database")
}

func Test_NotifyMember_suite(t *testing.T) {
	suite.Run(t, new(NotifyMemberSuite))
}

func Test_notifyMemberTemplate(t *testing.T) {
	g := goldie.New(t)

	nextRound := time.Date(2022, time.January, 3, 12, 0, 0, 0, time.UTC)

	p := notifyMemberTemplate{
		ChannelID: "C0123456789",
		UserID:    "U0123456789",
		NextRound: nextRound,
	}

	content, err := renderTemplate(notifyMemberTemplateFilename, p)
	assert.Nil(t, err)

	g.Assert(t, "notify_member.json", []byte(content))
}
