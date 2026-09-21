package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/api"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/dbtest"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/scoring"
)

// The fixture is one school with accidents at known distances, so a radius
// filter can be checked against a number rather than against "fewer than
// before".
const (
	schoolLon = 12.62
	schoolLat = 50.79
	// Roughly 110 m and 890 m north of the school.
	nearLat = schoolLat + 0.001
	farLat  = schoolLat + 0.008
)

type fixture struct {
	pool      *pgxpool.Pool
	server    *httptest.Server
	schoolID  int64
	kitaID    int64
	hotspotID int64
}

func seed(t *testing.T) *fixture {
	t.Helper()
	pool := dbtest.Pool(t)
	ctx := context.Background()
	t.Cleanup(func() {
		dbtest.Truncate(t, pool, "accidents", "institutions", "infrastructure", "hotspots")
	})

	f := &fixture{pool: pool}

	const institution = `
		INSERT INTO institutions (osm_type, osm_id, kind, name, school_type, geom, tags)
		VALUES ($1, $2, $3, $4, $5, ST_MakePoint($6, $7)::geography, $8) RETURNING id`
	if err := pool.QueryRow(ctx, institution, "way", 1, "school", "Goetheschule Meerane",
		"Grundschule", schoolLon, schoolLat, []byte(`{"amenity":"school"}`)).Scan(&f.schoolID); err != nil {
		t.Fatalf("seed school: %v", err)
	}
	if err := pool.QueryRow(ctx, institution, "node", 2, "kindergarten", "Kita Pusteblume",
		nil, schoolLon+0.01, schoolLat, []byte(`{"amenity":"kindergarten"}`)).Scan(&f.kitaID); err != nil {
		t.Fatalf("seed kindergarten: %v", err)
	}

	const accident = `
		INSERT INTO accidents (ags, year, month, hour, weekday, severity, kind, type,
			bike, car, pedestrian, motorcycle, truck, other, geom, source_hash)
		VALUES ('14524280', $1, 5, 7, 3, $2, 3, 4, $3, true, $4, false, false, false,
			ST_MakePoint($5, $6)::geography, $7)`
	accidents := []struct {
		year       int
		severity   int
		bike       bool
		pedestrian bool
		lat        float64
	}{
		{2025, 1, false, true, nearLat},  // fatal, pedestrian, near
		{2024, 2, true, false, nearLat},  // serious, cyclist, near
		{2018, 3, false, false, nearLat}, // slight, cars only, near, old
		{2025, 2, false, true, farLat},   // serious, pedestrian, far away
	}
	for i, a := range accidents {
		hash := []byte{byte(i), 0x11, 0x22}
		if _, err := pool.Exec(ctx, accident, a.year, a.severity, a.bike, a.pedestrian,
			schoolLon, a.lat, hash); err != nil {
			t.Fatalf("seed accident %d: %v", i, err)
		}
	}

	const hotspot = `
		INSERT INTO hotspots (geom, accident_count, score, severity_breakdown, first_year, last_year)
		VALUES (ST_MakePoint($1, $2)::geography, $3, $4, $5, $6, $7) RETURNING id`
	if err := pool.QueryRow(ctx, hotspot, schoolLon, nearLat, 3, 42.5,
		[]byte(`{"fatal":1,"serious":1,"slight":1,"pedestrian":1,"bike":1}`), 2018, 2025).Scan(&f.hotspotID); err != nil {
		t.Fatalf("seed hotspot: %v", err)
	}
	var lowScore int64
	if err := pool.QueryRow(ctx, hotspot, schoolLon, nearLat+0.0005, 2, 3.5,
		[]byte(`{"fatal":0,"serious":0,"slight":2,"pedestrian":0,"bike":0}`), 2019, 2020).Scan(&lowScore); err != nil {
		t.Fatalf("seed second hotspot: %v", err)
	}

	const infrastructure = `
		INSERT INTO infrastructure (osm_type, osm_id, kind, geom, tags)
		VALUES ($1, $2, $3, ST_GeogFromText($4), $5)`
	rows := []struct {
		osmType string
		osmID   int64
		kind    string
		wkt     string
		tags    string
	}{
		{"node", 10, "crossing", "SRID=4326;POINT(12.62 50.7905)", `{"highway":"crossing"}`},
		{"node", 11, "traffic_signals", "SRID=4326;POINT(12.6201 50.7906)", `{"highway":"traffic_signals"}`},
		{"way", 12, "speed_limit", "SRID=4326;LINESTRING(12.62 50.7903,12.6205 50.7907)", `{"maxspeed":"30"}`},
	}
	for _, row := range rows {
		if _, err := pool.Exec(ctx, infrastructure, row.osmType, row.osmID, row.kind, row.wkt, []byte(row.tags)); err != nil {
			t.Fatalf("seed infrastructure: %v", err)
		}
	}

	f.server = httptest.NewServer(api.New(pool, api.Options{}).Routes())
	t.Cleanup(f.server.Close)
	return f
}

func (f *fixture) get(t *testing.T, path string, into any) int {
	t.Helper()
	resp, err := f.server.Client().Get(f.server.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	if into != nil {
		if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
	}
	return resp.StatusCode
}

func TestHealthz(t *testing.T) {
	f := seed(t)
	var body map[string]string
	if status := f.get(t, "/healthz", &body); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if body["status"] != "ok" {
		t.Errorf("body = %v", body)
	}
}

func TestSearchByNameIsASubstringMatch(t *testing.T) {
	f := seed(t)
	// People look for "Goethe", not for the official "Goetheschule Meerane".
	var body struct {
		Institutions []api.Institution `json:"institutions"`
		Sources      []api.Source      `json:"sources"`
	}
	if status := f.get(t, "/api/institutions?q=goethe", &body); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if len(body.Institutions) != 1 {
		t.Fatalf("found %d institutions", len(body.Institutions))
	}
	if body.Institutions[0].ID != f.schoolID {
		t.Errorf("found the wrong institution: %+v", body.Institutions[0])
	}
	if len(body.Sources) == 0 {
		t.Error("the response carries no attribution; ODbL requires one")
	}
}

func TestSearchByBBox(t *testing.T) {
	f := seed(t)
	var body struct {
		Institutions []api.Institution `json:"institutions"`
	}
	// A box around the school only, excluding the kindergarten 700 m east.
	if status := f.get(t, "/api/institutions?bbox=12.615,50.785,12.625,50.795", &body); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if len(body.Institutions) != 1 || body.Institutions[0].ID != f.schoolID {
		t.Errorf("box returned %d institutions: %+v", len(body.Institutions), body.Institutions)
	}
}

func TestSearchWithoutAnyCriterionExplainsItself(t *testing.T) {
	f := seed(t)
	var body struct {
		Error     string `json:"error"`
		Parameter string `json:"parameter"`
	}
	if status := f.get(t, "/api/institutions", &body); status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
	if body.Error == "" {
		t.Error("the error says nothing about what was missing")
	}
}

// Bad input is the caller's mistake to fix, so it has to say what was wrong —
// and it must never reach the database and come back as a 500.
//
// A box that is merely outside Germany is not rejected: the map viewport
// legitimately reaches across the border when somebody zooms out, and it simply
// finds nothing there. Catching swapped coordinates belongs to the region
// configuration, which a person writes once, not to a box the map generates.
func TestInvalidParametersAreRejectedWithAReason(t *testing.T) {
	f := seed(t)
	cases := []struct {
		path      string
		parameter string
	}{
		{"/api/institutions?bbox=1,2,3", "bbox"},
		{"/api/institutions?bbox=a,b,c,d", "bbox"},
		{"/api/institutions?bbox=12.7,50.8,12.6,50.7", "bbox"},
		{"/api/institutions?bbox=-200,-100,200,100", "bbox"},
		{"/api/institutions?q=x&limit=0", "limit"},
		{"/api/institutions/1/accidents?radius=99999", "radius"},
		{"/api/institutions/1/accidents?radius=ten", "radius"},
		{"/api/institutions/1/accidents?from=2030&to=2020", "from"},
		{"/api/institutions/1/accidents?from=1800", "from"},
		{"/api/institutions/1/accidents?modes=bus", "modes"},
		{"/api/institutions/abc/accidents", "id"},
	}
	for _, c := range cases {
		var body struct {
			Error     string `json:"error"`
			Parameter string `json:"parameter"`
		}
		status := f.get(t, c.path, &body)
		if status != http.StatusBadRequest {
			t.Errorf("GET %s: status %d, want 400", c.path, status)
			continue
		}
		if body.Parameter != c.parameter {
			t.Errorf("GET %s: blamed %q, want %q", c.path, body.Parameter, c.parameter)
		}
		if body.Error == "" {
			t.Errorf("GET %s: no reason given", c.path)
		}
	}
}

// An unknown school has to be a 404. An empty list would read as "no accidents
// here", which is the opposite of the truth.
func TestUnknownInstitutionIsNotAnEmptyResult(t *testing.T) {
	f := seed(t)
	for _, path := range []string{
		"/api/institutions/999999",
		"/api/institutions/999999/accidents",
		"/api/institutions/999999/hotspots",
		"/api/institutions/999999/infrastructure",
		"/api/institutions/999999/factsheet",
	} {
		if status := f.get(t, path, nil); status != http.StatusNotFound {
			t.Errorf("GET %s: status %d, want 404", path, status)
		}
	}
}

type accidentsBody struct {
	Accidents []api.Accident `json:"accidents"`
	Summary   struct {
		Total, Fatal, Serious, Slight, Pedestrian, Bike int
	} `json:"summary"`
	Radius  int          `json:"radius"`
	Sources []api.Source `json:"sources"`
}

func (f *fixture) accidents(t *testing.T, query string) accidentsBody {
	t.Helper()
	var body accidentsBody
	path := "/api/institutions/" + itoa(f.schoolID) + "/accidents" + query
	if status := f.get(t, path, &body); status != http.StatusOK {
		t.Fatalf("GET %s: status %d", path, status)
	}
	return body
}

func itoa(id int64) string { return strconv.FormatInt(id, 10) }

func TestRadiusDecidesWhatCounts(t *testing.T) {
	f := seed(t)
	near := f.accidents(t, "?radius=300")
	if near.Summary.Total != 3 {
		t.Errorf("300 m returned %d accidents, want the 3 close ones", near.Summary.Total)
	}
	wide := f.accidents(t, "?radius=2000")
	if wide.Summary.Total != 4 {
		t.Errorf("2000 m returned %d accidents, want all 4", wide.Summary.Total)
	}
	if wide.Radius != 2000 {
		t.Errorf("response reports radius %d", wide.Radius)
	}
}

func TestYearRangeNarrowsTheResult(t *testing.T) {
	f := seed(t)
	recent := f.accidents(t, "?radius=300&from=2024")
	if recent.Summary.Total != 2 {
		t.Errorf("from 2024 returned %d accidents, want 2", recent.Summary.Total)
	}
	for _, a := range recent.Accidents {
		if a.Year < 2024 {
			t.Errorf("accident from %d slipped through the filter", a.Year)
		}
	}
}

func TestModeFilterIsTheSchoolRouteView(t *testing.T) {
	f := seed(t)

	onFoot := f.accidents(t, "?radius=300&modes=foot")
	if onFoot.Summary.Total != 1 {
		t.Errorf("modes=foot returned %d accidents, want 1", onFoot.Summary.Total)
	}

	// Both modes means either of them, not both at once: an accident involving
	// a pedestrian and a cyclist is rare, and a filter returning almost nothing
	// is a filter nobody understands.
	both := f.accidents(t, "?radius=300&modes=foot,bike")
	if both.Summary.Total != 2 {
		t.Errorf("modes=foot,bike returned %d accidents, want 2", both.Summary.Total)
	}
}

func TestSummaryCountsMatchTheAccidents(t *testing.T) {
	f := seed(t)
	body := f.accidents(t, "?radius=300")
	if body.Summary.Fatal != 1 || body.Summary.Serious != 1 || body.Summary.Slight != 1 {
		t.Errorf("summary = %+v", body.Summary)
	}
	if body.Summary.Pedestrian != 1 || body.Summary.Bike != 1 {
		t.Errorf("involvement counts = %d pedestrian, %d bike", body.Summary.Pedestrian, body.Summary.Bike)
	}
	if len(body.Accidents) != body.Summary.Total {
		t.Errorf("%d accidents listed but summary says %d", len(body.Accidents), body.Summary.Total)
	}
	for _, a := range body.Accidents {
		if a.Distance <= 0 || a.Distance > 300 {
			t.Errorf("accident %d reports distance %d m", a.ID, a.Distance)
		}
	}
}

func TestAccidentsCarryBothAttributions(t *testing.T) {
	f := seed(t)
	body := f.accidents(t, "?radius=300")
	var licences []string
	for _, s := range body.Sources {
		licences = append(licences, s.Licence)
	}
	if len(licences) < 2 {
		t.Errorf("sources = %v; accidents and the school come from two differently licensed sources", licences)
	}
}

func TestHotspotsAreRankedByScore(t *testing.T) {
	f := seed(t)
	var body struct {
		Hotspots []api.Hotspot `json:"hotspots"`
	}
	path := "/api/institutions/" + itoa(f.schoolID) + "/hotspots?radius=500"
	if status := f.get(t, path, &body); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if len(body.Hotspots) != 2 {
		t.Fatalf("found %d hotspots", len(body.Hotspots))
	}
	if body.Hotspots[0].Score < body.Hotspots[1].Score {
		t.Errorf("hotspots are not ranked: %v before %v", body.Hotspots[0].Score, body.Hotspots[1].Score)
	}
	if body.Hotspots[0].Breakdown["fatal"] != 1 {
		t.Errorf("breakdown = %v", body.Hotspots[0].Breakdown)
	}
}

func TestInfrastructureKeepsItsGeometry(t *testing.T) {
	f := seed(t)
	var body struct {
		Infrastructure []api.Infrastructure `json:"infrastructure"`
		Counts         map[string]int       `json:"counts"`
	}
	path := "/api/institutions/" + itoa(f.schoolID) + "/infrastructure?radius=500"
	if status := f.get(t, path, &body); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if body.Counts["crossing"] != 1 || body.Counts["traffic_signals"] != 1 || body.Counts["speed_limit"] != 1 {
		t.Errorf("counts = %v", body.Counts)
	}

	for _, item := range body.Infrastructure {
		var geometry struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(item.Geometry, &geometry); err != nil {
			t.Fatalf("geometry of %s is not GeoJSON: %v", item.Kind, err)
		}
		// A speed limit applies to a stretch of road, so the map has to be able
		// to draw it as a line.
		if item.Kind == "speed_limit" && geometry.Type != "LineString" {
			t.Errorf("speed limit handed out as %s", geometry.Type)
		}
		if item.Kind == "crossing" && geometry.Type != "Point" {
			t.Errorf("crossing handed out as %s", geometry.Type)
		}
	}
}

// The sheet has to print the weights it was actually produced with, not a copy
// that can go stale.
func TestFactsheetCarriesTheMethodItUsed(t *testing.T) {
	f := seed(t)
	var sheet api.Factsheet
	path := "/api/institutions/" + itoa(f.schoolID) + "/factsheet?radius=300"
	if status := f.get(t, path, &sheet); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}

	if sheet.Method.WeightFatal != scoring.WeightFatal ||
		sheet.Method.WeightVulnerable != scoring.WeightVulnerable ||
		sheet.Method.HalfLifeYears != scoring.HalfLifeYears {
		t.Errorf("method = %+v, does not match the scoring constants", sheet.Method)
	}
	if sheet.Method.ReferenceYear != 2025 {
		t.Errorf("reference year = %d, want the most recent reporting year 2025", sheet.Method.ReferenceYear)
	}
	if sheet.Summary.Total != 3 {
		t.Errorf("summary total = %d, want 3 within 300 m", sheet.Summary.Total)
	}
	if len(sheet.Hotspots) == 0 {
		t.Error("the sheet carries no hotspots")
	}
	if sheet.Counts["crossing"] != 1 {
		t.Errorf("infrastructure counts = %v", sheet.Counts)
	}
	if len(sheet.Sources) < 2 {
		t.Errorf("the sheet carries %d sources; both licences require attribution", len(sheet.Sources))
	}
}

// The sheet is printed and handed over, so it must not state a range the data
// does not cover. Without a "to" the filter reaches to a guard year far in the
// future, and printing that would be a claim about years nobody has data for.
func TestFactsheetReportsTheYearsItActuallyCovers(t *testing.T) {
	f := seed(t)
	var sheet api.Factsheet
	path := "/api/institutions/" + itoa(f.schoolID) + "/factsheet?radius=2000"
	if status := f.get(t, path, &sheet); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}

	// The fixture holds accidents from 2018 to 2025.
	if sheet.Years.From != 2018 || sheet.Years.To != 2025 {
		t.Errorf("years = %d–%d, want the range the data covers, 2018–2025", sheet.Years.From, sheet.Years.To)
	}
}

// The fact sheet and the map must not disagree: same filter, same number.
func TestFactsheetAgreesWithTheAccidentList(t *testing.T) {
	f := seed(t)
	list := f.accidents(t, "?radius=300&from=2024&modes=foot")

	var sheet api.Factsheet
	path := "/api/institutions/" + itoa(f.schoolID) + "/factsheet?radius=300&from=2024&modes=foot"
	if status := f.get(t, path, &sheet); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if sheet.Summary.Total != list.Summary.Total {
		t.Errorf("sheet says %d accidents, the map says %d", sheet.Summary.Total, list.Summary.Total)
	}
}

func TestPathOutsideTheAPINamesTheWrongPort(t *testing.T) {
	f := seed(t)
	// The proxy forwards the page as well as its routes, so both have to say it.
	for _, path := range []string{"/", "/einrichtung/1"} {
		var body map[string]string
		if status := f.get(t, path, &body); status != http.StatusNotFound {
			t.Fatalf("%s: status = %d", path, status)
		}
		if !strings.Contains(body["hint"], "WEB_PORT") {
			t.Errorf("%s: hint = %q, expected it to name WEB_PORT", path, body["hint"])
		}
	}
}

func TestUnknownAPIPathStaysAPlainNotFound(t *testing.T) {
	f := seed(t)
	// A wrong API call is the caller's own mistake and needs no deployment advice.
	var body map[string]string
	if status := f.get(t, "/api/gibtesnicht", &body); status != http.StatusNotFound {
		t.Fatalf("status = %d", status)
	}
	if body["hint"] != "" {
		t.Errorf("hint = %q, expected none", body["hint"])
	}
}

func TestEmptyDatabaseIsNotAnEmptySearchResult(t *testing.T) {
	f := seed(t)
	var match struct {
		Institutions    []api.Institution `json:"institutions"`
		NothingImported bool              `json:"nothingImported"`
	}

	// A miss on a filled database is a miss, and says nothing about the import.
	if status := f.get(t, "/api/institutions?q=gibtesnichtxyz", &match); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if len(match.Institutions) != 0 || match.NothingImported {
		t.Errorf("got %d institutions, nothingImported = %v", len(match.Institutions), match.NothingImported)
	}

	dbtest.Truncate(t, f.pool, "institutions")

	if status := f.get(t, "/api/institutions?q=gibtesnichtxyz", &match); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if !match.NothingImported {
		t.Error("an empty database answered like an empty result")
	}
}

func TestBBoxSearchAlsoReportsAnEmptyDatabase(t *testing.T) {
	f := seed(t)
	// The map view has the same problem: nothing in sight and nothing imported
	// look alike.
	dbtest.Truncate(t, f.pool, "institutions")

	var body struct {
		NothingImported bool `json:"nothingImported"`
	}
	if status := f.get(t, "/api/institutions?bbox=12.5,50.7,12.7,50.9", &body); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if !body.NothingImported {
		t.Error("an empty database answered like an empty map view")
	}
}

func addSchool(t *testing.T, f *fixture, name string, osmID int64) {
	t.Helper()
	const q = `INSERT INTO institutions (osm_type, osm_id, kind, name, geom, tags)
		VALUES ('node', $1, 'school', $2, ST_MakePoint($3, $4)::geography, '{}')`
	if _, err := f.pool.Exec(context.Background(), q, osmID, name, schoolLon, schoolLat); err != nil {
		t.Fatalf("insert %q: %v", name, err)
	}
}

func TestSearchMatchesHowPeopleTypeAName(t *testing.T) {
	f := seed(t)
	addSchool(t, f, "Bergschule St. Egidien", 900)
	addSchool(t, f, "Grundschule Mühlberger Straße", 901)

	for _, c := range []struct {
		typed string
		want  string
		why   string
	}{
		{"Bergschule St. Egidien", "Bergschule St. Egidien", "the official spelling"},
		{"Bergschule St Egidien", "Bergschule St. Egidien", "nobody types the full stop"},
		{"bergschule egidien", "Bergschule St. Egidien", "a word in the middle is left out"},
		{"St Egidien", "Bergschule St. Egidien", "only the place is remembered"},
		{"egidien bergschule", "Bergschule St. Egidien", "the words in the other order"},
		{"Muehlberger Strasse", "Grundschule Mühlberger Straße", "umlauts written out"},
		{"Bergshule Egidien", "Bergschule St. Egidien", "a typo still has to land"},
	} {
		var body institutionListBody
		if status := f.get(t, "/api/institutions?q="+url.QueryEscape(c.typed), &body); status != http.StatusOK {
			t.Fatalf("%q: status = %d", c.typed, status)
		}
		if len(body.Institutions) == 0 {
			t.Errorf("%q found nothing (%s)", c.typed, c.why)
			continue
		}
		if got := *body.Institutions[0].Name; got != c.want {
			t.Errorf("%q found %q, want %q (%s)", c.typed, got, c.want, c.why)
		}
	}
}

func TestSearchStillFindsNothingForNonsense(t *testing.T) {
	f := seed(t)
	addSchool(t, f, "Bergschule St. Egidien", 900)

	// Tolerating a typo must not turn into answering everything with anything.
	var body institutionListBody
	if status := f.get(t, "/api/institutions?q=zahnarztpraxis", &body); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if len(body.Institutions) != 0 {
		t.Errorf("got %d hits for a word that means nothing here", len(body.Institutions))
	}
}

type institutionListBody struct {
	Institutions []api.Institution `json:"institutions"`
}

func addSchoolAt(t *testing.T, f *fixture, name string, osmID int64, tags string) {
	t.Helper()
	const q = `INSERT INTO institutions (osm_type, osm_id, kind, name, geom, tags)
		VALUES ('node', $1, 'school', $2, ST_MakePoint($3, $4)::geography, $5)`
	if _, err := f.pool.Exec(context.Background(), q, osmID, name, schoolLon, schoolLat, []byte(tags)); err != nil {
		t.Fatalf("insert %q: %v", name, err)
	}
}

func TestSearchFindsASchoolByItsTown(t *testing.T) {
	f := seed(t)
	addSchoolAt(t, f, "Gerhart-Hauptmann-Grundschule", 910,
		`{"addr:city":"Werdau","addr:street":"Gerhard-Weck-Straße"}`)
	addSchoolAt(t, f, "Umweltschule Werdau", 911, `{}`)
	addSchoolAt(t, f, "Grundschule Stenn", 912, `{"addr:city":"Lichtentanne","addr:suburb":"Stenn"}`)

	for _, c := range []struct {
		typed string
		want  []string
		why   string
	}{
		{"werdau", []string{"Umweltschule Werdau", "Gerhart-Hauptmann-Grundschule"},
			"the town is the one thing a parent knows for certain"},
		{"werdau grundschule", []string{"Gerhart-Hauptmann-Grundschule"},
			"town and kind together narrow it down"},
		{"weck strasse", []string{"Gerhart-Hauptmann-Grundschule"},
			"the street, umlaut written out"},
		{"lichtentanne", []string{"Grundschule Stenn"},
			"the municipality, which the name does not say"},
	} {
		var body institutionListBody
		if status := f.get(t, "/api/institutions?q="+url.QueryEscape(c.typed), &body); status != http.StatusOK {
			t.Fatalf("%q: status = %d", c.typed, status)
		}
		var got []string
		for _, i := range body.Institutions {
			got = append(got, *i.Name)
		}
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("%q found %v, want %v (%s)", c.typed, got, c.want, c.why)
		}
	}
}

func TestExtentCoversEveryInstitution(t *testing.T) {
	f := seed(t)
	var e api.Extent
	if status := f.get(t, "/api/extent", &e); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if e.Institutions != 2 || e.BBox == nil {
		t.Fatalf("extent = %+v", e)
	}
	// The school and the kindergarten 0.01° apart span the box.
	box := *e.BBox
	if box[0] != schoolLon || box[2] < schoolLon+0.009 || box[1] != schoolLat || box[3] != schoolLat {
		t.Errorf("bbox = %v", box)
	}
}

func TestExtentOfAnEmptyDatabaseIsNotABox(t *testing.T) {
	f := seed(t)
	dbtest.Truncate(t, f.pool, "institutions")
	var e api.Extent
	if status := f.get(t, "/api/extent", &e); status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if e.Institutions != 0 || e.BBox != nil {
		t.Errorf("extent = %+v, expected no box and no institutions", e)
	}
}
