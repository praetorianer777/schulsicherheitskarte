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

	// NothingImported separates "no match" from "no data". Both arrive as an
	// empty list, but one is answered by trying another spelling and the other
	// by running the import — and an installation that has not been imported
	// yet answers every search this way.
	NothingImported bool `json:"nothingImported,omitempty"`

	Sources []Source `json:"sources"`
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

	var found []Institution
	switch {
	case rawBox != "":
		box, err := parseBBox(rawBox)
		if err != nil {
			fail(w, r, err)
			return
		}
		found, err = s.institutionsInBox(r.Context(), box, limit)
		if err != nil {
			fail(w, r, err)
			return
		}
	case name != "":
		found, err = s.institutionsByName(r.Context(), name, limit)
		if err != nil {
			fail(w, r, err)
			return
		}
	default:
		fail(w, r, invalid("q", "give either q to search by name or bbox to search the map view"))
		return
	}

	list := institutionList{Institutions: found, Sources: []Source{sourceOSM}}

	// Only asked when there is nothing to show, so the ordinary search pays
	// nothing for it.
	if len(found) == 0 {
		imported, err := s.anyInstitution(r.Context())
		if err != nil {
			fail(w, r, err)
			return
		}
		list.NothingImported = !imported
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) institutionsInBox(ctx context.Context, box [4]float64, limit int) ([]Institution, error) {
	const q = `SELECT ` + institutionColumns + ` FROM institutions
		WHERE ST_Intersects(geom, ST_MakeEnvelope($1, $2, $3, $4, 4326)::geography)
		ORDER BY name NULLS LAST, id LIMIT $5`
	rows, err := s.pool.Query(ctx, q, box[0], box[1], box[2], box[3], limit)
	if err != nil {
		return nil, err
	}
	return collectInstitutions(rows)
}

// institutionsByName matches the way the name was typed, not the way it is
// written in OpenStreetMap. Every word has to occur, in any order, in the name
// or the address, with punctuation and umlauts folded away by search_name:
// "bergschule egidien" and "Bergschule St Egidien" both have to reach
// "Bergschule St. Egidien", because nobody types the full stop — and "werdau"
// has to reach a school whose name does not say where it is.
const institutionsByWord = `
	WITH words AS (
		SELECT array_agg('%' || word || '%') AS patterns
		FROM unnest(string_to_array(search_name($1), ' ')) AS word
		WHERE word <> ''
	)
	SELECT ` + institutionColumns + ` FROM institutions, words
	WHERE name IS NOT NULL AND search_text LIKE ALL (words.patterns)
	ORDER BY length(name), name LIMIT $2`

// A misspelled word matches none of the above, and an empty page is a worse
// answer than a close one. Ordered by how close, so the guess stays visible as
// a guess. The name is compared on its own as well: the address makes the
// stored text longer, and a typo in a short name would otherwise drown in it.
const institutionsBySimilarity = `
	SELECT ` + institutionColumns + ` FROM institutions
	WHERE name IS NOT NULL
	  AND (search_name(name) % search_name($1) OR search_text % search_name($1))
	ORDER BY greatest(similarity(search_name(name), search_name($1)),
	                  similarity(search_text, search_name($1))) DESC,
	         length(name), name
	LIMIT $2`

func (s *Server) institutionsByName(ctx context.Context, name string, limit int) ([]Institution, error) {
	rows, err := s.pool.Query(ctx, institutionsByWord, name, limit)
	if err != nil {
		return nil, err
	}
	found, err := collectInstitutions(rows)
	if err != nil || len(found) > 0 {
		return found, err
	}

	rows, err = s.pool.Query(ctx, institutionsBySimilarity, name, limit)
	if err != nil {
		return nil, err
	}
	return collectInstitutions(rows)
}

func collectInstitutions(rows pgx.Rows) ([]Institution, error) {
	defer rows.Close()
	found := []Institution{}
	for rows.Next() {
		i, err := scanInstitution(rows)
		if err != nil {
			return nil, err
		}
		found = append(found, i)
	}
	return found, rows.Err()
}

func (s *Server) anyInstitution(ctx context.Context) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM institutions)`).Scan(&exists)
	return exists, err
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
