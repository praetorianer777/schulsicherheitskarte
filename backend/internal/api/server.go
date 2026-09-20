// Package api serves the read side of the map.
package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Server {
	return &Server{pool: pool}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
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
