package api

import (
	"net/http"
	"path"

	"github.com/gorilla/mux"
)

// Route represents a HTTP route
type Route struct {
	Path    string
	Methods []string
	Func    func(w http.ResponseWriter, r *http.Request)
}

// RegisterRoutes dynamically registers routes on the mux with the given prefix
func RegisterRoutes(m *mux.Router, prefix string, routes []Route) {
	for _, r := range routes {
		p := path.Join(prefix, r.Path)
		m.HandleFunc(p, r.Func).Methods(r.Methods...)
	}
}
