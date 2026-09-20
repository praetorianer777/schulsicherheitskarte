package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/scoring"
)

// Factsheet is everything a printed page for a road safety inspection needs,
// in one request — including how the numbers were produced. A sheet that states
// a danger index without saying what it is invites the question and has no
// answer ready.
type Factsheet struct {
	Institution Institution    `json:"institution"`
	Radius      int            `json:"radius"`
	Years       factsheetYears `json:"years"`
	Summary     accidentCounts `json:"summary"`
	Hotspots    []Hotspot      `json:"hotspots"`
	Counts      map[string]int `json:"infrastructureCounts"`
	Method      Method         `json:"method"`
	Sources     []Source       `json:"sources"`
}

type factsheetYears struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// Method carries the weights the index was computed with, so the sheet prints
// the figures it was actually produced from rather than a copy that can go
// stale.
type Method struct {
	WeightFatal      float64 `json:"weightFatal"`
	WeightSerious    float64 `json:"weightSerious"`
	WeightSlight     float64 `json:"weightSlight"`
	WeightVulnerable float64 `json:"weightVulnerable"`
	HalfLifeYears    float64 `json:"halfLifeYears"`
	ClusterRadius    float64 `json:"clusterRadiusMetres"`
	MinAccidents     int     `json:"clusterMinAccidents"`
	ReferenceYear    int     `json:"referenceYear"`
}

// How many hotspots a printed page can carry without becoming a list nobody
// reads. The rest stay available through the hotspots endpoint.
const factsheetHotspots = 5

func (s *Server) factsheet(w http.ResponseWriter, r *http.Request) {
	institution, err := s.loadInstitution(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	filter, err := parseAccidentFilter(r)
	if err != nil {
		fail(w, r, err)
		return
	}

	sheet := Factsheet{
		Institution: institution,
		Radius:      filter.radius,
		Years:       factsheetYears{From: filter.years.From, To: filter.years.To},
		Counts:      map[string]int{},
		Method: Method{
			WeightFatal:      scoring.WeightFatal,
			WeightSerious:    scoring.WeightSerious,
			WeightSlight:     scoring.WeightSlight,
			WeightVulnerable: scoring.WeightVulnerable,
			HalfLifeYears:    scoring.HalfLifeYears,
			ClusterRadius:    scoring.ClusterRadiusMetres,
			MinAccidents:     scoring.ClusterMinAccidents,
		},
		Sources: []Source{sourceAccidents, sourceOSM},
	}

	const summary = `
		SELECT count(*),
		       count(*) FILTER (WHERE a.severity = 1),
		       count(*) FILTER (WHERE a.severity = 2),
		       count(*) FILTER (WHERE a.severity = 3),
		       count(*) FILTER (WHERE a.pedestrian),
		       count(*) FILTER (WHERE a.bike)` + accidentsNearInstitution
	if err := s.pool.QueryRow(r.Context(), summary, filter.args(institution.ID)...).Scan(
		&sheet.Summary.Total, &sheet.Summary.Fatal, &sheet.Summary.Serious,
		&sheet.Summary.Slight, &sheet.Summary.Pedestrian, &sheet.Summary.Bike); err != nil {
		fail(w, r, err)
		return
	}

	if sheet.Hotspots, err = s.loadHotspots(r.Context(), institution.ID, filter.radius, factsheetHotspots); err != nil {
		fail(w, r, err)
		return
	}

	const counts = `
		SELECT f.kind, count(*)
		FROM infrastructure f, institutions i
		WHERE i.id = $1 AND ST_DWithin(f.geom, i.geom, $2)
		GROUP BY f.kind`
	rows, err := s.pool.Query(r.Context(), counts, institution.ID, filter.radius)
	if err != nil {
		fail(w, r, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var n int
		if err := rows.Scan(&kind, &n); err != nil {
			fail(w, r, err)
			return
		}
		sheet.Counts[kind] = n
	}
	if err := rows.Err(); err != nil {
		fail(w, r, err)
		return
	}

	// The reporting years the data actually holds. The sheet is printed and
	// handed to an authority, so it must not state a range it does not cover —
	// the upper bound of an unrestricted filter is a guard value, not a claim.
	var first, last *int
	const span = `SELECT min(year), max(year) FROM accidents`
	if err := s.pool.QueryRow(r.Context(), span).Scan(&first, &last); err != nil {
		fail(w, r, err)
		return
	}
	if last != nil {
		sheet.Method.ReferenceYear = *last
		if sheet.Years.To > *last {
			sheet.Years.To = *last
		}
	}
	if first != nil && sheet.Years.From < *first {
		sheet.Years.From = *first
	}

	writeJSON(w, http.StatusOK, sheet)
}
