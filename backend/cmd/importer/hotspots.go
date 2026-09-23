package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/db"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/importrun"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/scoring"
)

func recomputeHotspots(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("hotspots", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
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

	run, err := importrun.Start(ctx, pool, "hotspots", "")
	if err != nil {
		return err
	}

	result, err := scoring.RecomputeHotspots(ctx, pool)
	if err != nil {
		_ = run.Finish(ctx, 0, 0, "", err)
		return err
	}
	if err := run.Finish(ctx, result.Accidents, result.Hotspots, "", nil); err != nil {
		return err
	}

	log.Printf("%d accidents clustered into %d hotspots, counting back from reporting year %d",
		result.Accidents, result.Hotspots, result.Reference)
	log.Printf("%d institutions: accidents within %d m counted for the overview",
		result.Institutions, scoring.NearbyRadiusMetres)
	return nil
}
