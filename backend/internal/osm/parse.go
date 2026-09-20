package osm

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Point struct {
	Lon float64
	Lat float64
}

type Institution struct {
	OSMType    string
	OSMID      int64
	Kind       string // school | kindergarten
	Name       string
	SchoolType string
	Point      Point
	Tags       map[string]string
}

type Infrastructure struct {
	OSMType  string
	OSMID    int64
	Kind     string // crossing | traffic_signals | traffic_calming | speed_limit
	Geometry []Point
	Tags     map[string]string
}

type response struct {
	Elements []element `json:"elements"`
}

type element struct {
	Type     string            `json:"type"`
	ID       int64             `json:"id"`
	Lat      *float64          `json:"lat"`
	Lon      *float64          `json:"lon"`
	Center   *Point            `json:"-"`
	RawCentr json.RawMessage   `json:"center"`
	Geometry []geoPoint        `json:"geometry"`
	Tags     map[string]string `json:"tags"`
}

type geoPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

func decode(body []byte) ([]element, error) {
	var r response
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("parse overpass response: %w", err)
	}
	for i := range r.Elements {
		if len(r.Elements[i].RawCentr) == 0 {
			continue
		}
		var c geoPoint
		if err := json.Unmarshal(r.Elements[i].RawCentr, &c); err != nil {
			return nil, fmt.Errorf("parse center of %s/%d: %w", r.Elements[i].Type, r.Elements[i].ID, err)
		}
		r.Elements[i].Center = &Point{Lon: c.Lon, Lat: c.Lat}
	}
	return r.Elements, nil
}

// point returns where an element sits: its own coordinate for a node, the
// centre for a way or relation asked for with "out center", or the middle of
// its geometry.
func (e element) point() (Point, bool) {
	if e.Lat != nil && e.Lon != nil {
		return Point{Lon: *e.Lon, Lat: *e.Lat}, true
	}
	if e.Center != nil {
		return *e.Center, true
	}
	if len(e.Geometry) > 0 {
		p := e.Geometry[len(e.Geometry)/2]
		return Point{Lon: p.Lon, Lat: p.Lat}, true
	}
	return Point{}, false
}

// ParseInstitutions reads schools and kindergartens.
//
// A school is regularly mapped twice: as the grounds and as a node inside them.
// Importing both puts two markers on the map and splits the accidents around
// one school across two pages, so the weaker of the pair is dropped.
func ParseInstitutions(body []byte) ([]Institution, error) {
	elements, err := decode(body)
	if err != nil {
		return nil, err
	}

	var all []Institution
	for _, e := range elements {
		kind := e.Tags["amenity"]
		if kind != "school" && kind != "kindergarten" {
			continue
		}
		p, ok := e.point()
		if !ok {
			continue
		}
		all = append(all, Institution{
			OSMType:    e.Type,
			OSMID:      e.ID,
			Kind:       kind,
			Name:       strings.TrimSpace(e.Tags["name"]),
			SchoolType: schoolType(e.Tags),
			Point:      p,
			Tags:       e.Tags,
		})
	}
	return deduplicate(all), nil
}

// How close two mappings of the same institution are assumed to be. Matching
// names allow a wider radius: the node sits somewhere on the grounds, whose
// centre can be a building away. Without a name the only evidence is distance,
// so the radius is tighter.
const (
	namedRadiusMetres   = 300
	unnamedRadiusMetres = 150
)

// rank orders the mappings of one institution by how much they are worth
// keeping. A named entry beats an unnamed one because it is the one a parent
// can search for. Among named ones the more complete mapping wins: a relation
// describes the school, a way its grounds, a node only a spot on the map.
func rank(i Institution) int {
	switch {
	case i.Name == "":
		return 0
	case i.OSMType == "relation":
		return 3
	case i.OSMType == "way":
		return 2
	default:
		return 1
	}
}

// better is a total order, so exactly one of a group survives. The id decides
// between two mappings of the same type, which makes the result independent of
// the order Overpass happened to return them in.
func better(a, b Institution) bool {
	if rank(a) != rank(b) {
		return rank(a) > rank(b)
	}
	return a.OSMID < b.OSMID
}

// deduplicate keeps the best mapping of each institution.
//
// It walks the entries best first and keeps one only if nothing already kept
// supersedes it. Comparing against survivors rather than against all entries is
// what keeps a discarded entry from discarding a third one: with A, B and C in
// a row 250 m apart, B loses to A, and C — 500 m from A and no longer measured
// against B — survives as the separate school it may well be.
func deduplicate(all []Institution) []Institution {
	order := make([]Institution, len(all))
	copy(order, all)
	sort.SliceStable(order, func(i, j int) bool { return better(order[i], order[j]) })

	var kept []Institution
	for _, candidate := range order {
		if !supersededBy(candidate, kept) {
			kept = append(kept, candidate)
		}
	}
	sort.Slice(kept, func(i, j int) bool {
		if kept[i].OSMType != kept[j].OSMType {
			return kept[i].OSMType < kept[j].OSMType
		}
		return kept[i].OSMID < kept[j].OSMID
	})
	return kept
}

func supersededBy(candidate Institution, kept []Institution) bool {
	for _, other := range kept {
		if other.Kind != candidate.Kind {
			continue
		}
		if candidate.Name == "" {
			// Two unnamed entries carry no evidence that they are the same
			// place, and dropping one would silently lose an institution. Only
			// a named neighbour settles it.
			if other.Name != "" && withinMetres(candidate.Point, other.Point, unnamedRadiusMetres) {
				return true
			}
			continue
		}
		// Two schools in neighbouring villages regularly share a name — around
		// here, three are called Goetheschule — so a name alone is not enough.
		if strings.EqualFold(candidate.Name, other.Name) &&
			withinMetres(candidate.Point, other.Point, namedRadiusMetres) {
			return true
		}
	}
	return false
}

// A flat approximation is accurate to well under a metre at these distances and
// this latitude, and avoids a geodesy dependency for a comparison whose
// threshold is a judgement call anyway.
func withinMetres(a, b Point, metres float64) bool {
	const degreeLat = 111_320.0
	degreeLon := 111_320.0 * 0.62 // cos(51°), the latitude of the pilot region
	dLat := (a.Lat - b.Lat) * degreeLat
	dLon := (a.Lon - b.Lon) * degreeLon
	return dLat*dLat+dLon*dLon <= metres*metres
}

// German schools carry their type in a handful of competing tags; the raw tags
// are stored alongside, so a better answer later costs no re-import.
func schoolType(tags map[string]string) string {
	for _, key := range []string{"school:type", "school:DE:type", "school:DE", "isced:level"} {
		if v := strings.TrimSpace(tags[key]); v != "" {
			return v
		}
	}
	return ""
}

// ParseInfrastructure reads crossings, signals, traffic calming and speed
// limits. The kind is decided by tags, because one query returns several kinds
// and a single element can be more than one of them.
func ParseInfrastructure(body []byte) ([]Infrastructure, error) {
	elements, err := decode(body)
	if err != nil {
		return nil, err
	}

	var out []Infrastructure
	for _, e := range elements {
		geometry := geometryOf(e)
		if len(geometry) == 0 {
			continue
		}
		for _, kind := range kindsOf(e.Tags) {
			out = append(out, Infrastructure{
				OSMType:  e.Type,
				OSMID:    e.ID,
				Kind:     kind,
				Geometry: geometry,
				Tags:     e.Tags,
			})
		}
	}
	return out, nil
}

func kindsOf(tags map[string]string) []string {
	var kinds []string
	switch tags["highway"] {
	case "crossing":
		kinds = append(kinds, "crossing")
	case "traffic_signals":
		kinds = append(kinds, "traffic_signals")
	}
	if tags["traffic_calming"] != "" {
		kinds = append(kinds, "traffic_calming")
	}
	// A road carries a limit only together with an explicit maxspeed; the
	// implicit limit of a residential street is not stated in the data.
	if tags["maxspeed"] != "" && tags["highway"] != "" {
		kinds = append(kinds, "speed_limit")
	}
	return kinds
}

func geometryOf(e element) []Point {
	if len(e.Geometry) > 0 {
		points := make([]Point, 0, len(e.Geometry))
		for _, g := range e.Geometry {
			points = append(points, Point{Lon: g.Lon, Lat: g.Lat})
		}
		return points
	}
	if p, ok := e.point(); ok {
		return []Point{p}
	}
	return nil
}
