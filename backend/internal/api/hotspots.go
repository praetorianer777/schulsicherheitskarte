package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Hotspot struct {
	ID        int64          `json:"id"`
	Lon       float64        `json:"lon"`
	Lat       float64        `json:"lat"`
	Count     int            `json:"accidentCount"`
	Score     float64        `json:"score"`
	Breakdown map[string]int `json:"breakdown"`
	FirstYear int            `json:"firstYear"`
	LastYear  int            `json:"lastYear"`
	Distance  int            `json:"distance"`
}

type hotspotList struct {
	Hotspots []Hotspot `json:"hotspots"`
	Radius   int       `json:"radius"`
	Sources  []Source  `json:"sources"`
}

func (s *Server) hotspots(w http.ResponseWriter, r *http.Request) {
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

	hotspots, err := s.loadHotspots(r.Context(), id, radius, 0)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, hotspotList{
		Hotspots: hotspots,
		Radius:   radius,
		Sources:  []Source{sourceAccidents, sourceOSM},
	})
}

// limit of zero means all of them.
func (s *Server) loadHotspots(ctx context.Context, id int64, radius, limit int) ([]Hotspot, error) {
	q := `
		SELECT h.id, ST_X(h.geom::geometry), ST_Y(h.geom::geometry),
		       h.accident_count, h.score, h.severity_breakdown,
		       h.first_year, h.last_year, round(ST_Distance(h.geom, i.geom))::int
		FROM hotspots h, institutions i
		WHERE i.id = $1 AND ST_DWithin(h.geom, i.geom, $2)
		ORDER BY h.score DESC, h.id`
	args := []any{id, radius}
	if limit > 0 {
		q += ` LIMIT $3`
		args = append(args, limit)
	}

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hotspots := []Hotspot{}
	for rows.Next() {
		var h Hotspot
		var breakdown []byte
		if err := rows.Scan(&h.ID, &h.Lon, &h.Lat, &h.Count, &h.Score, &breakdown,
			&h.FirstYear, &h.LastYear, &h.Distance); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(breakdown, &h.Breakdown); err != nil {
			return nil, err
		}
		hotspots = append(hotspots, h)
	}
	return hotspots, rows.Err()
}
