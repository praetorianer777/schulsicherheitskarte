package accidents

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Matcher decides whether an accident belongs to a configured region.
type Matcher interface {
	Matches(ags string) bool
}

type Result struct {
	Read    int
	Written int
	Skipped int // outside the configured regions
}

// ImportArchive reads the CSV inside a downloaded archive and upserts every
// accident belonging to the configured regions.
func ImportArchive(ctx context.Context, pool *pgxpool.Pool, path string, regions Matcher) (*Result, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer zr.Close()

	// The 2016 archive ships its data as .txt, and every archive carries a
	// schema.ini beside it. Picking the largest data file is what survives both.
	var entry *zip.File
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		if !strings.HasSuffix(name, ".csv") && !strings.HasSuffix(name, ".txt") {
			continue
		}
		if entry == nil || f.UncompressedSize64 > entry.UncompressedSize64 {
			entry = f
		}
	}
	if entry == nil {
		return nil, fmt.Errorf("archive %s contains no .csv or .txt data file", path)
	}

	rc, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", entry.Name, err)
	}
	defer rc.Close()

	return Import(ctx, pool, rc, regions)
}

// Import reads accident rows and writes those inside the configured regions.
// Rows already present are left untouched, so a repeated import produces no
// duplicates and no churn.
func Import(ctx context.Context, pool *pgxpool.Pool, r io.Reader, regions Matcher) (*Result, error) {
	reader, err := NewReader(r)
	if err != nil {
		return nil, err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	// Staged first so that the whole year lands in one statement; a row-by-row
	// upsert of a national file is minutes of round trips.
	const createStaging = `
		CREATE TEMP TABLE staging_accidents (
			ags text, year smallint, month smallint, hour smallint, weekday smallint,
			severity smallint, kind smallint, type smallint,
			light smallint, road_condition smallint,
			bike boolean, car boolean, pedestrian boolean,
			motorcycle boolean, truck boolean, other boolean,
			lon double precision, lat double precision,
			source_hash bytea
		) ON COMMIT DROP`
	if _, err := tx.Exec(ctx, createStaging); err != nil {
		return nil, fmt.Errorf("create staging table: %w", err)
	}

	result := &Result{}
	var rows [][]any
	for {
		rec, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		result.Read++

		if !regions.Matches(rec.AGS) {
			result.Skipped++
			continue
		}
		rows = append(rows, []any{
			rec.AGS, rec.Year, rec.Month, rec.Hour, rec.Weekday,
			rec.Severity, rec.Kind, rec.Type, rec.Light, rec.RoadCondition,
			rec.Bike, rec.Car, rec.Pedestrian, rec.Motorcycle, rec.Truck, rec.Other,
			rec.Lon, rec.Lat, rec.SourceHash,
		})
	}

	if len(rows) > 0 {
		columns := []string{
			"ags", "year", "month", "hour", "weekday",
			"severity", "kind", "type", "light", "road_condition",
			"bike", "car", "pedestrian", "motorcycle", "truck", "other",
			"lon", "lat", "source_hash",
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"staging_accidents"}, columns, pgx.CopyFromRows(rows)); err != nil {
			return nil, fmt.Errorf("stage rows: %w", err)
		}
	}

	const upsert = `
		INSERT INTO accidents (
			ags, year, month, hour, weekday, severity, kind, type,
			light, road_condition, bike, car, pedestrian, motorcycle, truck, other,
			geom, source_hash)
		SELECT ags, year, month, hour, weekday, severity, kind, type,
			light, road_condition, bike, car, pedestrian, motorcycle, truck, other,
			ST_SetSRID(ST_MakePoint(lon, lat), 4326)::geography, source_hash
		FROM (
			-- A file can carry the same accident twice; ON CONFLICT cannot
			-- resolve a conflict between two rows of the same statement.
			SELECT DISTINCT ON (source_hash) * FROM staging_accidents
		) s
		ON CONFLICT (source_hash) DO NOTHING`
	tag, err := tx.Exec(ctx, upsert)
	if err != nil {
		return nil, fmt.Errorf("upsert accidents: %w", err)
	}
	result.Written = int(tag.RowsAffected())

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}
