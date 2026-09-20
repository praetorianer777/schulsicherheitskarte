package accidents_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/accidents"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/config"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/dbtest"
)

func zwickau(t *testing.T) *config.Config {
	t.Helper()
	cfg := &config.Config{Regions: []config.Region{{
		Name: "Landkreis Zwickau", AGSPrefixes: []string{"14524"},
		BBox: [4]float64{12.2263668, 50.5465600, 12.8061082, 50.9242066},
	}}}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestImportKeepsOnlyTheConfiguredRegion(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	t.Cleanup(func() { dbtest.Truncate(t, pool, "accidents") })

	// The fixture holds two Zwickau accidents and one from Berlin.
	result, err := accidents.Import(ctx, pool, strings.NewReader(fixture(t, "2016.csv")), zwickau(t))
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Read != 3 || result.Written != 2 || result.Skipped != 1 {
		t.Errorf("read %d, wrote %d, skipped %d; want 3, 2, 1", result.Read, result.Written, result.Skipped)
	}

	var outside int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM accidents WHERE ags NOT LIKE '14524%'").Scan(&outside); err != nil {
		t.Fatal(err)
	}
	if outside != 0 {
		t.Errorf("%d accidents from outside the configured region were imported", outside)
	}
}

func TestImportIsIdempotent(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	t.Cleanup(func() { dbtest.Truncate(t, pool, "accidents") })

	body := fixture(t, "2025.csv")
	first, err := accidents.Import(ctx, pool, strings.NewReader(body), zwickau(t))
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	second, err := accidents.Import(ctx, pool, strings.NewReader(body), zwickau(t))
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if second.Written != 0 {
		t.Errorf("re-importing the same file wrote %d rows, want 0", second.Written)
	}

	var total int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM accidents").Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != first.Written {
		t.Errorf("table holds %d accidents after two imports, want %d", total, first.Written)
	}
}

// A published file can list the same accident twice; ON CONFLICT cannot resolve
// a conflict between two rows of the same statement, so the import has to.
func TestDuplicateRowsWithinOneFile(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	t.Cleanup(func() { dbtest.Truncate(t, pool, "accidents") })

	const header = "ULAND;UREGBEZ;UKREIS;UGEMEINDE;UJAHR;UMONAT;USTUNDE;UWOCHENTAG;UKATEGORIE;UART;UTYP1;IstRad;IstPKW;IstFuss;IstKrad;IstSonstige;XGCSWGS84;YGCSWGS84\n"
	const row = "14;5;24;280;2024;5;7;3;2;3;4;0;1;1;0;0;12,6210000;50,7900000\n"

	result, err := accidents.Import(ctx, pool, strings.NewReader(header+row+row), zwickau(t))
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Written != 1 {
		t.Errorf("wrote %d rows for a file listing the same accident twice, want 1", result.Written)
	}
}

func TestImportStoresTheGeometry(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	t.Cleanup(func() { dbtest.Truncate(t, pool, "accidents") })

	if _, err := accidents.Import(ctx, pool, strings.NewReader(fixture(t, "2025.csv")), zwickau(t)); err != nil {
		t.Fatalf("import: %v", err)
	}

	// Every published coordinate has to come back where it was put, in the
	// right order. Rows are compared by position rather than by id: the import
	// deduplicates with DISTINCT ON, which fixes no row order.
	for _, want := range [][2]float64{{12.6230, 50.7920}, {12.6240, 50.7930}} {
		var distance float64
		const q = `
			SELECT min(ST_Distance(geom, ST_MakePoint($1, $2)::geography))
			FROM accidents`
		if err := pool.QueryRow(ctx, q, want[0], want[1]).Scan(&distance); err != nil {
			t.Fatal(err)
		}
		if distance > 1 {
			t.Errorf("no accident within 1 m of the published coordinate %v, %v; nearest is %.1f m away",
				want[0], want[1], distance)
		}
	}

	// Latitude and longitude the wrong way round would still land inside this
	// check if only one point were tested, so the extent is checked too.
	var extent float64
	if err := pool.QueryRow(ctx, `SELECT max(ST_X(geom::geometry)) FROM accidents`).Scan(&extent); err != nil {
		t.Fatal(err)
	}
	if extent > 16 {
		t.Errorf("longitude %.4f is outside Germany; the coordinate pair was stored swapped", extent)
	}
}

func TestImportRefusesAnUnusableHeader(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()

	if _, err := accidents.Import(ctx, pool, strings.NewReader("foo;bar\n1;2\n"), zwickau(t)); err == nil {
		t.Fatal("a file with an unusable header was imported")
	}
}
