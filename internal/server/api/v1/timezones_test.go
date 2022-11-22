package v1

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/require"

	"github.com/chat-roulettte/chat-roulette/internal/server"
)

func Test_timezonesHandler(t *testing.T) {

	key, _ := hex.DecodeString("8c4faf836e29d282f2dc7ffdf4ef59c6081e2d8964ba0ac9cd4bc8800021300c")

	store := sessions.NewCookieStore(key)

	opts := &server.ServerOptions{
		SessionsStore: store,
	}

	srv := server.NewTestServer(opts)
	s := &implServer{srv}

	method := http.MethodGet

	server := mux.NewRouter()
	server.HandleFunc("/v1/timezones/{country}", s.timezonesHandler).Methods(method)

	t.Run("unauthenticated", func(t *testing.T) {
		r := require.New(t)

		resp := httptest.NewRecorder()
		req, _ := http.NewRequest(method, "/v1/timezones/Canada", nil)

		server.ServeHTTP(resp, req)

		r.Equal(http.StatusUnauthorized, resp.Code)
	})

	t.Run("success", func(t *testing.T) {
		r := require.New(t)

		resp := httptest.NewRecorder()
		req, _ := http.NewRequest(method, "/v1/timezones/Canada", nil)

		session, err := store.Get(req, "GOSESSION")
		r.NoError(err)
		session.Values["authenticated"] = true
		session.Save(req, resp)

		server.ServeHTTP(resp, req)

		r.Equal(http.StatusOK, resp.Code)
		r.Contains(resp.Body.String(), "America/Toronto")

		var contents timezonesResponse
		json.NewDecoder(resp.Body).Decode(&contents)

		r.Len(contents.Zones, 28)
	})
}
