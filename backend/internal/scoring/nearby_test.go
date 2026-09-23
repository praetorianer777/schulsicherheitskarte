package scoring_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/dbtest"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/scoring"
)

func insertSchool(t *testing.T, pool *pgxpool.Pool, osmID int64, lon, lat float64) {
	t.Helper()
	const q = `INSERT INTO institutions (osm_type, osm_id, kind, name, geom)
		VALUES ('node', $1, 'school', 'Schule', ST_MakePoint($2, $3)::geography)`
	if _, err := pool.Exec(context.Background(), q, osmID, lon, lat); err != nil {
		t.Fatalf("insert school %d: %v", osmID, err)
	}
}

func accidentsNearby(t *testing.T, pool *pgxpool.Pool, osmID int64) *int {
	t.Helper()
	var n *int
	const q = `SELECT accidents_nearby FROM institutions WHERE osm_id = $1`
	if err := pool.QueryRow(context.Background(), q, osmID).Scan(&n); err != nil {
		t.Fatalf("read school %d: %v", osmID, err)
	}
	return n
}

// 0.0001° of latitude is 11.1 m.
func TestRecomputeCountsTheAccidentsWithin500Metres(t *testing.T) {
	pool, ctx := setup(t)
	t.Cleanup(func() { dbtest.Truncate(t, pool, "institutions") })

	insertSchool(t, pool, 1, 12.62, 50.79)
	insertSchool(t, pool, 2, 12.70, 50.79) // 5.6 km east, nothing around it
	insert(t, pool,
		accident{12.62, 50.7910, 3, 2025, false, false}, // 111 m
		accident{12.62, 50.7940, 2, 2016, true, false},  // 445 m, the oldest year counts too
		accident{12.62, 50.7946, 3, 2020, false, false}, // 511 m, outside
	)

	result, err := scoring.RecomputeHotspots(ctx, pool)
	if err != nil {
		t.Fatalf("recompute: %v", err)
	}
	if result.Institutions != 2 {
		t.Errorf("counted %d institutions, want 2", result.Institutions)
	}

	for osmID, want := range map[int64]int{1: 2, 2: 0} {
		got := accidentsNearby(t, pool, osmID)
		if got == nil || *got != want {
			t.Errorf("school %d: accidents_nearby = %v, want %d", osmID, got, want)
		}
	}
}

// A school imported after the last run has not been counted. Showing it as
// zero would say "no accidents here" about a place nobody has looked at.
func TestAnInstitutionNotYetCountedIsNotZero(t *testing.T) {
	pool, ctx := setup(t)
	t.Cleanup(func() { dbtest.Truncate(t, pool, "institutions") })

	insert(t, pool, accident{12.62, 50.7910, 3, 2025, false, false})
	if _, err := scoring.RecomputeHotspots(ctx, pool); err != nil {
		t.Fatal(err)
	}
	insertSchool(t, pool, 1, 12.62, 50.79)

	if got := accidentsNearby(t, pool, 1); got != nil {
		t.Errorf("accidents_nearby = %d before any count", *got)
	}
}
