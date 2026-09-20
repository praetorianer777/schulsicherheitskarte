// Command importer fills the database from the open data sources.
//
//	importer accidents -years 2016-2025
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/accidents"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/config"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/db"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/importrun"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		usage()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch os.Args[1] {
	case "accidents":
		if err := importAccidents(ctx, os.Args[2:]); err != nil {
			log.Fatalf("accidents: %v", err)
		}
	case "osm":
		if err := importOSM(ctx, os.Args[2:]); err != nil {
			log.Fatalf("osm: %v", err)
		}
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  importer accidents [-years 2016-2025] [-regions regions.yaml] [-cache data/cache]")
	fmt.Fprintln(os.Stderr, "  importer osm [-regions regions.yaml] [-cache data/cache/osm] [-offline]")
	os.Exit(2)
}

func importAccidents(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("accidents", flag.ExitOnError)
	yearSpec := fs.String("years", "2016-2025", "reporting year or range, e.g. 2024 or 2016-2025")
	regionPath := fs.String("regions", "regions.yaml", "path to the region configuration")
	cacheDir := fs.String("cache", "data/cache", "where downloaded archives are kept")
	if err := fs.Parse(args); err != nil {
		return err
	}

	years, err := parseYears(*yearSpec)
	if err != nil {
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

	client := accidents.DefaultClient()
	for _, year := range years {
		if err := importYear(ctx, pool, client, year, *cacheDir, regions); err != nil {
			return err
		}
	}
	return nil
}

func importYear(ctx context.Context, pool *pgxpool.Pool, client *http.Client, year int, cacheDir string, regions *config.Config) error {
	run, err := importrun.Start(ctx, pool, "accidents", strconv.Itoa(year))
	if err != nil {
		return err
	}

	download, err := accidents.Fetch(ctx, client, year, cacheDir)
	if err != nil {
		_ = run.Finish(ctx, 0, 0, "", err)
		return err
	}
	if download.Cached {
		log.Printf("%d: archive unchanged, reading the cached copy", year)
	}

	result, err := accidents.ImportArchive(ctx, pool, download.Path, regions)
	if err != nil {
		_ = run.Finish(ctx, 0, 0, download.Checksum, err)
		return err
	}

	if err := run.Finish(ctx, result.Read, result.Written, download.Checksum, nil); err != nil {
		return err
	}
	log.Printf("%d: %d rows read, %d outside the configured regions, %d newly imported",
		year, result.Read, result.Skipped, result.Written)
	return nil
}

func parseYears(spec string) ([]int, error) {
	from, to, found := strings.Cut(spec, "-")
	start, err := strconv.Atoi(strings.TrimSpace(from))
	if err != nil {
		return nil, fmt.Errorf("years %q: %q is not a year", spec, from)
	}
	end := start
	if found {
		if end, err = strconv.Atoi(strings.TrimSpace(to)); err != nil {
			return nil, fmt.Errorf("years %q: %q is not a year", spec, to)
		}
	}
	if end < start {
		return nil, fmt.Errorf("years %q: the range ends before it starts", spec)
	}
	years := make([]int, 0, end-start+1)
	for y := start; y <= end; y++ {
		years = append(years, y)
	}
	return years, nil
}
