package api

import "net/http"

// Extent is what the start page opens on: where the imported institutions are.
// It comes from the data rather than from regions.yaml, so the map shows where
// the data actually ends — and an empty database is visible as one.
type Extent struct {
	// minLon, minLat, maxLon, maxLat; nil when nothing has been imported.
	BBox         *[4]float64 `json:"bbox"`
	Institutions int         `json:"institutions"`
}

func (s *Server) extent(w http.ResponseWriter, r *http.Request) {
	const q = `
		SELECT count, ST_XMin(box), ST_YMin(box), ST_XMax(box), ST_YMax(box)
		FROM (SELECT count(*) AS count, ST_Extent(geom::geometry) AS box FROM institutions) AS t`
	var e Extent
	var minLon, minLat, maxLon, maxLat *float64
	if err := s.pool.QueryRow(r.Context(), q).Scan(&e.Institutions, &minLon, &minLat, &maxLon, &maxLat); err != nil {
		fail(w, r, err)
		return
	}
	if minLon != nil {
		e.BBox = &[4]float64{*minLon, *minLat, *maxLon, *maxLat}
	}
	writeJSON(w, http.StatusOK, e)
}
