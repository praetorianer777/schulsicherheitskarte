package osm_test

import (
	"context"
	"testing"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/dbtest"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/osm"
)

// The pilot region, wide enough to hold every fixture coordinate.
var pilot = osm.BBox{12.2263668, 50.5465600, 12.8061082, 50.9242066}

func school(id int64, lon, lat float64, name string) osm.Institution {
	return osm.Institution{
		OSMType: "way", OSMID: id, Kind: "school", Name: name,
		Point: osm.Point{Lon: lon, Lat: lat}, Tags: map[string]string{"amenity": "school"},
	}
}

func TestImportInstitutionsIsIdempotent(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	t.Cleanup(func() { dbtest.Truncate(t, pool, "institutions") })

	items := []osm.Institution{
		school(1, 12.62, 50.79, "Grundschule St. Egidien"),
		school(2, 12.63, 50.80, "Oberschule Lichtenstein"),
	}
	if _, err := osm.ImportInstitutions(ctx, pool, items, pilot); err != nil {
		t.Fatalf("first import: %v", err)
	}
	if _, err := osm.ImportInstitutions(ctx, pool, items, pilot); err != nil {
		t.Fatalf("second import: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM institutions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("table holds %d institutions after importing two twice", count)
	}
}

func TestImportInstitutionsUpdatesAChangedName(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	t.Cleanup(func() { dbtest.Truncate(t, pool, "institutions") })

	if _, err := osm.ImportInstitutions(ctx, pool, []osm.Institution{school(1, 12.62, 50.79, "Alte Schule")}, pilot); err != nil {
		t.Fatal(err)
	}
	if _, err := osm.ImportInstitutions(ctx, pool, []osm.Institution{school(1, 12.62, 50.79, "Neue Schule")}, pilot); err != nil {
		t.Fatal(err)
	}

	var name string
	if err := pool.QueryRow(ctx, "SELECT name FROM institutions WHERE osm_id = 1").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "Neue Schule" {
		t.Errorf("name = %q; a rename in OpenStreetMap did not reach the database", name)
	}
}

// A school demolished in OpenStreetMap has to leave the map here too, or it
// stays forever and nobody knows why.
func TestVanishedInstitutionsArePruned(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	t.Cleanup(func() { dbtest.Truncate(t, pool, "institutions") })

	both := []osm.Institution{school(1, 12.62, 50.79, "Bleibt"), school(2, 12.63, 50.80, "Verschwindet")}
	if _, err := osm.ImportInstitutions(ctx, pool, both, pilot); err != nil {
		t.Fatal(err)
	}

	result, err := osm.ImportInstitutions(ctx, pool, both[:1], pilot)
	if err != nil {
		t.Fatal(err)
	}
	if result.Pruned != 1 {
		t.Errorf("pruned %d rows, want 1", result.Pruned)
	}

	var remaining string
	if err := pool.QueryRow(ctx, "SELECT name FROM institutions").Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != "Bleibt" {
		t.Errorf("the wrong institution survived: %q", remaining)
	}
}

// Pruning is bounded by the imported box. Importing one region must not empty
// another.
func TestPruningStaysInsideTheImportedBox(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	t.Cleanup(func() { dbtest.Truncate(t, pool, "institutions") })

	berlin := osm.BBox{13.088, 52.338, 13.761, 52.675}
	if _, err := osm.ImportInstitutions(ctx, pool, []osm.Institution{school(9, 13.4, 52.52, "Berliner Schule")}, berlin); err != nil {
		t.Fatal(err)
	}
	if _, err := osm.ImportInstitutions(ctx, pool, []osm.Institution{school(1, 12.62, 50.79, "Zwickauer Schule")}, pilot); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM institutions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("table holds %d institutions; importing one region deleted another's", count)
	}
}

func crossing(id int64, lon, lat float64) osm.Infrastructure {
	return osm.Infrastructure{
		OSMType: "node", OSMID: id, Kind: "crossing",
		Geometry: []osm.Point{{Lon: lon, Lat: lat}},
		Tags:     map[string]string{"highway": "crossing"},
	}
}

func TestSpeedLimitKeepsItsLine(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	t.Cleanup(func() { dbtest.Truncate(t, pool, "infrastructure") })

	limit := osm.Infrastructure{
		OSMType: "way", OSMID: 1, Kind: "speed_limit",
		Geometry: []osm.Point{{Lon: 12.62, Lat: 50.79}, {Lon: 12.621, Lat: 50.791}, {Lon: 12.622, Lat: 50.792}},
		Tags:     map[string]string{"highway": "residential", "maxspeed": "30"},
	}
	if _, err := osm.ImportInfrastructure(ctx, pool, []osm.Infrastructure{limit}, pilot, []string{"speed_limit"}); err != nil {
		t.Fatal(err)
	}

	var geomType string
	var points int
	const q = `SELECT GeometryType(geom::geometry), ST_NPoints(geom::geometry) FROM infrastructure`
	if err := pool.QueryRow(ctx, q).Scan(&geomType, &points); err != nil {
		t.Fatal(err)
	}
	if geomType != "LINESTRING" || points != 3 {
		t.Errorf("stored a %s with %d points; a limit applies to a stretch of road", geomType, points)
	}
}

// One query fetches crossings, another speed limits. The crossings run must
// not delete the limits the other run wrote.
func TestPruningTouchesOnlyTheImportedKinds(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	t.Cleanup(func() { dbtest.Truncate(t, pool, "infrastructure") })

	limit := osm.Infrastructure{
		OSMType: "way", OSMID: 1, Kind: "speed_limit",
		Geometry: []osm.Point{{Lon: 12.62, Lat: 50.79}, {Lon: 12.621, Lat: 50.791}},
		Tags:     map[string]string{"maxspeed": "30"},
	}
	if _, err := osm.ImportInfrastructure(ctx, pool, []osm.Infrastructure{limit}, pilot, []string{"speed_limit"}); err != nil {
		t.Fatal(err)
	}
	if _, err := osm.ImportInfrastructure(ctx, pool, []osm.Infrastructure{crossing(2, 12.63, 50.80)}, pilot,
		[]string{"crossing", "traffic_signals", "traffic_calming"}); err != nil {
		t.Fatal(err)
	}

	var kinds int
	if err := pool.QueryRow(ctx, "SELECT count(DISTINCT kind) FROM infrastructure").Scan(&kinds); err != nil {
		t.Fatal(err)
	}
	if kinds != 2 {
		t.Errorf("%d kinds left; importing crossings pruned the speed limits", kinds)
	}
}

// The same node can be a crossing and traffic calming; the key includes the
// kind, so both rows have to survive.
func TestOneNodeCanHoldTwoKinds(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	t.Cleanup(func() { dbtest.Truncate(t, pool, "infrastructure") })

	point := []osm.Point{{Lon: 12.62, Lat: 50.79}}
	items := []osm.Infrastructure{
		{OSMType: "node", OSMID: 1, Kind: "crossing", Geometry: point, Tags: map[string]string{}},
		{OSMType: "node", OSMID: 1, Kind: "traffic_calming", Geometry: point, Tags: map[string]string{}},
	}
	if _, err := osm.ImportInfrastructure(ctx, pool, items, pilot, []string{"crossing", "traffic_calming"}); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM infrastructure WHERE osm_id = 1").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("stored %d rows for a raised crossing, want 2", count)
	}
}

func TestImportsTheRealFixtures(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	t.Cleanup(func() { dbtest.Truncate(t, pool, "institutions", "infrastructure") })

	institutions, err := osm.ParseInstitutions(fixture(t, "institutions"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := osm.ImportInstitutions(ctx, pool, institutions, pilot); err != nil {
		t.Fatalf("import institutions: %v", err)
	}

	for _, name := range []string{"crossings", "speed_limits"} {
		items, err := osm.ParseInfrastructure(fixture(t, name))
		if err != nil {
			t.Fatal(err)
		}
		kinds := []string{"crossing", "traffic_signals", "traffic_calming"}
		if name == "speed_limits" {
			kinds = []string{"speed_limit"}
		}
		if _, err := osm.ImportInfrastructure(ctx, pool, items, pilot, kinds); err != nil {
			t.Fatalf("import %s: %v", name, err)
		}
	}

	var institutionCount, infrastructureCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM institutions").Scan(&institutionCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM infrastructure").Scan(&infrastructureCount); err != nil {
		t.Fatal(err)
	}
	if institutionCount == 0 || infrastructureCount == 0 {
		t.Errorf("imported %d institutions and %d infrastructure rows from the real fixtures",
			institutionCount, infrastructureCount)
	}
}
