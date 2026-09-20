// Command migrate applies the schema migrations and exits. It runs as a job in
// the compose stack before the API starts.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/db"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	pool, err := db.Open(ctx, dsn)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Print("migrations applied")
}
