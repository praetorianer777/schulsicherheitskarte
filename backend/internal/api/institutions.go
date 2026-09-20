package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type Institution struct {
	ID         int64             `json:"id"`
	Kind       string            `json:"kind"`
	Name       *string           `json:"name"`
	SchoolType *string           `json:"schoolType,omitempty"`
	Lon        float64           `json:"lon"`
	Lat        float64           `json:"lat"`
	Tags       map[string]string `json:"tags,omitempty"`
	OSMType    string            `json:"osmType"`
	OSMID      int64             `json:"osmId"`
}

type institutionList struct {
	Institutions []Institution `json:"institutions"`
	Sources      []Source      `json:"sources"`
}

const institutionColumns = `
	id, kind, name, school_type,
	ST_X(geom::geometry), ST_Y(geom::geometry),
	osm_type, osm_id`

func scanInstitution(rows pgx.Rows) (Institution, error) {
	var i Institution
	err := rows.Scan(&i.ID, &i.Kind, &i.Name, &i.SchoolType, &i.Lon, &i.Lat, &i.OSMType, &i.OSMID)
	return i, err
}

// searchInstitutions answers the start page: either by name, or by what is
// inside the map's current view.
func (s *Server) searchInstitutions(w http.ResponseWriter, r *http.Request) {
	name := query(r, "q")
	rawBox := query(r, "bbox")

	limit, err := parseLimit(query(r, "limit"), 50, 500)
	if err != nil {
		fail(w, r, err)
		return
	}

	var rows pgx.Rows
	switch {
	case rawBox != "":
		box, err := parseBBox(rawBox)
		if err != nil {
			fail(w, r, err)
			return
		}
		const q = `SELECT ` + institutionColumns + ` FROM institutions
			WHERE ST_Intersects(geom, ST_MakeEnvelope($1, $2, $3, $4, 4326)::geography)
			ORDER BY name NULLS LAST, id LIMIT $5`
		rows, err = s.pool.Query(r.Context(), q, box[0], box[1], box[2], box[3], limit)
		if err != nil {
			fail(w, r, err)
			return
		}
	case name != "":
		// Substring rather than prefix: people look for "Goethe", not for the
		// official "Goetheschule Meerane".
		const q = `SELECT ` + institutionColumns + ` FROM institutions
			WHERE name ILIKE '%' || $1 || '%'
			ORDER BY length(name), name LIMIT $2`
		rows, err = s.pool.Query(r.Context(), q, name, limit)
		if err != nil {
			fail(w, r, err)
			return
		}
	default:
		fail(w, r, invalid("q", "give either q to search by name or bbox to search the map view"))
		return
	}
	defer rows.Close()

	list := institutionList{Institutions: []Institution{}, Sources: []Source{sourceOSM}}
	for rows.Next() {
		i, err := scanInstitution(rows)
		if err != nil {
			fail(w, r, err)
			return
		}
		list.Institutions = append(list.Institutions, i)
	}
	if err := rows.Err(); err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

type institutionDetail struct {
	Institution Institution `json:"institution"`
	Sources     []Source    `json:"sources"`
}

func (s *Server) institution(w http.ResponseWriter, r *http.Request) {
	i, err := s.loadInstitution(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, institutionDetail{Institution: i, Sources: []Source{sourceOSM}})
}

func (s *Server) loadInstitution(ctx context.Context, rawID string) (Institution, error) {
	id, err := parseID(rawID)
	if err != nil {
		return Institution{}, err
	}

	var i Institution
	var tags []byte
	const q = `SELECT ` + institutionColumns + `, tags FROM institutions WHERE id = $1`
	err = s.pool.QueryRow(ctx, q, id).Scan(
		&i.ID, &i.Kind, &i.Name, &i.SchoolType, &i.Lon, &i.Lat, &i.OSMType, &i.OSMID, &tags)
	if err == pgx.ErrNoRows {
		return Institution{}, errNotFound
	}
	if err != nil {
		return Institution{}, err
	}
	if len(tags) > 0 {
		_ = json.Unmarshal(tags, &i.Tags)
	}
	return i, nil
}

// DisplayName is what the fact sheet prints when OpenStreetMap has no name.
func (i Institution) DisplayName() string {
	if i.Name != nil && strings.TrimSpace(*i.Name) != "" {
		return *i.Name
	}
	return ""
}
