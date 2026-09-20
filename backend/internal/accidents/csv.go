// Package accidents imports the Unfallatlas open data published by the German
// statistical offices (licence dl-de/by-2-0).
package accidents

import (
	"bufio"
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Record is one accident, already reduced to what the schema stores.
type Record struct {
	AGS           string
	Year          int
	Month         int
	Hour          int
	Weekday       int
	Severity      int
	Kind          int
	Type          int
	Light         *int
	RoadCondition *int

	Bike       bool
	Car        bool
	Pedestrian bool
	Motorcycle bool
	Truck      bool
	Other      bool

	Lon float64
	Lat float64

	SourceHash []byte
}

// The published column names drift between reporting years, so columns are
// looked up by meaning rather than by position: IstGkfz appears only from 2017,
// the road condition is called IstStrassenzustand in the early years and
// USTRZUSTAND later, and the trailing "e" of IstSonstige comes and goes.
// Matching is case-insensitive.
var columnAliases = map[string][]string{
	"land":     {"ULAND"},
	"regbez":   {"UREGBEZ"},
	"kreis":    {"UKREIS"},
	"gemeinde": {"UGEMEINDE"},
	"year":     {"UJAHR"},
	"month":    {"UMONAT"},
	"hour":     {"USTUNDE"},
	"weekday":  {"UWOCHENTAG"},
	"severity": {"UKATEGORIE"},
	"kind":     {"UART"},
	"type":     {"UTYP1"},
	"light":    {"ULICHTVERH"},
	// IstStrasse is IstStrassenzustand truncated to ten characters, an artefact
	// of the shapefile column limit that the 2016 CSV inherited.
	"road":       {"IstStrassenzustand", "USTRZUSTAND", "STRZUSTAND", "IstStrasse"},
	"bike":       {"IstRad"},
	"car":        {"IstPKW"},
	"pedestrian": {"IstFuss"},
	"motorcycle": {"IstKrad"},
	"truck":      {"IstGkfz"},
	"other":      {"IstSonstige", "IstSonstig", "IstSonstiges"},
	"lon":        {"XGCSWGS84"},
	"lat":        {"YGCSWGS84"},
}

// Without these a row cannot be placed on a map or attributed to a
// municipality, so a file missing any of them is not usable at all.
var requiredColumns = []string{
	"land", "regbez", "kreis", "gemeinde",
	"year", "month", "hour", "weekday",
	"severity", "kind", "type",
	"bike", "car", "pedestrian", "motorcycle", "other",
	"lon", "lat",
}

// Order is fixed so the digest of a row does not change when the published
// column order does.
var hashedFields = []string{
	"land", "regbez", "kreis", "gemeinde", "year", "month", "hour", "weekday",
	"severity", "kind", "type", "light", "road",
	"bike", "car", "pedestrian", "motorcycle", "truck", "other", "lon", "lat",
}

type Reader struct {
	csv     *csv.Reader
	columns map[string]int
	line    int
}

// NewReader reads and validates the header. An unknown header fails here, with
// the columns it did find, rather than silently shifting every value by one.
func NewReader(r io.Reader) (*Reader, error) {
	br := bufio.NewReader(r)

	// A UTF-8 BOM would become part of the first column name.
	if bom, err := br.Peek(3); err == nil && bom[0] == 0xEF && bom[1] == 0xBB && bom[2] == 0xBF {
		_, _ = br.Discard(3)
	}

	header, err := br.ReadString('\n')
	if err != nil && header == "" {
		return nil, fmt.Errorf("read header: %w", err)
	}
	delimiter := detectDelimiter(header)

	columns, err := mapColumns(splitRecord(header, delimiter))
	if err != nil {
		return nil, err
	}

	cr := csv.NewReader(br)
	cr.Comma = delimiter
	cr.FieldsPerRecord = -1 // trailing empty columns appear in some years
	cr.ReuseRecord = true
	cr.LazyQuotes = true

	return &Reader{csv: cr, columns: columns, line: 1}, nil
}

func detectDelimiter(header string) rune {
	if strings.Count(header, ";") >= strings.Count(header, ",") {
		return ';'
	}
	return ','
}

func splitRecord(line string, delimiter rune) []string {
	line = strings.TrimRight(line, "\r\n")
	parts := strings.Split(line, string(delimiter))
	for i := range parts {
		parts[i] = strings.TrimSpace(strings.Trim(parts[i], `"`))
	}
	return parts
}

func mapColumns(header []string) (map[string]int, error) {
	index := make(map[string]int, len(header))
	for i, name := range header {
		index[strings.ToLower(name)] = i
	}

	columns := map[string]int{}
	for field, aliases := range columnAliases {
		for _, alias := range aliases {
			if i, ok := index[strings.ToLower(alias)]; ok {
				columns[field] = i
				break
			}
		}
	}

	var missing []string
	for _, field := range requiredColumns {
		if _, ok := columns[field]; !ok {
			missing = append(missing, field+" ("+strings.Join(columnAliases[field], " or ")+")")
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("unusable header: no column for %s; found %s",
			strings.Join(missing, ", "), strings.Join(header, ", "))
	}
	return columns, nil
}

// Next returns the next record, or io.EOF at the end of the file.
func (r *Reader) Next() (*Record, error) {
	for {
		row, err := r.csv.Read()
		if err != nil {
			return nil, err
		}
		r.line++

		// Some exports end with a stray empty line.
		if len(row) == 1 && strings.TrimSpace(row[0]) == "" {
			continue
		}

		rec, err := r.parse(row)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", r.line, err)
		}
		return rec, nil
	}
}

func (r *Reader) parse(row []string) (*Record, error) {
	get := func(field string) (string, bool) {
		i, ok := r.columns[field]
		if !ok || i >= len(row) {
			return "", false
		}
		return strings.TrimSpace(row[i]), true
	}

	num := func(field string) (int, error) {
		raw, ok := get(field)
		if !ok {
			return 0, fmt.Errorf("column %s is missing", field)
		}
		v, err := strconv.Atoi(raw)
		if err != nil {
			return 0, fmt.Errorf("column %s: %q is not a number", field, raw)
		}
		return v, nil
	}

	optional := func(field string) *int {
		raw, ok := get(field)
		if !ok || raw == "" {
			return nil
		}
		v, err := strconv.Atoi(raw)
		if err != nil {
			return nil
		}
		return &v
	}

	land, err := num("land")
	if err != nil {
		return nil, err
	}
	regbez, err := num("regbez")
	if err != nil {
		return nil, err
	}
	kreis, err := num("kreis")
	if err != nil {
		return nil, err
	}
	gemeinde, err := num("gemeinde")
	if err != nil {
		return nil, err
	}

	rec := &Record{
		// The AGS is fixed width per part: 2 + 1 + 2 + 3. Leading zeros are part
		// of the key, so it is assembled rather than concatenated as published.
		AGS:           fmt.Sprintf("%02d%01d%02d%03d", land, regbez, kreis, gemeinde),
		Light:         optional("light"),
		RoadCondition: optional("road"),
	}

	for _, f := range []struct {
		name   string
		target *int
	}{
		{"year", &rec.Year}, {"month", &rec.Month}, {"hour", &rec.Hour},
		{"weekday", &rec.Weekday}, {"severity", &rec.Severity},
		{"kind", &rec.Kind}, {"type", &rec.Type},
	} {
		v, err := num(f.name)
		if err != nil {
			return nil, err
		}
		*f.target = v
	}

	for _, f := range []struct {
		name   string
		target *bool
	}{
		{"bike", &rec.Bike}, {"car", &rec.Car}, {"pedestrian", &rec.Pedestrian},
		{"motorcycle", &rec.Motorcycle}, {"truck", &rec.Truck}, {"other", &rec.Other},
	} {
		raw, ok := get(f.name)
		if !ok {
			continue // truck is absent in the earliest reporting years
		}
		*f.target = raw == "1"
	}

	if rec.Lon, err = coordinate(get, "lon"); err != nil {
		return nil, err
	}
	if rec.Lat, err = coordinate(get, "lat"); err != nil {
		return nil, err
	}

	if err := rec.validate(); err != nil {
		return nil, err
	}

	rec.SourceHash = hashRow(get)
	return rec, nil
}

// The coordinate columns use a decimal comma in some years and a decimal point
// in others.
func coordinate(get func(string) (string, bool), field string) (float64, error) {
	raw, ok := get(field)
	if !ok || raw == "" {
		return 0, fmt.Errorf("column %s is missing", field)
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", "."), 64)
	if err != nil {
		return 0, fmt.Errorf("column %s: %q is not a coordinate", field, raw)
	}
	return v, nil
}

func (r *Record) validate() error {
	if r.Severity < 1 || r.Severity > 3 {
		return fmt.Errorf("severity %d is outside the published range 1..3", r.Severity)
	}
	if r.Month < 1 || r.Month > 12 {
		return fmt.Errorf("month %d is out of range", r.Month)
	}
	if r.Hour < 0 || r.Hour > 23 {
		return fmt.Errorf("hour %d is out of range", r.Hour)
	}
	if r.Weekday < 1 || r.Weekday > 7 {
		return fmt.Errorf("weekday %d is out of range", r.Weekday)
	}
	// Germany, generously bounded. A swapped pair lands in the Indian Ocean and
	// would otherwise be imported as a perfectly valid point.
	if r.Lon < 5 || r.Lon > 16 || r.Lat < 46 || r.Lat > 56 {
		return fmt.Errorf("coordinate %.5f, %.5f is outside Germany; longitude and latitude may be swapped", r.Lon, r.Lat)
	}
	return nil
}

// hashRow digests the parsed values, not the raw line: a re-published file with
// different whitespace, column order or line endings must not look like a set
// of new accidents.
func hashRow(get func(string) (string, bool)) []byte {
	h := sha256.New()
	for _, field := range hashedFields {
		raw, _ := get(field)
		h.Write([]byte(field))
		h.Write([]byte{'='})
		h.Write([]byte(strings.ReplaceAll(raw, ",", ".")))
		h.Write([]byte{'\n'})
	}
	return h.Sum(nil)
}
