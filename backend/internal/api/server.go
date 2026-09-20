// Package api serves the read side of the map.
package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
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
}

type Server struct {
	pool    *pgxpool.Pool
	options Options
}

func New(pool *pgxpool.Pool, options Options) *Server {
	return &Server{pool: pool, options: options}
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

	r.Route("/api", func(r chi.Router) {
		r.Get("/institutions", s.searchInstitutions)
		r.Route("/institutions/{id}", func(r chi.Router) {
			r.Get("/", s.institution)
			r.Get("/accidents", s.accidents)
			r.Get("/hotspots", s.hotspots)
			r.Get("/infrastructure", s.infrastructure)
			r.Get("/factsheet", s.factsheet)
		})
	})
	return r
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
