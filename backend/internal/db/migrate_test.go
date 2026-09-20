package db_test

import (
	"context"
	"testing"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/db"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/dbtest"
)

func TestMigrateIsIdempotent(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()

	var before int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM schema_migrations").Scan(&before); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if before == 0 {
		t.Fatal("no migrations were applied")
	}

	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	var after int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM schema_migrations").Scan(&after); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if after != before {
		t.Errorf("applying the migrations twice changed the recorded count: %d → %d", before, after)
	}
}

func TestSchemaHasEveryTable(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()

	want := []string{
		"accidents", "institutions", "infrastructure",
		"reports", "report_confirmations", "hotspots", "import_runs",
	}
	for _, table := range want {
		var exists bool
		const q = `SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1)`
		if err := pool.QueryRow(ctx, q, table).Scan(&exists); err != nil {
			t.Fatalf("look up %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s is missing", table)
		}
	}
}

func TestGeometryColumnsAreIndexed(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()

	// A geography column without a GIST index turns every radius query into a
	// sequential scan, which only shows up once the table is full.
	const q = `
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attname = 'geom'
		WHERE n.nspname = 'public' AND c.relkind = 'r'
		  AND NOT EXISTS (
			SELECT 1 FROM pg_index i
			JOIN pg_class ic ON ic.oid = i.indexrelid
			JOIN pg_am am ON am.oid = ic.relam
			WHERE i.indrelid = c.oid AND am.amname = 'gist'
			  AND a.attnum = ANY (i.indkey))`
	rows, err := pool.Query(ctx, q)
	if err != nil {
		t.Fatalf("query indexes: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatal(err)
		}
		t.Errorf("table %s has a geom column without a GIST index", table)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestAccidentsRejectDuplicateSourceHash(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	t.Cleanup(func() { dbtest.Truncate(t, pool, "accidents") })

	const insert = `
		INSERT INTO accidents (ags, year, month, hour, weekday, severity, kind, type,
			bike, car, pedestrian, motorcycle, truck, other, geom, source_hash)
		VALUES ('14524280', 2024, 5, 7, 3, 2, 3, 4,
			false, true, true, false, false, false,
			ST_MakePoint(12.62, 50.79)::geography, $1)`

	if _, err := pool.Exec(ctx, insert, []byte("same-row")); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := pool.Exec(ctx, insert, []byte("same-row")); err == nil {
		t.Error("the same source row was imported twice; the unique constraint did not hold")
	}
}

func TestReportsDefaultToPending(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	t.Cleanup(func() { dbtest.Truncate(t, pool, "reports") })

	var status string
	const q = `
		INSERT INTO reports (geom, category, description, submitter_hash)
		VALUES (ST_MakePoint(12.62, 50.79)::geography, 'speeding', 'test', $1)
		RETURNING status`
	if err := pool.QueryRow(ctx, q, []byte("hash")).Scan(&status); err != nil {
		t.Fatalf("insert report: %v", err)
	}
	// A report that is already public on arrival makes moderation pointless.
	if status != "pending" {
		t.Errorf("new report has status %q, want %q", status, "pending")
	}
}
