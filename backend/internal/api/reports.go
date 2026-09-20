package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/reports"
)

type Report struct {
	ID            string    `json:"id"`
	Lon           float64   `json:"lon"`
	Lat           float64   `json:"lat"`
	Category      string    `json:"category"`
	Description   string    `json:"description"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
	Confirmations int       `json:"confirmations"`
}

type reportList struct {
	Reports    []Report `json:"reports"`
	Categories []string `json:"categories"`
}

func asReport(r reports.Report) Report {
	return Report{
		ID: r.ID.String(), Lon: r.Lon, Lat: r.Lat,
		Category: r.Category, Description: r.Description,
		Status: string(r.Status), CreatedAt: r.CreatedAt, Confirmations: r.Confirmations,
	}
}

func asReports(items []reports.Report) []Report {
	out := make([]Report, 0, len(items))
	for _, item := range items {
		out = append(out, asReport(item))
	}
	return out
}

// listReports serves the public map: approved reports only.
func (s *Server) listReports(w http.ResponseWriter, r *http.Request) {
	if s.reports == nil {
		fail(w, r, errReportsDisabled)
		return
	}
	box, err := parseBBox(query(r, "bbox"))
	if err != nil {
		fail(w, r, err)
		return
	}
	items, err := s.reports.ListApproved(r.Context(), box)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, reportList{Reports: asReports(items), Categories: reports.Categories})
}

func (s *Server) institutionReports(w http.ResponseWriter, r *http.Request) {
	if s.reports == nil {
		fail(w, r, errReportsDisabled)
		return
	}
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	radius, err := parseRadius(query(r, "radius"))
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := s.requireInstitution(r.Context(), id); err != nil {
		fail(w, r, err)
		return
	}
	items, err := s.reports.ListNear(r.Context(), id, radius)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, reportList{Reports: asReports(items), Categories: reports.Categories})
}

type submissionBody struct {
	Lon         float64 `json:"lon"`
	Lat         float64 `json:"lat"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
}

func (s *Server) createReport(w http.ResponseWriter, r *http.Request) {
	if s.reports == nil {
		fail(w, r, errReportsDisabled)
		return
	}

	var body submissionBody
	// A body larger than this is not a report; reading it in full would only
	// give an unauthenticated caller a way to spend memory.
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&body); err != nil {
		fail(w, r, invalid("body", "expected JSON with lon, lat, category and description"))
		return
	}

	if !reports.ValidCategory(body.Category) {
		fail(w, r, invalid("category", "%q is not one of %s", body.Category, strings.Join(reports.Categories, ", ")))
		return
	}
	description := strings.TrimSpace(body.Description)
	if len([]rune(description)) > reports.MaxDescription {
		fail(w, r, invalid("description", "longer than %d characters", reports.MaxDescription))
		return
	}
	if err := checkInGermany(body.Lon, body.Lat); err != nil {
		fail(w, r, err)
		return
	}

	report, err := s.reports.Create(r.Context(), reports.Submission{
		Lon: body.Lon, Lat: body.Lat, Category: body.Category, Description: description,
	}, s.reports.Fingerprint(r.RemoteAddr))

	if errors.Is(err, reports.ErrRateLimited) {
		writeJSON(w, http.StatusTooManyRequests, errorBody{
			Error: "Es wurden bereits mehrere Meldungen von diesem Anschluss abgeschickt. Bitte am nächsten Tag erneut versuchen.",
		})
		return
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, asReport(*report))
}

func (s *Server) confirmReport(w http.ResponseWriter, r *http.Request) {
	if s.reports == nil {
		fail(w, r, errReportsDisabled)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		fail(w, r, invalid("id", "%q is not a report id", chi.URLParam(r, "id")))
		return
	}

	err = s.reports.Confirm(r.Context(), id, s.reports.Fingerprint(r.RemoteAddr))
	switch {
	case errors.Is(err, reports.ErrNotFound):
		fail(w, r, errNotFound)
	case errors.Is(err, reports.ErrDuplicate):
		// Confirming twice is not an error worth an error page; the state the
		// caller wanted is the state that holds.
		writeJSON(w, http.StatusOK, map[string]string{"status": "already confirmed"})
	case err != nil:
		fail(w, r, err)
	default:
		writeJSON(w, http.StatusCreated, map[string]string{"status": "confirmed"})
	}
}

// moderationOnly guards the queue with a shared token from the environment.
func (s *Server) moderationOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.options.ModerationToken == "" {
			writeJSON(w, http.StatusServiceUnavailable, errorBody{
				Error: "Moderation ist nicht konfiguriert (MODERATION_TOKEN fehlt).",
			})
			return
		}
		presented := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		// Constant time, so a wrong token cannot be found one character at a
		// time by measuring how long the answer takes.
		if subtle.ConstantTimeCompare([]byte(presented), []byte(s.options.ModerationToken)) != 1 {
			writeJSON(w, http.StatusUnauthorized, errorBody{Error: "Kein gültiges Moderations-Token."})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) moderationQueue(w http.ResponseWriter, r *http.Request) {
	status := reports.Status(query(r, "status"))
	if status == "" {
		status = reports.StatusPending
	}
	if status != reports.StatusPending && status != reports.StatusApproved && status != reports.StatusRejected {
		fail(w, r, invalid("status", "%q is not a status", status))
		return
	}
	limit, err := parseLimit(query(r, "limit"), 100, 500)
	if err != nil {
		fail(w, r, err)
		return
	}
	items, err := s.reports.ListByStatus(r.Context(), status, limit)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, reportList{Reports: asReports(items), Categories: reports.Categories})
}

type decisionBody struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

func (s *Server) moderateReport(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		fail(w, r, invalid("id", "%q is not a report id", chi.URLParam(r, "id")))
		return
	}
	var body decisionBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&body); err != nil {
		fail(w, r, invalid("body", "expected JSON with status and an optional note"))
		return
	}

	status := reports.Status(body.Status)
	if status != reports.StatusApproved && status != reports.StatusRejected {
		fail(w, r, invalid("status", "expected approved or rejected, got %q", body.Status))
		return
	}

	err = s.reports.Moderate(r.Context(), id, status, strings.TrimSpace(body.Note))
	if errors.Is(err, reports.ErrNotFound) {
		fail(w, r, errNotFound)
		return
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": body.Status})
}

// The data sources are German, so a report outside the country is a mistake
// rather than an exotic location.
func checkInGermany(lon, lat float64) error {
	if lon < 5 || lon > 16 || lat < 46 || lat > 56 {
		return invalid("lon", "%.5f, %.5f liegt außerhalb Deutschlands", lon, lat)
	}
	return nil
}

var errReportsDisabled = errors.New("reports are not configured")
