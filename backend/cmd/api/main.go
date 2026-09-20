// Command api serves the read side of the map.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/api"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/db"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/reports"
)

func main() {
	// The image has no shell and no curl, so the binary probes itself. A health
	// check that needs a second tool in the image is a second thing to break.
	healthcheck := flag.Bool("healthcheck", false, "probe a running instance and exit")
	flag.Parse()
	if *healthcheck {
		os.Exit(probe())
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		slog.Error("DATABASE_URL is not set")
		os.Exit(1)
	}
	address := os.Getenv("LISTEN_ADDRESS")
	if address == "" {
		address = ":8080"
	}

	pool, err := db.Open(ctx, dsn)
	if err != nil {
		slog.Error("database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	handler := api.New(pool, api.Options{
		TrustProxyHeaders: os.Getenv("TRUST_PROXY_HEADERS") == "true",
		ModerationToken:   os.Getenv("MODERATION_TOKEN"),
	}).WithReports(reports.NewStore(pool, reportSalt())).Routes()

	server := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("listening", "address", address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("serve", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	// Requests in flight get a moment to finish; a container that is being
	// replaced should not turn an open request into an error page.
	shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		slog.Error("shutdown", "error", err)
	}
}

// reportSalt keys the submitter fingerprint. Without a configured one a random
// salt is generated: a fixed default would be no salt at all, and refusing to
// start would take the whole site down over a feature that is not essential to
// it. The cost is that the rate limit starts over after a restart, which is
// why it says so.
func reportSalt() []byte {
	if configured := os.Getenv("REPORT_SALT"); configured != "" {
		return []byte(configured)
	}
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		slog.Error("generate report salt", "error", err)
		os.Exit(1)
	}
	slog.Warn("REPORT_SALT is not set; a random one was generated, so the reporting rate limit starts over on every restart")
	return salt
}

func probe() int {
	address := os.Getenv("LISTEN_ADDRESS")
	if address == "" {
		address = ":8080"
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		fmt.Fprintf(os.Stderr, "LISTEN_ADDRESS %q: %v\n", address, err)
		return 1
	}
	if host == "" {
		host = "127.0.0.1"
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthz answered %s\n", resp.Status)
		return 1
	}
	return 0
}
