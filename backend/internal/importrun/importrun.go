// Package importrun records what an import did, including when it failed.
// Without it, a number on the map cannot be traced back to the data it came
// from.
package importrun

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Run struct {
	ID   int64
	pool *pgxpool.Pool
}

func Start(ctx context.Context, pool *pgxpool.Pool, source, detail string) (*Run, error) {
	const q = `INSERT INTO import_runs (source, detail) VALUES ($1, $2) RETURNING id`
	run := &Run{pool: pool}
	if err := pool.QueryRow(ctx, q, source, detail).Scan(&run.ID); err != nil {
		return nil, fmt.Errorf("record import run: %w", err)
	}
	return run, nil
}

// Finish closes the run. A failing import is recorded as failed rather than
// left hanging in "running", which is what a crashed run looks like.
func (r *Run) Finish(ctx context.Context, read, written int, checksum string, cause error) error {
	status, message := "succeeded", ""
	if cause != nil {
		status, message = "failed", cause.Error()
	}
	const q = `
		UPDATE import_runs
		SET status = $2, finished_at = now(), rows_read = $3, rows_written = $4,
		    checksum = $5, message = NULLIF($6, '')
		WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, r.ID, status, read, written, checksum, message)
	return err
}
