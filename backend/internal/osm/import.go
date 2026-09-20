package osm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Result struct {
	Written int
	Pruned  int
}

// ImportInstitutions writes schools and kindergartens and removes the ones that
// have disappeared from OpenStreetMap inside the imported box.
func ImportInstitutions(ctx context.Context, pool *pgxpool.Pool, items []Institution, box BBox) (*Result, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	// Taken before the writes so that a row touched by this import is never
	// mistaken for a stale one.
	start, err := transactionStart(ctx, tx)
	if err != nil {
		return nil, err
	}

	const staging = `
		CREATE TEMP TABLE staging_institutions (
			osm_type text, osm_id bigint, kind text, name text,
			school_type text, lon double precision, lat double precision, tags jsonb
		) ON COMMIT DROP`
	if _, err := tx.Exec(ctx, staging); err != nil {
		return nil, fmt.Errorf("create staging table: %w", err)
	}

	rows := make([][]any, 0, len(items))
	for _, item := range items {
		tags, err := json.Marshal(item.Tags)
		if err != nil {
			return nil, err
		}
		rows = append(rows, []any{
			item.OSMType, item.OSMID, item.Kind, nullable(item.Name),
			nullable(item.SchoolType), item.Point.Lon, item.Point.Lat, tags,
		})
	}
	if len(rows) > 0 {
		columns := []string{"osm_type", "osm_id", "kind", "name", "school_type", "lon", "lat", "tags"}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"staging_institutions"}, columns, pgx.CopyFromRows(rows)); err != nil {
			return nil, fmt.Errorf("stage institutions: %w", err)
		}
	}

	const upsert = `
		INSERT INTO institutions (osm_type, osm_id, kind, name, school_type, geom, tags, updated_at)
		SELECT osm_type, osm_id, kind, name, school_type,
		       ST_SetSRID(ST_MakePoint(lon, lat), 4326)::geography, tags, now()
		FROM (SELECT DISTINCT ON (osm_type, osm_id) * FROM staging_institutions) s
		ON CONFLICT (osm_type, osm_id) DO UPDATE SET
			kind = EXCLUDED.kind, name = EXCLUDED.name,
			school_type = EXCLUDED.school_type, geom = EXCLUDED.geom,
			tags = EXCLUDED.tags, updated_at = now()`
	tag, err := tx.Exec(ctx, upsert)
	if err != nil {
		return nil, fmt.Errorf("upsert institutions: %w", err)
	}

	pruned, err := prune(ctx, tx, "institutions", start, box)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &Result{Written: int(tag.RowsAffected()), Pruned: pruned}, nil
}

// ImportInfrastructure writes crossings, signals, traffic calming and speed
// limits.
func ImportInfrastructure(ctx context.Context, pool *pgxpool.Pool, items []Infrastructure, box BBox, kinds []string) (*Result, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	start, err := transactionStart(ctx, tx)
	if err != nil {
		return nil, err
	}

	const staging = `
		CREATE TEMP TABLE staging_infrastructure (
			osm_type text, osm_id bigint, kind text, wkt text, tags jsonb
		) ON COMMIT DROP`
	if _, err := tx.Exec(ctx, staging); err != nil {
		return nil, fmt.Errorf("create staging table: %w", err)
	}

	rows := make([][]any, 0, len(items))
	for _, item := range items {
		tags, err := json.Marshal(item.Tags)
		if err != nil {
			return nil, err
		}
		rows = append(rows, []any{item.OSMType, item.OSMID, item.Kind, wkt(item.Geometry), tags})
	}
	if len(rows) > 0 {
		columns := []string{"osm_type", "osm_id", "kind", "wkt", "tags"}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"staging_infrastructure"}, columns, pgx.CopyFromRows(rows)); err != nil {
			return nil, fmt.Errorf("stage infrastructure: %w", err)
		}
	}

	const upsert = `
		INSERT INTO infrastructure (osm_type, osm_id, kind, geom, tags, updated_at)
		SELECT osm_type, osm_id, kind, ST_GeogFromText('SRID=4326;' || wkt), tags, now()
		FROM (SELECT DISTINCT ON (osm_type, osm_id, kind) * FROM staging_infrastructure) s
		ON CONFLICT (osm_type, osm_id, kind) DO UPDATE SET
			geom = EXCLUDED.geom, tags = EXCLUDED.tags, updated_at = now()`
	tag, err := tx.Exec(ctx, upsert)
	if err != nil {
		return nil, fmt.Errorf("upsert infrastructure: %w", err)
	}

	// Only the kinds this import actually asked for may be pruned; a run that
	// fetched crossings must not delete the speed limits of an earlier run.
	pruned, err := prune(ctx, tx, "infrastructure", start, box, kinds...)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &Result{Written: int(tag.RowsAffected()), Pruned: pruned}, nil
}

// prune removes rows inside the imported box that this import did not touch.
// Without it, an object deleted in OpenStreetMap — a crossing that was removed
// because it was never built — stays on the map forever.
func prune(ctx context.Context, tx pgx.Tx, table string, start time.Time, box BBox, kinds ...string) (int, error) {
	q := `DELETE FROM ` + table + `
		WHERE updated_at < $1
		  AND ST_Intersects(geom, ST_MakeEnvelope($2, $3, $4, $5, 4326)::geography)`
	args := []any{start, box[0], box[1], box[2], box[3]}
	if len(kinds) > 0 {
		q += ` AND kind = ANY($6)`
		args = append(args, kinds)
	}
	tag, err := tx.Exec(ctx, q, args...)
	if err != nil {
		return 0, fmt.Errorf("prune %s: %w", table, err)
	}
	return int(tag.RowsAffected()), nil
}

// now() is fixed for the whole transaction, so reading it here gives a cutoff
// the writes below are guaranteed to be at or after.
func transactionStart(ctx context.Context, tx pgx.Tx) (time.Time, error) {
	var t time.Time
	if err := tx.QueryRow(ctx, "SELECT now()").Scan(&t); err != nil {
		return t, err
	}
	return t, nil
}

func wkt(points []Point) string {
	if len(points) == 1 {
		return fmt.Sprintf("POINT(%.7f %.7f)", points[0].Lon, points[0].Lat)
	}
	parts := make([]string, 0, len(points))
	for _, p := range points {
		parts = append(parts, fmt.Sprintf("%.7f %.7f", p.Lon, p.Lat))
	}
	return "LINESTRING(" + strings.Join(parts, ",") + ")"
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
