package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Accident struct {
	ID         int64   `json:"id"`
	Lon        float64 `json:"lon"`
	Lat        float64 `json:"lat"`
	Severity   int     `json:"severity"`
	Year       int     `json:"year"`
	Month      int     `json:"month"`
	Hour       int     `json:"hour"`
	Weekday    int     `json:"weekday"`
	Pedestrian bool    `json:"pedestrian"`
	Bike       bool    `json:"bike"`
	Car        bool    `json:"car"`
	Motorcycle bool    `json:"motorcycle"`
	Truck      bool    `json:"truck"`
	Distance   int     `json:"distance"`
}

type accidentList struct {
	Accidents []Accident     `json:"accidents"`
	Summary   accidentCounts `json:"summary"`
	Radius    int            `json:"radius"`
	Sources   []Source       `json:"sources"`
}

type accidentCounts struct {
	Total      int `json:"total"`
	Fatal      int `json:"fatal"`
	Serious    int `json:"serious"`
	Slight     int `json:"slight"`
	Pedestrian int `json:"pedestrian"`
	Bike       int `json:"bike"`
}

type accidentFilter struct {
	radius int
	years  yearRange
	modes  modes
}

func parseAccidentFilter(r *http.Request) (accidentFilter, error) {
	var f accidentFilter
	var err error
	if f.radius, err = parseRadius(query(r, "radius")); err != nil {
		return f, err
	}
	if f.years, err = parseYears(query(r, "from"), query(r, "to")); err != nil {
		return f, err
	}
	if f.modes, err = parseModes(query(r, "modes")); err != nil {
		return f, err
	}
	return f, nil
}

// The filter is one expression used by both the list and the fact sheet, so the
// number on the sheet is the number on the map.
const accidentsNearInstitution = `
	FROM accidents a, institutions i
	WHERE i.id = $1
	  AND ST_DWithin(a.geom, i.geom, $2)
	  AND a.year BETWEEN $3 AND $4
	  AND (NOT $5::boolean OR a.pedestrian OR a.bike)
	  AND (NOT $6::boolean OR a.pedestrian)
	  AND (NOT $7::boolean OR a.bike)`

func (f accidentFilter) args(id int64) []any {
	// A single mode filters to that mode; both means either, not both at once —
	// an accident involving a pedestrian and a cyclist is rare, and a filter
	// that returns almost nothing is a filter nobody understands.
	both := f.modes.Foot && f.modes.Bike
	return []any{
		id, f.radius, f.years.From, f.years.To,
		both,
		f.modes.Foot && !both,
		f.modes.Bike && !both,
	}
}

func (s *Server) accidents(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	filter, err := parseAccidentFilter(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := s.requireInstitution(r.Context(), id); err != nil {
		fail(w, r, err)
		return
	}

	const q = `
		SELECT a.id, ST_X(a.geom::geometry), ST_Y(a.geom::geometry),
		       a.severity, a.year, a.month, a.hour, a.weekday,
		       a.pedestrian, a.bike, a.car, a.motorcycle, a.truck,
		       round(ST_Distance(a.geom, i.geom))::int` + accidentsNearInstitution + `
		ORDER BY a.severity, a.year DESC, a.id`

	rows, err := s.pool.Query(r.Context(), q, filter.args(id)...)
	if err != nil {
		fail(w, r, err)
		return
	}
	defer rows.Close()

	list := accidentList{Accidents: []Accident{}, Radius: filter.radius, Sources: []Source{sourceAccidents, sourceOSM}}
	for rows.Next() {
		var a Accident
		if err := rows.Scan(&a.ID, &a.Lon, &a.Lat, &a.Severity, &a.Year, &a.Month, &a.Hour,
			&a.Weekday, &a.Pedestrian, &a.Bike, &a.Car, &a.Motorcycle, &a.Truck, &a.Distance); err != nil {
			fail(w, r, err)
			return
		}
		list.Accidents = append(list.Accidents, a)
		list.Summary.Total++
		switch a.Severity {
		case 1:
			list.Summary.Fatal++
		case 2:
			list.Summary.Serious++
		default:
			list.Summary.Slight++
		}
		if a.Pedestrian {
			list.Summary.Pedestrian++
		}
		if a.Bike {
			list.Summary.Bike++
		}
	}
	if err := rows.Err(); err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// requireInstitution turns an unknown id into a 404 rather than an empty list,
// which would otherwise read as "no accidents here".
func (s *Server) requireInstitution(ctx context.Context, id int64) error {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM institutions WHERE id = $1)`, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return errNotFound
	}
	return nil
}
