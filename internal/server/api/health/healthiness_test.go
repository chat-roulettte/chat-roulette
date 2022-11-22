package health

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getHealthiness(t *testing.T) {
	s := &implServer{}

	path := "/healthy"
	req, _ := http.NewRequest(http.MethodGet, path, nil)
	resp := httptest.NewRecorder()

	server := http.NewServeMux()
	server.Handle(path, http.HandlerFunc(s.healthHandler))

	server.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "ok", resp.Body.String())
}
