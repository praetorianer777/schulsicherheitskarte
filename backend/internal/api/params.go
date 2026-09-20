package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// A bad request is a mistake somebody can fix, so it says what was wrong and
// what was expected. An unexplained 400 costs whoever hits it an afternoon.
type badRequest struct {
	Parameter string
	Reason    string
}

func (e *badRequest) Error() string {
	return fmt.Sprintf("%s: %s", e.Parameter, e.Reason)
}

func invalid(parameter, format string, args ...any) *badRequest {
	return &badRequest{Parameter: parameter, Reason: fmt.Sprintf(format, args...)}
}

// BBox is minLon, minLat, maxLon, maxLat.
type BBox [4]float64

func parseBBox(raw string) (BBox, error) {
	var box BBox
	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		return box, invalid("bbox", "expected four comma-separated numbers (minLon,minLat,maxLon,maxLat), got %d", len(parts))
	}
	for i, part := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return box, invalid("bbox", "%q is not a number", part)
		}
		box[i] = v
	}
	if box[0] >= box[2] || box[1] >= box[3] {
		return box, invalid("bbox", "empty or inverted; the order is minLon,minLat,maxLon,maxLat")
	}
	if box[0] < -180 || box[2] > 180 || box[1] < -90 || box[3] > 90 {
		return box, invalid("bbox", "outside the coordinate range; longitude and latitude may be swapped")
	}
	return box, nil
}

// Radius limits. Below the lower bound nothing is found because the
// coordinates are snapped to the road network anyway; above the upper one the
// result stops being about a school route.
const (
	minRadiusMetres     = 50
	maxRadiusMetres     = 2000
	defaultRadiusMetres = 500
)

func parseRadius(raw string) (int, error) {
	if raw == "" {
		return defaultRadiusMetres, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, invalid("radius", "%q is not a whole number of metres", raw)
	}
	if v < minRadiusMetres || v > maxRadiusMetres {
		return 0, invalid("radius", "%d m is outside %d–%d m", v, minRadiusMetres, maxRadiusMetres)
	}
	return v, nil
}

// The Unfallatlas starts in 2016; the upper bound only has to keep a typo from
// becoming a query over a thousand years.
const (
	firstReportingYear = 2016
	lastPlausibleYear  = 2100
)

type yearRange struct {
	From int
	To   int
}

func parseYears(from, to string) (yearRange, error) {
	years := yearRange{From: firstReportingYear, To: lastPlausibleYear}
	if from != "" {
		v, err := strconv.Atoi(from)
		if err != nil {
			return years, invalid("from", "%q is not a year", from)
		}
		years.From = v
	}
	if to != "" {
		v, err := strconv.Atoi(to)
		if err != nil {
			return years, invalid("to", "%q is not a year", to)
		}
		years.To = v
	}
	if years.From < firstReportingYear || years.To > lastPlausibleYear {
		return years, invalid("from", "the published data starts in %d and cannot reach past %d", firstReportingYear, lastPlausibleYear)
	}
	if years.From > years.To {
		return years, invalid("from", "%d is after to=%d", years.From, years.To)
	}
	return years, nil
}

// Modes narrows the accidents to those involving somebody on foot or on a
// bicycle — the school route view of the data.
type modes struct {
	Foot bool
	Bike bool
}

// Any returns false when no filter was asked for, which means every accident.
func (m modes) Any() bool { return m.Foot || m.Bike }

func parseModes(raw string) (modes, error) {
	var m modes
	if strings.TrimSpace(raw) == "" {
		return m, nil
	}
	for _, part := range strings.Split(raw, ",") {
		switch strings.TrimSpace(strings.ToLower(part)) {
		case "foot":
			m.Foot = true
		case "bike":
			m.Bike = true
		case "":
		default:
			return m, invalid("modes", "%q is not a mode; expected foot, bike or both", part)
		}
	}
	return m, nil
}

func parseLimit(raw string, fallback, max int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, invalid("limit", "%q is not a whole number", raw)
	}
	if v < 1 || v > max {
		return 0, invalid("limit", "%d is outside 1–%d", v, max)
	}
	return v, nil
}

func parseID(raw string) (int64, error) {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < 1 {
		return 0, invalid("id", "%q is not an institution id", raw)
	}
	return v, nil
}

func query(r *http.Request, name string) string {
	return strings.TrimSpace(r.URL.Query().Get(name))
}
