// Package dbtest hands tests a migrated database of their own.
//
// Tests that need one skip when TEST_DATABASE_URL is unset, so `go test ./...`
// stays usable without Docker; run-tests.sh starts the container and sets it.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/db"
)

var (
	once     sync.Once
	shared   *pgxpool.Pool
	setupErr error
)

// Pool returns a pool against a freshly created, migrated database, one per
// test binary.
//
// `go test ./...` runs the packages in parallel against the same server, so a
// shared database would let one package's cleanup truncate a table another
// package is in the middle of using — a failure that only shows up under load
// and never in a single-package run.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; run ./run-tests.sh for the database tests")
	}

	once.Do(func() { shared, setupErr = create(context.Background(), dsn) })
	if setupErr != nil {
		t.Fatalf("prepare test database: %v", setupErr)
	}
	return shared
}

func create(ctx context.Context, adminDSN string) (*pgxpool.Pool, error) {
	name, err := uniqueName()
	if err != nil {
		return nil, err
	}

	admin, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer admin.Close(ctx)

	// The name is generated here and contains nothing but letters, digits and
	// underscores; CREATE DATABASE takes no parameters.
	if _, err := admin.Exec(ctx, `CREATE DATABASE "`+name+`"`); err != nil {
		return nil, fmt.Errorf("create database %s: %w", name, err)
	}

	pool, err := db.Open(ctx, withDatabase(adminDSN, name))
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func uniqueName() (string, error) {
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	return "ssk_test_" + hex.EncodeToString(suffix[:]), nil
}

func withDatabase(dsn, name string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	u.Path = "/" + name
	return u.String()
}

// Truncate empties the named tables and their dependants. Registering it with
// t.Cleanup keeps a failing test from poisoning the next one in the same
// package.
func Truncate(t *testing.T, pool *pgxpool.Pool, tables ...string) {
	t.Helper()
	if len(tables) == 0 {
		return
	}
	stmt := "TRUNCATE " + strings.Join(tables, ", ") + " RESTART IDENTITY CASCADE"
	if _, err := pool.Exec(context.Background(), stmt); err != nil {
		t.Fatalf("truncate %v: %v", tables, err)
	}
}
