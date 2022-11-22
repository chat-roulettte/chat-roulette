package health

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slacktest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/server"
)

const readinessPath = "/ready"

type ReadinessTestSuite struct {
	suite.Suite
	mock sqlmock.Sqlmock

	Request    *http.Request
	Response   *httptest.ResponseRecorder
	HTTPServer *http.ServeMux
}

func (s *ReadinessTestSuite) SetupTest() {
	s.Request, _ = http.NewRequest(http.MethodGet, readinessPath, nil)
	s.Response = httptest.NewRecorder()
	s.HTTPServer = http.NewServeMux()
}

func (s *ReadinessTestSuite) AfterTest(_, _ string) {
	require.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *ReadinessTestSuite) Test_readinessHandler_Success() {
	r := require.New(s.T())

	slackServer := slacktest.NewTestServer()
	go slackServer.Start()
	defer slackServer.Stop()

	slackClient := slack.New("xoxb-test-token-here", slack.OptionAPIURL(slackServer.GetAPIURL()))

	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	r.NoError(err)
	s.mock = mock
	mock.ExpectPing()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{
		DisableAutomaticPing: true,
	})
	r.NoError(err)

	opts := &server.ServerOptions{
		DevMode:     true,
		DB:          gormDB,
		SlackClient: slackClient,
	}

	srv := &implServer{server.NewTestServer(opts)}

	s.HTTPServer.Handle(readinessPath, http.HandlerFunc(srv.readinessHandler))
	s.HTTPServer.ServeHTTP(s.Response, s.Request)

	r.Equal(http.StatusOK, s.Response.Code)
	r.Equal("ready", s.Response.Body.String())
}

func (s *ReadinessTestSuite) Test_readinessHandler_all_err() {
	r := require.New(s.T())

	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	r.NoError(err)
	s.mock = mock
	mock.ExpectPing().WillReturnError(fmt.Errorf("connection error"))

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{
		DisableAutomaticPing: true,
	})
	r.NoError(err)

	opts := &server.ServerOptions{
		DevMode:     true,
		DB:          gormDB,
		SlackClient: slack.New("invalid-token"),
	}

	srv := &implServer{server.NewTestServer(opts)}

	s.HTTPServer.Handle(readinessPath, http.HandlerFunc(srv.readinessHandler))
	s.HTTPServer.ServeHTTP(s.Response, s.Request)

	r.Equal(http.StatusServiceUnavailable, s.Response.Code)
	r.Equal("not ready", s.Response.Body.String())
}

func (s *ReadinessTestSuite) Test_readinessHandler_slack_err() {
	r := require.New(s.T())

	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	r.NoError(err)
	s.mock = mock
	mock.ExpectPing()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{
		DisableAutomaticPing: true,
	})
	r.NoError(err)

	opts := &server.ServerOptions{
		DevMode:     true,
		DB:          gormDB,
		SlackClient: slack.New("xoxb-invalid-slack-authtoken"),
	}

	srv := &implServer{server.NewTestServer(opts)}

	s.HTTPServer.Handle(readinessPath, http.HandlerFunc(srv.readinessHandler))
	s.HTTPServer.ServeHTTP(s.Response, s.Request)

	r.Equal(http.StatusServiceUnavailable, s.Response.Code)
	r.Equal("not ready", s.Response.Body.String())
}

func TestReadiness_suite(t *testing.T) {
	suite.Run(t, new(ReadinessTestSuite))
}
