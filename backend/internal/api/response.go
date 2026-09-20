package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// Source is an attribution notice. Both licences require one, and a map that
// carries it only in a footer somewhere loses it the moment somebody uses the
// API directly — so every response that carries data carries its sources.
type Source struct {
	Name    string `json:"name"`
	Licence string `json:"licence"`
	URL     string `json:"url"`
}

var (
	sourceAccidents = Source{
		Name:    "Unfallatlas der Statistischen Ämter des Bundes und der Länder",
		Licence: "dl-de/by-2-0",
		URL:     "https://unfallatlas.statistikportal.de/",
	}
	sourceOSM = Source{
		Name:    "© OpenStreetMap-Mitwirkende",
		Licence: "ODbL",
		URL:     "https://www.openstreetmap.org/copyright",
	}
)

type errorBody struct {
	Error     string `json:"error"`
	Parameter string `json:"parameter,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("write response", "error", err)
	}
}

// fail turns an error into the right status. A bad parameter is the caller's
// to fix and says so; anything else is ours, is logged in full, and is not
// leaked to the caller.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	var bad *badRequest
	if errors.As(err, &bad) {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: bad.Reason, Parameter: bad.Parameter})
		return
	}
	if errors.Is(err, errNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "not found"})
		return
	}
	slog.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	writeJSON(w, http.StatusInternalServerError, errorBody{Error: "internal error"})
}

var errNotFound = errors.New("not found")
