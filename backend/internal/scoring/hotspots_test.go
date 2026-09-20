package scoring_test

import (
	"context"
	"math"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/dbtest"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/scoring"
)

type accident struct {
	lon, lat   float64
	severity   int
	year       int
	pedestrian bool
	bike       bool
}

func insert(t *testing.T, pool *pgxpool.Pool, accidents ...accident) {
	t.Helper()
	ctx := context.Background()
	const q = `
		INSERT INTO accidents (ags, year, month, hour, weekday, severity, kind, type,
			bike, car, pedestrian, motorcycle, truck, other, geom, source_hash)
		VALUES ('14524280', $1, 5, 7, 3, $2, 3, 4,
			$3, true, $4, false, false, false,
			ST_MakePoint($5, $6)::geography, $7)`
	for i, a := range accidents {
		hash := []byte{byte(i), byte(i >> 8), 0x5a}
		if _, err := pool.Exec(ctx, q, a.year, a.severity, a.bike, a.pedestrian, a.lon, a.lat, hash); err != nil {
			t.Fatalf("insert accident %d: %v", i, err)
		}
	}
}

func setup(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	pool := dbtest.Pool(t)
	t.Cleanup(func() { dbtest.Truncate(t, pool, "accidents", "hotspots") })
	return pool, context.Background()
}

// Roughly 33 m apart at this latitude — the same place as far as a road safety
// inspection is concerned.
const nearby = 0.0003

func TestAccidentsAtTheSamePlaceBecomeOneHotspot(t *testing.T) {
	pool, ctx := setup(t)
	insert(t,
		pool,
		accident{12.62, 50.79, 2, 2024, true, false},
		accident{12.62, 50.79 + nearby, 3, 2023, false, true},
	)

	result, err := scoring.RecomputeHotspots(ctx, pool)
	if err != nil {
		t.Fatalf("recompute: %v", err)
	}
	if result.Hotspots != 1 {
		t.Fatalf("produced %d hotspots for two accidents at one place", result.Hotspots)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT accident_count FROM hotspots").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("hotspot covers %d accidents, want 2", count)
	}
}

func TestAccidentsFarApartStayApart(t *testing.T) {
	pool, ctx := setup(t)
	insert(t,
		pool,
		accident{12.62, 50.79, 2, 2024, true, false},
		accident{12.62, 50.79 + nearby, 3, 2024, false, false},
		// ~220 m away, a different junction.
		accident{12.62, 50.792, 2, 2024, true, false},
		accident{12.62, 50.792 + nearby, 3, 2024, false, false},
	)

	result, err := scoring.RecomputeHotspots(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if result.Hotspots != 2 {
		t.Errorf("produced %d hotspots for two separate places, want 2", result.Hotspots)
	}
}

// One accident is an accident. Presenting it as a pattern is what gets a fact
// sheet dismissed at the first road safety inspection.
func TestALoneAccidentProducesNoHotspotRow(t *testing.T) {
	pool, ctx := setup(t)
	insert(t, pool, accident{12.62, 50.79, 1, 2025, true, false})

	result, err := scoring.RecomputeHotspots(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if result.Hotspots != 0 {
		t.Errorf("produced %d hotspots from a single accident", result.Hotspots)
	}
}

// The score in the database and the documented formula have to be the same
// thing. They are written twice — once in Go for the fact sheet's explanation,
// once in SQL for the clustering — and nothing but this test stops them from
// drifting apart.
func TestTheStoredScoreMatchesTheDocumentedFormula(t *testing.T) {
	pool, ctx := setup(t)
	accidents := []accident{
		{12.62, 50.79, 1, 2025, true, false},
		{12.62, 50.79 + nearby, 2, 2021, false, true},
		{12.62, 50.79 + 2*nearby, 3, 2017, false, false},
	}
	insert(t, pool, accidents...)

	if _, err := scoring.RecomputeHotspots(ctx, pool); err != nil {
		t.Fatal(err)
	}

	var want float64
	const reference = 2025 // the most recent reporting year in the fixture
	for _, a := range accidents {
		want += scoring.Score(a.severity, a.pedestrian || a.bike, a.year, reference)
	}

	var got float64
	if err := pool.QueryRow(ctx, "SELECT score FROM hotspots").Scan(&got); err != nil {
		t.Fatal(err)
	}
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("database scored %v, the documented formula gives %v", got, want)
	}
}

// The reference year comes from the data, not from the clock, so the same data
// gives the same number whenever somebody opens the page.
func TestTheReferenceYearComesFromTheData(t *testing.T) {
	pool, ctx := setup(t)
	insert(t,
		pool,
		accident{12.62, 50.79, 1, 2019, false, false},
		accident{12.62, 50.79 + nearby, 1, 2019, false, false},
	)

	result, err := scoring.RecomputeHotspots(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if result.Reference != 2019 {
		t.Fatalf("counted back from %d, want the most recent reporting year 2019", result.Reference)
	}

	// Both accidents sit in the reference year, so neither is discounted.
	var score float64
	if err := pool.QueryRow(ctx, "SELECT score FROM hotspots").Scan(&score); err != nil {
		t.Fatal(err)
	}
	if math.Abs(score-2*scoring.WeightFatal) > 1e-9 {
		t.Errorf("score = %v, want %v; the age was measured against the wrong year",
			score, 2*scoring.WeightFatal)
	}
}

func TestSeverityBreakdownCountsWhatHappened(t *testing.T) {
	pool, ctx := setup(t)
	insert(t,
		pool,
		accident{12.62, 50.79, 1, 2025, true, false},
		accident{12.62, 50.79 + nearby, 2, 2025, false, true},
		accident{12.62, 50.79 + 2*nearby, 3, 2025, true, true},
	)

	if _, err := scoring.RecomputeHotspots(ctx, pool); err != nil {
		t.Fatal(err)
	}

	var fatal, serious, slight, pedestrian, bike int
	const q = `SELECT (severity_breakdown->>'fatal')::int, (severity_breakdown->>'serious')::int,
		(severity_breakdown->>'slight')::int, (severity_breakdown->>'pedestrian')::int,
		(severity_breakdown->>'bike')::int FROM hotspots`
	if err := pool.QueryRow(ctx, q).Scan(&fatal, &serious, &slight, &pedestrian, &bike); err != nil {
		t.Fatal(err)
	}
	if fatal != 1 || serious != 1 || slight != 1 {
		t.Errorf("severity breakdown = %d fatal, %d serious, %d slight", fatal, serious, slight)
	}
	// The fact sheet needs to say "two of them on foot" without querying again.
	if pedestrian != 2 || bike != 2 {
		t.Errorf("involvement breakdown = %d pedestrian, %d bike; want 2 and 2", pedestrian, bike)
	}
}

func TestTheHotspotSitsBetweenItsAccidents(t *testing.T) {
	pool, ctx := setup(t)
	insert(t,
		pool,
		accident{12.62, 50.79, 2, 2025, true, false},
		accident{12.62, 50.79 + nearby, 2, 2025, true, false},
	)

	if _, err := scoring.RecomputeHotspots(ctx, pool); err != nil {
		t.Fatal(err)
	}

	var distance float64
	const q = `SELECT ST_Distance(geom, ST_MakePoint(12.62, $1)::geography) FROM hotspots`
	if err := pool.QueryRow(ctx, q, 50.79+nearby/2).Scan(&distance); err != nil {
		t.Fatal(err)
	}
	if distance > 5 {
		t.Errorf("centroid sits %.1f m from the midpoint of its two accidents", distance)
	}
}

func TestRecomputeReplacesTheTable(t *testing.T) {
	pool, ctx := setup(t)
	insert(t,
		pool,
		accident{12.62, 50.79, 2, 2025, true, false},
		accident{12.62, 50.79 + nearby, 2, 2025, true, false},
	)

	first, err := scoring.RecomputeHotspots(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	second, err := scoring.RecomputeHotspots(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if first.Hotspots != second.Hotspots {
		t.Errorf("recomputing changed the count from %d to %d", first.Hotspots, second.Hotspots)
	}

	var total int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM hotspots").Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != second.Hotspots {
		t.Errorf("table holds %d rows after two runs, want %d; the old rows were kept", total, second.Hotspots)
	}
}

// Hotspots describe the accidents that are imported. If those are gone, the
// hotspots have to go too rather than linger as an answer to nothing.
func TestEmptyingTheAccidentsEmptiesTheHotspots(t *testing.T) {
	pool, ctx := setup(t)
	insert(t,
		pool,
		accident{12.62, 50.79, 2, 2025, true, false},
		accident{12.62, 50.79 + nearby, 2, 2025, true, false},
	)
	if _, err := scoring.RecomputeHotspots(ctx, pool); err != nil {
		t.Fatal(err)
	}

	if _, err := pool.Exec(ctx, "TRUNCATE accidents"); err != nil {
		t.Fatal(err)
	}
	result, err := scoring.RecomputeHotspots(ctx, pool)
	if err != nil {
		t.Fatalf("recompute on empty accidents: %v", err)
	}
	if result.Hotspots != 0 {
		t.Errorf("produced %d hotspots without accidents", result.Hotspots)
	}

	var remaining int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM hotspots").Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Errorf("%d hotspots survived the accidents they described", remaining)
	}
}
