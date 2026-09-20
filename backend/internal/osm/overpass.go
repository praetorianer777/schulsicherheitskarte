// Package osm fills the institution and infrastructure tables from
// OpenStreetMap (ODbL, attribution "© OpenStreetMap contributors").
package osm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const DefaultEndpoint = "https://overpass-api.de/api/interpreter"

// Overpass is a volunteer-run service. A named agent is what its operators ask
// for and what lets them see who is causing load.
const userAgent = "schulsicherheitskarte/0.1 (+https://github.com/praetorianer777/schulsicherheitskarte)"

type Client struct {
	Endpoint string
	HTTP     *http.Client
	CacheDir string

	// Attempts bounds the retries on the status codes Overpass uses to say it
	// is busy. Zero means the default.
	Attempts int

	// Sleep is replaced in tests so backoff costs no wall clock.
	Sleep func(time.Duration)
}

func NewClient(cacheDir string) *Client {
	endpoint := os.Getenv("OVERPASS_URL")
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	return &Client{
		Endpoint: endpoint,
		// Overpass answers a county-sized query in tens of seconds and is
		// allowed to take its time.
		HTTP:     &http.Client{Timeout: 5 * time.Minute},
		CacheDir: cacheDir,
		Attempts: 5,
		Sleep:    time.Sleep,
	}
}

// ErrBusy is what a caller sees when Overpass stayed busy for every attempt.
var ErrBusy = errors.New("overpass is busy")

// Query runs one Overpass QL query and returns the raw response. The raw bytes
// are kept under name so a parser change can be replayed against exactly what
// the server said, without querying a volunteer service again.
func (c *Client) Query(ctx context.Context, name, query string) ([]byte, error) {
	var lastErr error
	attempts := c.Attempts
	if attempts <= 0 {
		attempts = 5
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		body, retryAfter, err := c.once(ctx, query)
		if err == nil {
			if err := c.store(name, body); err != nil {
				return nil, err
			}
			return body, nil
		}
		lastErr = err
		if retryAfter < 0 {
			return nil, err // not a busy answer; retrying would not help
		}
		if attempt == attempts {
			break
		}

		// 429 and 504 are how Overpass says "not now". Both are expected under
		// load and both are worth waiting out.
		wait := time.Duration(1<<attempt) * time.Second
		if retryAfter > wait {
			wait = retryAfter
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		c.Sleep(wait)
	}
	return nil, fmt.Errorf("%w after %d attempts: %v", ErrBusy, attempts, lastErr)
}

// once returns the body, or the delay the server asked for. A negative delay
// means the failure is not worth retrying.
func (c *Client) once(ctx context.Context, query string) ([]byte, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint,
		strings.NewReader(url.Values{"data": {query}}.Encode()))
	if err != nil {
		return nil, -1, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err // a transport error is worth another try
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, 0, err
		}
		return body, 0, nil
	case http.StatusTooManyRequests, http.StatusGatewayTimeout, http.StatusServiceUnavailable:
		return nil, retryAfter(resp.Header.Get("Retry-After")), fmt.Errorf("overpass answered %s", resp.Status)
	default:
		excerpt, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return nil, -1, fmt.Errorf("overpass answered %s: %s", resp.Status, strings.TrimSpace(string(excerpt)))
	}
}

func retryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return 0
}

func (c *Client) store(name string, body []byte) error {
	if c.CacheDir == "" {
		return nil
	}
	if err := os.MkdirAll(c.CacheDir, 0o755); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}
	return os.WriteFile(filepath.Join(c.CacheDir, name+".json"), body, 0o644)
}

// Cached reads a previously stored response, so development and debugging do
// not hit the service for data that has not changed.
func (c *Client) Cached(name string) ([]byte, error) {
	if c.CacheDir == "" {
		return nil, os.ErrNotExist
	}
	return os.ReadFile(filepath.Join(c.CacheDir, name+".json"))
}
