package health

import "net/http"

// healthHandler is used to check if the application is healthy
//
// HTTP Method: GET
//
// HTTP Path: /ready
func (s *implServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
