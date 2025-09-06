package v1

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bincyber/go-sqlcrypter"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/require"

	"github.com/chat-roulettte/chat-roulette/internal/bot"
	"github.com/chat-roulettte/chat-roulette/internal/database"
	"github.com/chat-roulettte/chat-roulette/internal/database/models"
	"github.com/chat-roulettte/chat-roulette/internal/server"
)

func Test_updateMemberHandler(t *testing.T) {

	key, _ := hex.DecodeString("8c4faf836e29d282f2dc7ffdf4ef59c6081e2d8964ba0ac9cd4bc8800021300c")

	store := sessions.NewCookieStore(key)

	sqlcrypter.Init(database.NoOpCrypter{})

	db, mock := database.NewMockedGormDB()

	opts := &server.ServerOptions{
		SessionsStore: store,
		DB:            db,
	}

	srv := server.NewTestServer(opts)
	s := &implServer{srv}

	method := http.MethodPost
	path := "/v1/member"

	router := mux.NewRouter()
	router.HandleFunc(path, s.updateMemberHandler).Methods(method)

	isActive := false

	params := updateMemberRequest{
		ChannelID:      "C9876543210",
		UserID:         "U0123456789",
		ConnectionMode: models.ConnectionModePhysical.String(),
		IsActive:       &isActive,
		Country:        "United States of America",
		City:           "Phoenix",
		Timezone:       "America/Phoenix",
		ProfileType:    "Twitter",
		ProfileLink:    "twitter.com/test",
	}

	t.Run("unauthenticated", func(t *testing.T) {
		r := require.New(t)

		resp := httptest.NewRecorder()
		req, _ := http.NewRequest(method, path, nil)

		router.ServeHTTP(resp, req)

		r.Equal(http.StatusUnauthorized, resp.Code)
	})

	t.Run("unauthorized", func(t *testing.T) {
		r := require.New(t)

		body := new(bytes.Buffer)
		json.NewEncoder(body).Encode(params)

		resp := httptest.NewRecorder()
		req, _ := http.NewRequest(method, path, body)

		session, err := store.Get(req, server.SessionKey)
		r.NoError(err)
		session.Values["authenticated"] = true
		session.Values["slack_user_id"] = "U1111222233"
		session.Save(req, resp)

		router.ServeHTTP(resp, req)

		r.Equal(http.StatusForbidden, resp.Code)
		r.Contains(resp.Body.String(), "authorization failed")
	})

	t.Run("validation", func(t *testing.T) {
		r := require.New(t)

		isActive := true

		p := &bot.UpdateMemberParams{
			ChannelID:      "C9876543210",
			UserID:         "U0123456789",
			IsActive:       &isActive,
			ConnectionMode: "unknown",
		}

		body := new(bytes.Buffer)
		json.NewEncoder(body).Encode(p)

		resp := httptest.NewRecorder()
		req, _ := http.NewRequest(method, path, body)

		session, err := store.Get(req, server.SessionKey)
		r.NoError(err)
		session.Values["authenticated"] = true
		session.Save(req, resp)

		router.ServeHTTP(resp, req)

		r.Equal(http.StatusBadRequest, resp.Code)
		r.Contains(resp.Body.String(), "Validation failed")
	})

	t.Run("success", func(t *testing.T) {
		r := require.New(t)

		body := new(bytes.Buffer)
		json.NewEncoder(body).Encode(params)

		resp := httptest.NewRecorder()
		req, _ := http.NewRequest(method, path, body)

		session, err := store.Get(req, "GOSESSION")
		r.NoError(err)
		session.Values["authenticated"] = true
		session.Values["slack_user_id"] = params.UserID
		session.Save(req, resp)

		params.Country = "VW5pdGVkIFN0YXRlcyBvZiBBbWVyaWNh"
		params.City = "UGhvZW5peA=="
		params.Timezone = "QW1lcmljYS9QaG9lbml4"
		params.ProfileType = "VHdpdHRlcg=="
		params.ProfileLink = "dHdpdHRlci5jb20vdGVzdA=="

		database.MockQueueJob(
			mock,
			params,
			models.JobTypeUpdateMember.String(),
			models.JobPriorityHigh,
		)

		router.ServeHTTP(resp, req)

		r.Equal(http.StatusAccepted, resp.Code)
		r.NoError(mock.ExpectationsWereMet())
	})
}
