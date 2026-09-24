package osm_test

import (
	"context"
	"testing"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/dbtest"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/osm"
)

// Unlike the other fixtures, boundaries.json is built by hand: the real
// relations carry thousands of points each. The ids, tags and member roles are
// the ones Overpass returns for the pilot region; the rings are squares, so a
// test can say which side of a border a point lies on.
func parsedBoundaries(t *testing.T) []osm.Boundary {
	t.Helper()
	items, err := osm.ParseBoundaries(fixture(t, "boundaries"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return items
}

func boundaryNamed(items []osm.Boundary, name string) (osm.Boundary, bool) {
	for _, b := range items {
		if b.Name == name {
			return b, true
		}
	}
	return osm.Boundary{}, false
}

func TestParseBoundariesKnowsMunicipalitiesFromDistricts(t *testing.T) {
	items := parsedBoundaries(t)

	for name, kind := range map[string]string{
		"St. Egidien":  "municipality",
		"Kuhschnappel": "district",
		"Chemnitz":     "municipality", // a kreisfreie Stadt is level 6
	} {
		b, ok := boundaryNamed(items, name)
		if !ok {
			t.Errorf("%s was not read", name)
			continue
		}
		if b.Kind != kind {
			t.Errorf("%s is a %s, want %s", name, b.Kind, kind)
		}
	}
}

// A Landkreis is level 6 as well. Taken for a town, it would name every school
// in the county after the county.
func TestParseBoundariesLeavesTheLandkreisOut(t *testing.T) {
	if b, ok := boundaryNamed(parsedBoundaries(t), "Zwickau"); ok {
		t.Errorf("the Landkreis was read as %+v; its five-digit key marks it as no municipality", b)
	}
}

func TestParseBoundariesSkipsWhatCannotNameATown(t *testing.T) {
	for _, b := range parsedBoundaries(t) {
		if b.OSMID == 417999 {
			t.Errorf("a boundary without a name was read: %+v", b)
		}
	}
}

func TestParseBoundariesKeepsOnlyTheRings(t *testing.T) {
	b, ok := boundaryNamed(parsedBoundaries(t), "St. Egidien")
	if !ok {
		t.Fatal("St. Egidien was not read")
	}
	// Two outer ways and the enclave; the admin_centre node and the subarea
	// relation are not part of the outline.
	if len(b.Lines) != 3 {
		t.Errorf("read %d lines, want 3", len(b.Lines))
	}
	if b.AGS != "14524280" || b.AdminLevel != 8 {
		t.Errorf("ags = %q, level = %d", b.AGS, b.AdminLevel)
	}
}

func importBoundaries(t *testing.T) *osm.BoundaryResult {
	t.Helper()
	pool := dbtest.Pool(t)
	t.Cleanup(func() { dbtest.Truncate(t, pool, "institutions", "boundaries") })
	result, err := osm.ImportBoundaries(context.Background(), dbtest.Pool(t), parsedBoundaries(t), pilot)
	if err != nil {
		t.Fatalf("import boundaries: %v", err)
	}
	return result
}

func townOf(t *testing.T, osmID int64) (town, district *string) {
	t.Helper()
	const q = `SELECT town, district FROM institutions WHERE osm_id = $1`
	if err := dbtest.Pool(t).QueryRow(context.Background(), q, osmID).Scan(&town, &district); err != nil {
		t.Fatalf("read town of %d: %v", osmID, err)
	}
	return town, district
}

func deref(s *string) string {
	if s == nil {
		return "<none>"
	}
	return *s
}

func TestImportBoundariesAssemblesAreasAndSkipsOpenRings(t *testing.T) {
	result := importBoundaries(t)

	// St. Egidien, Kuhschnappel and Chemnitz; the unclosed one is counted, not
	// stored.
	if result.Written != 3 || result.Broken != 1 {
		t.Errorf("result = %+v, want 3 written and 1 broken", result)
	}
}

func TestInstitutionsImportedAfterTheBoundariesGetTheirTown(t *testing.T) {
	importBoundaries(t)
	ctx := context.Background()

	items := []osm.Institution{
		school(1, 12.63, 50.78, "Grundschule St. Egidien"),  // in St. Egidien
		school(2, 12.61, 50.78, "Grundschule Kuhschnappel"), // in its district
		school(3, 12.63, 50.80, "Schule in der Enklave"),    // in the hole
		school(4, 12.30, 50.60, "Schule im Nirgendwo"),      // outside every boundary
	}
	if _, err := osm.ImportInstitutions(ctx, dbtest.Pool(t), items, pilot); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		osmID          int64
		town, district string
	}{
		{1, "St. Egidien", "<none>"},
		{2, "St. Egidien", "Kuhschnappel"},
		{3, "<none>", "<none>"},
		{4, "<none>", "<none>"},
	} {
		town, district := townOf(t, c.osmID)
		if deref(town) != c.town || deref(district) != c.district {
			t.Errorf("institution %d: town %s, district %s; want %s, %s",
				c.osmID, deref(town), deref(district), c.town, c.district)
		}
	}
}

// The boundaries can arrive after the institutions — on a first import that
// fails half way, or when only the boundaries are refreshed.
func TestImportingTheBoundariesUpdatesInstitutionsAlreadyThere(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	t.Cleanup(func() { dbtest.Truncate(t, pool, "institutions", "boundaries") })

	if _, err := osm.ImportInstitutions(ctx, pool, []osm.Institution{school(1, 12.63, 50.78, "Vorher da")}, pilot); err != nil {
		t.Fatal(err)
	}
	if town, _ := townOf(t, 1); town != nil {
		t.Fatalf("town = %s before any boundary was imported", *town)
	}

	importBoundaries(t)
	if town, _ := townOf(t, 1); deref(town) != "St. Egidien" {
		t.Errorf("town = %s after the boundaries arrived", deref(town))
	}
}

func TestTheTownIsSearchable(t *testing.T) {
	importBoundaries(t)
	ctx := context.Background()
	pool := dbtest.Pool(t)

	if _, err := osm.ImportInstitutions(ctx, pool, []osm.Institution{school(1, 12.61, 50.78, "Grundschule")}, pilot); err != nil {
		t.Fatal(err)
	}
	var text string
	if err := pool.QueryRow(ctx, "SELECT search_text FROM institutions WHERE osm_id = 1").Scan(&text); err != nil {
		t.Fatal(err)
	}
	if text != "grundschule st egidien kuhschnappel" {
		t.Errorf("search_text = %q; town and district have to be in it", text)
	}
}

func TestTownCoverageSaysWhereTheTownCameFrom(t *testing.T) {
	importBoundaries(t)
	ctx := context.Background()
	pool := dbtest.Pool(t)

	withAddress := school(1, 12.63, 50.78, "Mit Adresse")
	withAddress.Tags = map[string]string{"amenity": "school", "addr:city": "St. Egidien"}
	items := []osm.Institution{
		withAddress,
		school(2, 12.61, 50.78, "Nur über die Grenze"),
		school(3, 12.30, 50.60, "Ohne Ort"),
	}
	if _, err := osm.ImportInstitutions(ctx, pool, items, pilot); err != nil {
		t.Fatal(err)
	}

	got, err := osm.CountTownCoverage(ctx, pool, pilot)
	if err != nil {
		t.Fatal(err)
	}
	want := osm.TownCoverage{Total: 3, Address: 1, Boundary: 1, None: 1}
	if *got != want {
		t.Errorf("coverage = %+v, want %+v", *got, want)
	}
}

// A municipality merged into its neighbour disappears from OpenStreetMap; its
// name must not stay on the schools that now belong to the neighbour.
func TestVanishedBoundariesArePruned(t *testing.T) {
	importBoundaries(t)
	ctx := context.Background()
	pool := dbtest.Pool(t)

	var withoutDistrict []osm.Boundary
	for _, b := range parsedBoundaries(t) {
		if b.Kind != "district" {
			withoutDistrict = append(withoutDistrict, b)
		}
	}
	if _, err := osm.ImportInstitutions(ctx, pool, []osm.Institution{school(1, 12.61, 50.78, "Grundschule")}, pilot); err != nil {
		t.Fatal(err)
	}

	result, err := osm.ImportBoundaries(ctx, pool, withoutDistrict, pilot)
	if err != nil {
		t.Fatal(err)
	}
	if result.Pruned != 1 {
		t.Errorf("pruned %d, want the one district", result.Pruned)
	}
	if _, district := townOf(t, 1); district != nil {
		t.Errorf("district = %s after its boundary was removed", *district)
	}
}
