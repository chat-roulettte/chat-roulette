package v1

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/trace"

	"github.com/chat-roulettte/chat-roulette/internal/tzx"
)

type timezonesResponse struct {
	Zones []string
}

// timezonesHandler returns the timezones for the specified country
//
// HTTP Method: GET
//
// HTTP Path: /timezones/{country}
func (s *implServer) timezonesHandler(w http.ResponseWriter, r *http.Request) {
	span := trace.SpanFromContext(r.Context())

	// Verify that the user is authenticated
	session, err := s.GetSession(r)
	if err != nil {
		span.RecordError(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Lookup the zones for the specified country
	value := mux.Vars(r)["country"]

	country, ok := tzx.GetCountryByName(value)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Build the response
	response := &timezonesResponse{}

	for _, zone := range country.Zones {
		response.Zones = append(response.Zones, zone.Name)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response) //nolint:errcheck
}
