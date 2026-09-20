package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/api"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/dbtest"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/reports"
)

const moderationToken = "geheim-im-test"

type reportFixture struct {
	server *httptest.Server
}

func seedReports(t *testing.T) *reportFixture {
	t.Helper()
	pool := dbtest.Pool(t)
	t.Cleanup(func() { dbtest.Truncate(t, pool, "reports", "report_confirmations", "institutions") })

	handler := api.New(pool, api.Options{ModerationToken: moderationToken}).
		WithReports(reports.NewStore(pool, []byte("test-salt"))).
		Routes()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &reportFixture{server: server}
}

func (f *reportFixture) post(t *testing.T, path string, body any, token string) (int, map[string]any) {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, f.server.URL+path, bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := f.server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	var decoded map[string]any
	_ = json.NewDecoder(response.Body).Decode(&decoded)
	return response.StatusCode, decoded
}

func (f *reportFixture) get(t *testing.T, path, token string) (int, map[string]any) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, f.server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := f.server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	var decoded map[string]any
	_ = json.NewDecoder(response.Body).Decode(&decoded)
	return response.StatusCode, decoded
}

func validReport() map[string]any {
	return map[string]any{
		"lon": 12.62, "lat": 50.79,
		"category":    "speeding",
		"description": "Autos fahren morgens deutlich zu schnell.",
	}
}

// The whole point of moderation: a report is not on the map until somebody has
// looked at it.
func TestASubmittedReportIsNotPublicYet(t *testing.T) {
	f := seedReports(t)

	status, _ := f.post(t, "/api/reports", validReport(), "")
	if status != http.StatusCreated {
		t.Fatalf("submitting a report answered %d", status)
	}

	_, body := f.get(t, "/api/reports?bbox=12.0,50.0,13.0,51.0", "")
	list, _ := body["reports"].([]any)
	if len(list) != 0 {
		t.Errorf("%d reports are public before moderation", len(list))
	}
}

func TestApprovingMakesTheReportPublic(t *testing.T) {
	f := seedReports(t)

	_, created := f.post(t, "/api/reports", validReport(), "")
	id, _ := created["id"].(string)

	status, _ := f.post(t, "/api/admin/reports/"+id, map[string]any{"status": "approved"}, moderationToken)
	if status != http.StatusOK {
		t.Fatalf("approving answered %d", status)
	}

	_, body := f.get(t, "/api/reports?bbox=12.0,50.0,13.0,51.0", "")
	list, _ := body["reports"].([]any)
	if len(list) != 1 {
		t.Errorf("%d reports are public after approval, want 1", len(list))
	}
}

// Without a token the queue is closed, and so is the decision.
func TestModerationNeedsTheToken(t *testing.T) {
	f := seedReports(t)
	_, created := f.post(t, "/api/reports", validReport(), "")
	id, _ := created["id"].(string)

	if status, _ := f.get(t, "/api/admin/reports", ""); status != http.StatusUnauthorized {
		t.Errorf("the queue answered %d without a token", status)
	}
	if status, _ := f.get(t, "/api/admin/reports", "falsches-token"); status != http.StatusUnauthorized {
		t.Errorf("the queue answered %d for a wrong token", status)
	}
	if status, _ := f.post(t, "/api/admin/reports/"+id, map[string]any{"status": "approved"}, ""); status != http.StatusUnauthorized {
		t.Errorf("a decision answered %d without a token", status)
	}

	// And the report is still where it was.
	_, body := f.get(t, "/api/reports?bbox=12.0,50.0,13.0,51.0", "")
	list, _ := body["reports"].([]any)
	if len(list) != 0 {
		t.Error("the report became public although no valid token was presented")
	}
}

func TestTheQueueShowsWhatIsWaiting(t *testing.T) {
	f := seedReports(t)
	f.post(t, "/api/reports", validReport(), "")

	status, body := f.get(t, "/api/admin/reports", moderationToken)
	if status != http.StatusOK {
		t.Fatalf("the queue answered %d", status)
	}
	list, _ := body["reports"].([]any)
	if len(list) != 1 {
		t.Errorf("queue holds %d reports, want 1", len(list))
	}
}

func TestBadSubmissionsAreRefusedWithAReason(t *testing.T) {
	f := seedReports(t)

	cases := []struct {
		name string
		body map[string]any
	}{
		{"unbekannte Kategorie", map[string]any{"lon": 12.62, "lat": 50.79, "category": "sonstiges"}},
		{"außerhalb Deutschlands", map[string]any{"lon": 2.35, "lat": 48.85, "category": "speeding"}},
		{"vertauschte Koordinaten", map[string]any{"lon": 50.79, "lat": 12.62, "category": "speeding"}},
	}
	for _, c := range cases {
		status, body := f.post(t, "/api/reports", c.body, "")
		if status != http.StatusBadRequest {
			t.Errorf("%s: answered %d, want 400", c.name, status)
			continue
		}
		if body["error"] == "" || body["parameter"] == "" {
			t.Errorf("%s: no reason given: %v", c.name, body)
		}
	}
}

func TestAnOverlongDescriptionIsRefused(t *testing.T) {
	f := seedReports(t)
	long := make([]rune, reports.MaxDescription+1)
	for i := range long {
		long[i] = 'a'
	}
	body := validReport()
	body["description"] = string(long)

	if status, _ := f.post(t, "/api/reports", body, ""); status != http.StatusBadRequest {
		t.Errorf("an overlong description answered %d", status)
	}
}

// Every request in a test comes from the same loopback address, so the limit
// is reachable — which is what makes it testable at all.
func TestTheRateLimitAnswers429(t *testing.T) {
	f := seedReports(t)

	for i := 0; i < reports.RateLimit; i++ {
		if status, _ := f.post(t, "/api/reports", validReport(), ""); status != http.StatusCreated {
			t.Fatalf("report %d answered %d", i, status)
		}
	}
	status, body := f.post(t, "/api/reports", validReport(), "")
	if status != http.StatusTooManyRequests {
		t.Fatalf("the eleventh report answered %d, want 429", status)
	}
	if body["error"] == "" {
		t.Error("the refusal says nothing about what to do")
	}
}

func TestConfirmingTwiceIsNotAnError(t *testing.T) {
	f := seedReports(t)
	_, created := f.post(t, "/api/reports", validReport(), "")
	id, _ := created["id"].(string)
	f.post(t, "/api/admin/reports/"+id, map[string]any{"status": "approved"}, moderationToken)

	if status, _ := f.post(t, "/api/reports/"+id+"/confirm", nil, ""); status != http.StatusCreated {
		t.Errorf("the first confirmation answered %d", status)
	}
	// The state the caller wanted is the state that holds.
	if status, _ := f.post(t, "/api/reports/"+id+"/confirm", nil, ""); status != http.StatusOK {
		t.Errorf("the second confirmation answered %d", status)
	}
}

func TestConfirmingAnUnknownReportIs404(t *testing.T) {
	f := seedReports(t)
	status, _ := f.post(t, "/api/reports/3f1d0a1e-0000-4000-8000-000000000000/confirm", nil, "")
	if status != http.StatusNotFound {
		t.Errorf("answered %d, want 404", status)
	}
}

// A missing token must mean the queue is closed, not that there is no check.
func TestWithoutAConfiguredTokenTheQueueIsClosed(t *testing.T) {
	pool := dbtest.Pool(t)
	t.Cleanup(func() { dbtest.Truncate(t, pool, "reports") })

	handler := api.New(pool, api.Options{}).
		WithReports(reports.NewStore(pool, []byte("test-salt"))).
		Routes()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	response, err := server.Client().Get(server.URL + "/api/admin/reports")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("answered %d; an unconfigured queue must not be an open one", response.StatusCode)
	}
}
