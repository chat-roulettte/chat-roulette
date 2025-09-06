package v1

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/ory/dockertest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"github.com/chat-roulettte/chat-roulette/internal/bot"
	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/server"
)

type UpdateChannelHandlerSuite struct {
	suite.Suite
	resource *dockertest.Resource
	db       *gorm.DB
	router   *mux.Router
	response *httptest.ResponseRecorder
	store    *sessions.CookieStore
}

func (s *UpdateChannelHandlerSuite) SetupTest() {
	resource, databaseURL, err := database.NewTestPostgresDB(true)
	if err != nil {
		log.Fatal(err)
	}
	s.resource = resource

	db, err := database.NewGormDB(databaseURL)
	if err != nil {
		log.Fatal(err)
	}

	// Write channel to the database
	db.Create(&models.Channel{
		ChannelID:      "C0123456789",
		Inviter:        "U9876543210",
		ConnectionMode: models.ConnectionModePhysical,
		Interval:       models.Biweekly,
		Weekday:        time.Friday,
		Hour:           12,
		NextRound:      time.Now().Add(24 * time.Hour),
	})

	s.db = db

	key, _ := hex.DecodeString("8c4faf836e29d282f2dc7ffdf4ef59c6081e2d8964ba0ac9cd4bc8800021300c")

	s.store = sessions.NewCookieStore(key)

	opts := &server.ServerOptions{
		SessionsStore: s.store,
		DB:            db,
	}

	srv := &implServer{server.NewTestServer(opts)}

	s.router = mux.NewRouter()
	s.router.HandleFunc("/v1/channel", srv.updateChannelHandler).Methods(http.MethodPost)

	s.response = httptest.NewRecorder()
}

func (s *UpdateChannelHandlerSuite) AfterTest(_, _ string) {
	s.resource.Close()
}

func (s *UpdateChannelHandlerSuite) Test_Unauthenticated() {
	r := require.New(s.T())

	request, _ := http.NewRequest(http.MethodPost, "/v1/channel", nil)

	s.router.ServeHTTP(s.response, request)

	r.Equal(http.StatusUnauthorized, s.response.Code)
}

func (s *UpdateChannelHandlerSuite) Test_Validation() {
	r := require.New(s.T())

	p := &bot.UpdateChannelParams{
		ChannelID:      "C0123456789",
		Interval:       "on the regular",
		ConnectionMode: "in-person",
		Weekday:        "Thursday",
		Hour:           24,
		NextRound:      time.Now().UTC().AddDate(0, 0, -2),
	}

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(p)

	request, _ := http.NewRequest(http.MethodPost, "/v1/channel", body)

	session, err := s.store.Get(request, server.SessionKey)
	r.NoError(err)
	session.Values["authenticated"] = true
	session.Save(request, s.response)

	s.router.ServeHTTP(s.response, request)

	r.Equal(http.StatusBadRequest, s.response.Code)
	r.Contains(s.response.Body.String(), "validation failed")
}

func (s *UpdateChannelHandlerSuite) Test_Unauthorized() {
	r := require.New(s.T())

	p := &bot.UpdateChannelParams{
		ChannelID: "C0123456789",
		Interval:  "weekly",
		Weekday:   "Thursday",
		Hour:      12,
		NextRound: time.Now().UTC(),
	}

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(p)

	request, _ := http.NewRequest(http.MethodPost, "/v1/channel", body)

	session, err := s.store.Get(request, server.SessionKey)
	r.NoError(err)
	session.Values["authenticated"] = true
	session.Values["slack_user_id"] = "U1111222233"
	session.Save(request, s.response)

	s.router.ServeHTTP(s.response, request)

	r.Equal(http.StatusForbidden, s.response.Code)
	r.Contains(s.response.Body.String(), "authorization failed")
}

func (s *UpdateChannelHandlerSuite) Test_Success() {
	r := require.New(s.T())

	p := &bot.UpdateChannelParams{
		ChannelID: "C0123456789",
		Interval:  "monthly",
		Weekday:   "Monday",
		Hour:      12,
		NextRound: time.Now().UTC().AddDate(0, 0, -1),
	}

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(p)

	request, _ := http.NewRequest(http.MethodPost, "/v1/channel", body)

	session, err := s.store.Get(request, server.SessionKey)
	r.NoError(err)
	session.Values["authenticated"] = true
	session.Values["slack_user_id"] = "U9876543210"
	session.Save(request, s.response)

	s.router.ServeHTTP(s.response, request)

	r.Equal(http.StatusAccepted, s.response.Code)

	var count int64
	result := s.db.Model(&models.Job{}).Where("job_type = ?", models.JobTypeUpdateChannel).Count(&count)
	r.NoError(result.Error)
	r.Equal(int64(1), count)
}

func Test_updateChannelHandler_suite(t *testing.T) {
	suite.Run(t, new(UpdateChannelHandlerSuite))
}
