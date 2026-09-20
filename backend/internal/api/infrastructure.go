package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Infrastructure struct {
	ID       int64             `json:"id"`
	Kind     string            `json:"kind"`
	Geometry json.RawMessage   `json:"geometry"` // GeoJSON: a point or a line
	Tags     map[string]string `json:"tags,omitempty"`
	Distance int               `json:"distance"`
}

type infrastructureList struct {
	Infrastructure []Infrastructure `json:"infrastructure"`
	Counts         map[string]int   `json:"counts"`
	Radius         int              `json:"radius"`
	Sources        []Source         `json:"sources"`
}

func (s *Server) infrastructure(w http.ResponseWriter, r *http.Request) {
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

	// Geometry is handed out as GeoJSON because a speed limit is a line and the
	// map has to draw it as one.
	const q = `
		SELECT f.id, f.kind, ST_AsGeoJSON(f.geom::geometry), f.tags,
		       round(ST_Distance(f.geom, i.geom))::int
		FROM infrastructure f, institutions i
		WHERE i.id = $1 AND ST_DWithin(f.geom, i.geom, $2)
		ORDER BY f.kind, f.id`

	rows, err := s.pool.Query(r.Context(), q, id, radius)
	if err != nil {
		fail(w, r, err)
		return
	}
	defer rows.Close()

	list := infrastructureList{
		Infrastructure: []Infrastructure{},
		Counts:         map[string]int{},
		Radius:         radius,
		Sources:        []Source{sourceOSM},
	}
	for rows.Next() {
		var f Infrastructure
		var geometry string
		var tags []byte
		if err := rows.Scan(&f.ID, &f.Kind, &geometry, &tags, &f.Distance); err != nil {
			fail(w, r, err)
			return
		}
		f.Geometry = json.RawMessage(geometry)
		if len(tags) > 0 {
			_ = json.Unmarshal(tags, &f.Tags)
		}
		list.Infrastructure = append(list.Infrastructure, f)
		list.Counts[f.Kind]++
	}
	if err := rows.Err(); err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}
