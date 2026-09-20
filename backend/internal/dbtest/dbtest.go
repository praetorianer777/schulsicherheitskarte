// Package dbtest hands tests a migrated database.
//
// Tests that need one skip when TEST_DATABASE_URL is unset, so `go test ./...`
// stays usable without Docker; run-tests.sh starts the container and sets it.
package dbtest

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/db"
)

var (
	once     sync.Once
	shared   *pgxpool.Pool
	setupErr error
)

// Pool returns a pool against the migrated test database. The migrations run
// once per test binary; every caller shares the same database, so tests that
// write have to clean up after themselves with Truncate.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; run ./run-tests.sh for the database tests")
	}

	once.Do(func() {
		ctx := context.Background()
		shared, setupErr = db.Open(ctx, dsn)
		if setupErr != nil {
			return
		}
		setupErr = db.Migrate(ctx, shared)
	})
	if setupErr != nil {
		t.Fatalf("prepare test database: %v", setupErr)
	}
	return shared
}

// Truncate empties the named tables and their dependants. Registering it with
// t.Cleanup keeps a failing test from poisoning the next one.
func Truncate(t *testing.T, pool *pgxpool.Pool, tables ...string) {
	t.Helper()
	if len(tables) == 0 {
		return
	}
	stmt := "TRUNCATE " + tables[0]
	for _, table := range tables[1:] {
		stmt += ", " + table
	}
	stmt += " RESTART IDENTITY CASCADE"
	if _, err := pool.Exec(context.Background(), stmt); err != nil {
		t.Fatalf("truncate %v: %v", tables, err)
	}
}
