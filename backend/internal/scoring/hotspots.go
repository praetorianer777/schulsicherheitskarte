package scoring

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Result struct {
	Accidents int
	Hotspots  int
	Reference int // the reporting year the recency weighting counts back from
}

// The projection the clustering runs in. Accidents are stored as geography in
// WGS84, which is right for the radius queries that dominate, but clustering
// needs a plane and a radius in metres. UTM zone 32N covers the whole country.
const metricSRID = 25832

// RecomputeHotspots rebuilds the hotspot table from the accidents currently
// imported.
//
// The table is replaced rather than updated: a hotspot is a statement about the
// data as a whole, and an accident from a new reporting year can merge two
// circles or move a centre. Patching rows would leave a mixture of answers from
// different inputs.
func RecomputeHotspots(ctx context.Context, pool *pgxpool.Pool) (*Result, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	accidents, reference, err := loadAccidents(ctx, tx)
	if err != nil {
		return nil, err
	}

	// Emptying the table is right even with nothing to cluster: the previous
	// hotspots described data that is no longer there.
	if _, err := tx.Exec(ctx, "TRUNCATE hotspots RESTART IDENTITY"); err != nil {
		return nil, fmt.Errorf("clear hotspots: %w", err)
	}

	hotspots := Cluster(accidents, reference)
	if err := writeHotspots(ctx, tx, hotspots); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &Result{Accidents: len(accidents), Hotspots: len(hotspots), Reference: reference}, nil
}

func loadAccidents(ctx context.Context, tx pgx.Tx) ([]Accident, int, error) {
	const q = `
		SELECT id,
		       ST_X(ST_Transform(geom::geometry, $1::integer)),
		       ST_Y(ST_Transform(geom::geometry, $1::integer)),
		       severity, year, pedestrian, bike
		FROM accidents`
	rows, err := tx.Query(ctx, q, metricSRID)
	if err != nil {
		return nil, 0, fmt.Errorf("read accidents: %w", err)
	}
	defer rows.Close()

	var accidents []Accident
	reference := 0
	for rows.Next() {
		var a Accident
		if err := rows.Scan(&a.ID, &a.X, &a.Y, &a.Severity, &a.Year, &a.Pedestrian, &a.Bike); err != nil {
			return nil, 0, err
		}
		if a.Year > reference {
			reference = a.Year
		}
		accidents = append(accidents, a)
	}
	return accidents, reference, rows.Err()
}

func writeHotspots(ctx context.Context, tx pgx.Tx, hotspots []Hotspot) error {
	if len(hotspots) == 0 {
		return nil
	}

	const staging = `
		CREATE TEMP TABLE staging_hotspots (
			x double precision, y double precision, accident_count integer,
			score double precision, severity_breakdown jsonb,
			first_year smallint, last_year smallint
		) ON COMMIT DROP`
	if _, err := tx.Exec(ctx, staging); err != nil {
		return fmt.Errorf("create staging table: %w", err)
	}

	rows := make([][]any, 0, len(hotspots))
	for _, h := range hotspots {
		breakdown, err := json.Marshal(map[string]int{
			"fatal": h.Fatal, "serious": h.Serious, "slight": h.Slight,
			"pedestrian": h.Ped, "bike": h.Bike,
		})
		if err != nil {
			return err
		}
		rows = append(rows, []any{h.X, h.Y, h.Count, h.Score, breakdown, h.FirstYear, h.LastYear})
	}

	columns := []string{"x", "y", "accident_count", "score", "severity_breakdown", "first_year", "last_year"}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"staging_hotspots"}, columns, pgx.CopyFromRows(rows)); err != nil {
		return fmt.Errorf("stage hotspots: %w", err)
	}

	const insert = `
		INSERT INTO hotspots (geom, accident_count, score, severity_breakdown, first_year, last_year)
		SELECT ST_Transform(ST_SetSRID(ST_MakePoint(x, y), $1::integer), 4326)::geography,
		       accident_count, score, severity_breakdown, first_year, last_year
		FROM staging_hotspots`
	if _, err := tx.Exec(ctx, insert, metricSRID); err != nil {
		return fmt.Errorf("write hotspots: %w", err)
	}
	return nil
}
