// Package api serves the read side of the map.
package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/reports"
)

type Options struct {
	// TrustProxyHeaders makes the server believe X-Forwarded-For, which is what
	// shows the real client instead of the reverse proxy in front of it.
	//
	// It defaults to off, and it has to: the header is whatever the caller sent.
	// As long as the API port can be reached directly, anyone can put an
	// arbitrary address in it. Turning it on is a statement that a proxy is the
	// only way in. The rate limiting on reports will key on this address, and a
	// rate limit keyed on a value the caller chooses is no rate limit at all.
	TrustProxyHeaders bool

	// ModerationToken guards the moderation queue. Empty means the queue is
	// closed rather than open: a missing token must not mean no check.
	ModerationToken string
}

type Server struct {
	pool    *pgxpool.Pool
	options Options

	// nil when no salt is configured, which turns the reporting endpoints off
	// rather than letting them run with a predictable fingerprint.
	reports *reports.Store
}

func New(pool *pgxpool.Pool, options Options) *Server {
	return &Server{pool: pool, options: options}
}

// WithReports enables the reporting endpoints.
func (s *Server) WithReports(store *reports.Store) *Server {
	s.reports = store
	return s
}

func (s *Server) Routes() http.Handler { return s.router() }

// router returns the concrete mux so a test can mount a probe on it and see
// what the middleware chain did to a request.
func (s *Server) router() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer)
	if s.options.TrustProxyHeaders {
		r.Use(middleware.RealIP)
	}
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/healthz", s.health)
	r.NotFound(s.notFound)

	r.Route("/api", func(r chi.Router) {
		r.Get("/institutions", s.searchInstitutions)
		r.Route("/institutions/{id}", func(r chi.Router) {
			r.Get("/", s.institution)
			r.Get("/accidents", s.accidents)
			r.Get("/hotspots", s.hotspots)
			r.Get("/infrastructure", s.infrastructure)
			r.Get("/factsheet", s.factsheet)
			r.Get("/reports", s.institutionReports)
		})

		r.Get("/reports", s.listReports)
		r.Post("/reports", s.createReport)
		r.Post("/reports/{id}/confirm", s.confirmReport)

		r.Route("/admin/reports", func(r chi.Router) {
			r.Use(s.moderationOnly)
			r.Get("/", s.moderationQueue)
			r.Post("/{id}", s.moderateReport)
		})
	})
	return r
}

// notFound answers what the API does not serve. The case worth spending words
// on is not a mistyped path but a reverse proxy pointed at the API port instead
// of the web port: the site then answers 404 on every page, and "404 page not
// found" gives the operator nothing to go on. A path outside /api is far more
// likely to be that than a wrong API call, so it gets the explanation.
func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "not found"})
		return
	}
	writeJSON(w, http.StatusNotFound, errorBody{
		Error: "not found",
		Hint: "This port serves the API: /api/... and /healthz. The website is served " +
			"by the web container on WEB_PORT — a reverse proxy belongs there, not here. " +
			"See DEPLOY.md, section 4a.",
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		// A server that answers "ok" while its database is gone is worse than
		// one that is down, because nothing restarts it.
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "database unreachable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
