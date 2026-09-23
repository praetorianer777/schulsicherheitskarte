package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/config"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/db"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/importrun"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/osm"
)

func importOSM(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("osm", flag.ExitOnError)
	regionPath := fs.String("regions", "regions.yaml", "path to the region configuration")
	cacheDir := fs.String("cache", "data/cache/osm", "where raw Overpass responses are kept")
	offline := fs.Bool("offline", false, "replay the cached responses instead of querying Overpass")
	if err := fs.Parse(args); err != nil {
		return err
	}

	regions, err := config.Load(*regionPath)
	if err != nil {
		return err
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL is not set")
	}
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	client := osm.NewClient(*cacheDir)
	for _, region := range regions.Regions {
		box := osm.BBox(region.BBox)
		// Before the institutions, so that each one is written with its town.
		if err := importBoundaries(ctx, pool, client, region.Name, box, *offline); err != nil {
			return err
		}
		if err := importInstitutions(ctx, pool, client, region.Name, box, *offline); err != nil {
			return err
		}
		if err := importInfrastructure(ctx, pool, client, region.Name, box, *offline); err != nil {
			return err
		}
	}
	return nil
}

func importInstitutions(ctx context.Context, pool *pgxpool.Pool, client *osm.Client, region string, box osm.BBox, offline bool) error {
	body, err := fetch(ctx, client, osm.QueryInstitutions, osm.InstitutionsQuery(box), offline)
	if err != nil {
		return err
	}

	run, err := importrun.Start(ctx, pool, "osm", region+"/"+osm.QueryInstitutions)
	if err != nil {
		return err
	}

	items, err := osm.ParseInstitutions(body)
	if err != nil {
		_ = run.Finish(ctx, 0, 0, "", err)
		return err
	}
	result, err := osm.ImportInstitutions(ctx, pool, items, box)
	if err != nil {
		_ = run.Finish(ctx, len(items), 0, "", err)
		return err
	}
	if err := run.Finish(ctx, len(items), result.Written, "", nil); err != nil {
		return err
	}
	log.Printf("%s: %d schools and kindergartens written, %d gone from OpenStreetMap and removed",
		region, result.Written, result.Pruned)

	coverage, err := osm.CountTownCoverage(ctx, pool, box)
	if err != nil {
		return err
	}
	log.Printf("%s: %d of %d found by their town — %d by addr:city, %d only through the municipal boundaries; %d still without a town",
		region, coverage.Address+coverage.Boundary, coverage.Total, coverage.Address, coverage.Boundary, coverage.None)
	return nil
}

func importBoundaries(ctx context.Context, pool *pgxpool.Pool, client *osm.Client, region string, box osm.BBox, offline bool) error {
	body, err := fetch(ctx, client, osm.QueryBoundaries, osm.BoundariesQuery(box), offline)
	if err != nil {
		return err
	}

	run, err := importrun.Start(ctx, pool, "osm", region+"/"+osm.QueryBoundaries)
	if err != nil {
		return err
	}

	items, err := osm.ParseBoundaries(body)
	if err != nil {
		_ = run.Finish(ctx, 0, 0, "", err)
		return err
	}
	result, err := osm.ImportBoundaries(ctx, pool, items, box)
	if err != nil {
		_ = run.Finish(ctx, len(items), 0, "", err)
		return err
	}
	if err := run.Finish(ctx, len(items), result.Written, "", nil); err != nil {
		return err
	}
	log.Printf("%s: %d municipal and district boundaries written, %d removed, %d skipped because their ways do not close",
		region, result.Written, result.Pruned, result.Broken)
	return nil
}

func importInfrastructure(ctx context.Context, pool *pgxpool.Pool, client *osm.Client, region string, box osm.BBox, offline bool) error {
	// Each query prunes only the kinds it fetched, so a failure in one leaves
	// the other's rows alone.
	queries := []struct {
		name  string
		query string
		kinds []string
	}{
		{osm.QueryCrossings, osm.CrossingsQuery(box), []string{"crossing", "traffic_signals", "traffic_calming"}},
		{osm.QuerySpeedLimits, osm.SpeedLimitsQuery(box), []string{"speed_limit"}},
	}

	for _, q := range queries {
		body, err := fetch(ctx, client, q.name, q.query, offline)
		if err != nil {
			return err
		}

		run, err := importrun.Start(ctx, pool, "osm", region+"/"+q.name)
		if err != nil {
			return err
		}

		items, err := osm.ParseInfrastructure(body)
		if err != nil {
			_ = run.Finish(ctx, 0, 0, "", err)
			return err
		}
		result, err := osm.ImportInfrastructure(ctx, pool, items, box, q.kinds)
		if err != nil {
			_ = run.Finish(ctx, len(items), 0, "", err)
			return err
		}
		if err := run.Finish(ctx, len(items), result.Written, "", nil); err != nil {
			return err
		}
		log.Printf("%s/%s: %d written, %d removed", region, q.name, result.Written, result.Pruned)
	}
	return nil
}

func fetch(ctx context.Context, client *osm.Client, name, query string, offline bool) ([]byte, error) {
	if offline {
		body, err := client.Cached(name)
		if err != nil {
			return nil, fmt.Errorf("no cached response for %s: %w", name, err)
		}
		return body, nil
	}
	body, err := client.Query(ctx, name, query)
	if errors.Is(err, osm.ErrBusy) {
		return nil, fmt.Errorf("%w — try again later, or run with -offline to replay the cached response", err)
	}
	return body, err
}
