package osm_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/osm"
)

// The fixtures are slices of real Overpass answers for the pilot region, kept
// with their original envelope so a change in the service's shape shows up here.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func byID(items []osm.Institution, id int64) (osm.Institution, bool) {
	for _, i := range items {
		if i.OSMID == id {
			return i, true
		}
	}
	return osm.Institution{}, false
}

func TestParseInstitutionsReadsEveryElementType(t *testing.T) {
	items, err := osm.ParseInstitutions(fixture(t, "institutions"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	seen := map[string]int{}
	for _, i := range items {
		seen[i.OSMType]++
		if i.Point.Lon == 0 || i.Point.Lat == 0 {
			t.Errorf("%s/%d has no coordinate", i.OSMType, i.OSMID)
		}
	}
	for _, kind := range []string{"node", "way", "relation"} {
		if seen[kind] == 0 {
			t.Errorf("no %s was read; ways and relations need their centre from `out center`", kind)
		}
	}
}

func TestParseInstitutionsReadsNameAndKind(t *testing.T) {
	items, err := osm.ParseInstitutions(fixture(t, "institutions"))
	if err != nil {
		t.Fatal(err)
	}

	school, ok := byID(items, 290932161)
	if !ok {
		t.Fatal("Grundschule Langenberg is missing")
	}
	if school.Name != "Grundschule Langenberg" || school.Kind != "school" {
		t.Errorf("got %q / %q", school.Name, school.Kind)
	}

	kindergarten, ok := byID(items, 33132233)
	if !ok {
		t.Fatal("Kindergarten Friedrich Fröbel is missing")
	}
	if kindergarten.Kind != "kindergarten" {
		t.Errorf("kind = %q, want kindergarten", kindergarten.Kind)
	}
	if kindergarten.Tags["amenity"] != "kindergarten" {
		t.Error("the raw tags were not kept")
	}
}

func institutionJSON(elements ...string) []byte {
	return []byte(`{"elements":[` + join(elements) + `]}`)
}

func join(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}

func node(id int64, lon, lat float64, tags map[string]string) string {
	t, _ := json.Marshal(tags)
	return fmt.Sprintf(`{"type":"node","id":%d,"lat":%f,"lon":%f,"tags":%s}`, id, lat, lon, t)
}

func area(id int64, lon, lat float64, tags map[string]string) string {
	t, _ := json.Marshal(tags)
	return fmt.Sprintf(`{"type":"way","id":%d,"center":{"lat":%f,"lon":%f},"tags":%s}`, id, lat, lon, t)
}

// The grounds and a node inside them are one school, not two.
func TestNodeInsideNamedGroundsIsDropped(t *testing.T) {
	body := institutionJSON(
		area(1, 12.62, 50.79, map[string]string{"amenity": "school", "name": "Goetheschule"}),
		node(2, 12.6205, 50.7902, map[string]string{"amenity": "school", "name": "Goetheschule"}),
	)
	items, err := osm.ParseInstitutions(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("kept %d entries for one school", len(items))
	}
	if items[0].OSMType != "way" {
		t.Errorf("kept the %s; the grounds are what the accidents sit around", items[0].OSMType)
	}
}

// Around here three schools are called Goetheschule, in different villages.
// Merging them would put one town's accidents on another town's page.
func TestSameNameFarApartIsKept(t *testing.T) {
	body := institutionJSON(
		area(1, 12.45762, 50.85362, map[string]string{"amenity": "school", "name": "Goetheschule"}),
		area(2, 12.68465, 50.71968, map[string]string{"amenity": "school", "name": "Goetheschule"}),
		area(3, 12.76594, 50.85966, map[string]string{"amenity": "school", "name": "Goetheschule"}),
	)
	items, err := osm.ParseInstitutions(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("kept %d of three schools that merely share a name", len(items))
	}
}

// OpenStreetMap holds grounds without a name beside a named node. The named one
// is the one a parent can search for.
func TestUnnamedGroundsBesideNamedNodeAreDropped(t *testing.T) {
	body := institutionJSON(
		area(1, 12.62, 50.79, map[string]string{"amenity": "kindergarten"}),
		node(2, 12.6204, 50.7901, map[string]string{"amenity": "kindergarten", "name": "Kita Pusteblume"}),
	)
	items, err := osm.ParseInstitutions(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("kept %d entries for one kindergarten", len(items))
	}
	if items[0].Name != "Kita Pusteblume" {
		t.Errorf("kept the unnamed entry; the searchable one is the named node")
	}
}

// Two unnamed entries carry no evidence that they are the same place, and
// dropping one would silently lose an institution.
func TestTwoUnnamedEntriesAreBothKept(t *testing.T) {
	body := institutionJSON(
		area(1, 12.62, 50.79, map[string]string{"amenity": "school"}),
		node(2, 12.6201, 50.7901, map[string]string{"amenity": "school"}),
	)
	items, err := osm.ParseInstitutions(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("kept %d of two entries that cannot be matched to each other", len(items))
	}
}

// A kindergarten next door to a school is not a duplicate of it.
func TestDifferentKindsAreNeverMerged(t *testing.T) {
	body := institutionJSON(
		area(1, 12.62, 50.79, map[string]string{"amenity": "school", "name": "Campus"}),
		node(2, 12.6201, 50.7901, map[string]string{"amenity": "kindergarten", "name": "Campus"}),
	)
	items, err := osm.ParseInstitutions(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("kept %d entries; a school and a kindergarten are not the same institution", len(items))
	}
}

func TestParseInfrastructureClassifiesByTags(t *testing.T) {
	items, err := osm.ParseInfrastructure(fixture(t, "crossings"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	seen := map[string]int{}
	for _, i := range items {
		seen[i.Kind]++
		if len(i.Geometry) == 0 {
			t.Errorf("%s/%d has no geometry", i.OSMType, i.OSMID)
		}
	}
	for _, kind := range []string{"crossing", "traffic_signals", "traffic_calming"} {
		if seen[kind] == 0 {
			t.Errorf("nothing was classified as %s", kind)
		}
	}
}

func TestSpeedLimitsKeepTheirLine(t *testing.T) {
	items, err := osm.ParseInfrastructure(fixture(t, "speed_limits"))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("no speed limits were read")
	}
	for _, i := range items {
		if i.Kind != "speed_limit" {
			t.Errorf("%s/%d was classified as %s", i.OSMType, i.OSMID, i.Kind)
		}
		// A limit applies to a stretch of road; reducing it to a point would
		// lose exactly the information that makes it useful.
		if len(i.Geometry) < 2 {
			t.Errorf("%s/%d kept %d points, want a line", i.OSMType, i.OSMID, len(i.Geometry))
		}
		if i.Tags["maxspeed"] == "" {
			t.Errorf("%s/%d has no maxspeed tag", i.OSMType, i.OSMID)
		}
	}
}

// A node can be a crossing and traffic calming at once; the schema keys on the
// kind, so both rows have to come out of the parser.
func TestOneElementCanBeSeveralKinds(t *testing.T) {
	body := []byte(`{"elements":[{"type":"node","id":1,"lat":50.79,"lon":12.62,
		"tags":{"highway":"crossing","traffic_calming":"table"}}]}`)
	items, err := osm.ParseInfrastructure(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("produced %d rows for a raised crossing, want 2", len(items))
	}
}

// Without an explicit maxspeed there is no limit in the data — the implicit
// limit of a residential street is not something OpenStreetMap states.
func TestRoadWithoutMaxspeedIsNotASpeedLimit(t *testing.T) {
	body := []byte(`{"elements":[{"type":"way","id":1,"tags":{"highway":"residential"},
		"geometry":[{"lat":50.79,"lon":12.62},{"lat":50.7901,"lon":12.6201}]}]}`)
	items, err := osm.ParseInfrastructure(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("produced %d rows for a road without a limit", len(items))
	}
}

func TestBrokenJSONIsRefused(t *testing.T) {
	if _, err := osm.ParseInstitutions([]byte("<html>service unavailable</html>")); err == nil {
		t.Fatal("an HTML error page was parsed as a response")
	}
}
