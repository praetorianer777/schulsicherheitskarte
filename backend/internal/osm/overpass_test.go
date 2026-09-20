package osm_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/osm"
)

func testClient(t *testing.T, handler http.HandlerFunc) (*osm.Client, *[]time.Duration) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	var slept []time.Duration
	client := osm.NewClient(t.TempDir())
	client.Endpoint = server.URL
	client.HTTP = server.Client()
	client.Sleep = func(d time.Duration) { slept = append(slept, d) }
	return client, &slept
}

// 429 is how Overpass says "not now". Giving up on the first one would mean a
// failed import whenever somebody else is running a big query.
func TestRetriesWhileOverpassIsBusy(t *testing.T) {
	var calls int
	client, slept := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"elements":[]}`))
	})

	body, err := client.Query(context.Background(), "test", "out;")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if string(body) != `{"elements":[]}` {
		t.Errorf("body = %q", body)
	}
	if calls != 3 {
		t.Errorf("made %d calls, want 3", calls)
	}
	if len(*slept) != 2 {
		t.Errorf("waited %d times between 3 attempts, want 2", len(*slept))
	}
}

func TestRespectsRetryAfter(t *testing.T) {
	var calls int
	client, slept := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "42")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"elements":[]}`))
	})

	if _, err := client.Query(context.Background(), "test", "out;"); err != nil {
		t.Fatal(err)
	}
	if len(*slept) != 1 || (*slept)[0] != 42*time.Second {
		t.Errorf("waited %v, want the 42 s the server asked for", *slept)
	}
}

func TestGivesUpAfterTheLastAttempt(t *testing.T) {
	var calls int
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusGatewayTimeout)
	})
	client.Attempts = 3

	_, err := client.Query(context.Background(), "test", "out;")
	if !errors.Is(err, osm.ErrBusy) {
		t.Fatalf("error = %v, want ErrBusy", err)
	}
	if calls != 3 {
		t.Errorf("made %d attempts, want 3", calls)
	}
}

// A malformed query returns 400 and will return 400 again; retrying only adds
// load to a volunteer service.
func TestDoesNotRetryARejectedQuery(t *testing.T) {
	var calls int
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("line 1: parse error"))
	})

	_, err := client.Query(context.Background(), "test", "nonsense")
	if err == nil {
		t.Fatal("a rejected query was reported as success")
	}
	if calls != 1 {
		t.Errorf("made %d attempts for a rejected query, want 1", calls)
	}
	// The server's own message is the only clue to what was wrong with the query.
	if !strings.Contains(err.Error(), "parse error") {
		t.Errorf("error hides what the server said: %v", err)
	}
}

// The raw answer is kept so a parser change can be replayed without querying a
// volunteer service again.
func TestStoresTheRawResponse(t *testing.T) {
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"elements":[{"type":"node"}]}`))
	})

	if _, err := client.Query(context.Background(), "institutions", "out;"); err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(filepath.Join(client.CacheDir, "institutions.json"))
	if err != nil {
		t.Fatalf("nothing was cached: %v", err)
	}
	if string(stored) != `{"elements":[{"type":"node"}]}` {
		t.Errorf("cached %q", stored)
	}

	cached, err := client.Cached("institutions")
	if err != nil || string(cached) != string(stored) {
		t.Errorf("Cached returned %q, %v", cached, err)
	}
}

func TestSendsTheQueryAndIdentifiesItself(t *testing.T) {
	var got, agent string
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got = r.Form.Get("data")
		agent = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{"elements":[]}`))
	})

	if _, err := client.Query(context.Background(), "test", "[out:json];node;out;"); err != nil {
		t.Fatal(err)
	}
	if got != "[out:json];node;out;" {
		t.Errorf("server received %q", got)
	}
	// The operators ask for a named agent so they can see who is causing load.
	if !strings.Contains(agent, "schulsicherheitskarte") {
		t.Errorf("User-Agent = %q", agent)
	}
}

func TestBBoxUsesOverpassOrder(t *testing.T) {
	// The configuration is minLon, minLat, maxLon, maxLat; Overpass wants
	// south, west, north, east. Swapping them silently queries the wrong place.
	query := osm.InstitutionsQuery(osm.BBox{12.2263668, 50.54656, 12.8061082, 50.9242066})
	if !strings.Contains(query, "50.54656,12.2263668,50.9242066,12.8061082") {
		t.Errorf("query does not carry the box in Overpass order:\n%s", query)
	}
}
