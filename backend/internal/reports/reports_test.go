package reports_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/dbtest"
	"github.com/praetorianer777/schulsicherheitskarte/backend/internal/reports"
)

func store(t *testing.T) (*reports.Store, *pgxpool.Pool) {
	t.Helper()
	pool := dbtest.Pool(t)
	t.Cleanup(func() { dbtest.Truncate(t, pool, "reports", "report_confirmations") })
	return reports.NewStore(pool, []byte("test-salt")), pool
}

func submission() reports.Submission {
	return reports.Submission{
		Lon: 12.62, Lat: 50.79,
		Category:    "crossing_unsafe",
		Description: "Kein Zebrastreifen, Autos fahren zu schnell.",
	}
}

// A report that is public the moment it arrives makes moderation pointless.
func TestNewReportsArrivePending(t *testing.T) {
	s, _ := store(t)
	ctx := context.Background()

	report, err := s.Create(ctx, submission(), s.Fingerprint("203.0.113.5:1234"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if report.Status != reports.StatusPending {
		t.Errorf("status = %q, want pending", report.Status)
	}

	public, err := s.ListApproved(ctx, [4]float64{12.0, 50.0, 13.0, 51.0})
	if err != nil {
		t.Fatal(err)
	}
	if len(public) != 0 {
		t.Errorf("%d reports are public before anyone looked at them", len(public))
	}
}

func TestApprovedReportsBecomeVisible(t *testing.T) {
	s, _ := store(t)
	ctx := context.Background()

	report, err := s.Create(ctx, submission(), s.Fingerprint("203.0.113.5:1234"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Moderate(ctx, report.ID, reports.StatusApproved, "geprüft"); err != nil {
		t.Fatalf("moderate: %v", err)
	}

	public, err := s.ListApproved(ctx, [4]float64{12.0, 50.0, 13.0, 51.0})
	if err != nil {
		t.Fatal(err)
	}
	if len(public) != 1 {
		t.Fatalf("%d reports are public after approval, want 1", len(public))
	}
	if public[0].Description != submission().Description {
		t.Errorf("description = %q", public[0].Description)
	}
}

func TestRejectedReportsStayInvisible(t *testing.T) {
	s, _ := store(t)
	ctx := context.Background()

	report, err := s.Create(ctx, submission(), s.Fingerprint("203.0.113.5:1234"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Moderate(ctx, report.ID, reports.StatusRejected, "Duplikat"); err != nil {
		t.Fatal(err)
	}

	public, _ := s.ListApproved(ctx, [4]float64{12.0, 50.0, 13.0, 51.0})
	if len(public) != 0 {
		t.Errorf("a rejected report is on the public map")
	}
}

func TestTheRateLimitStopsOneSubmitter(t *testing.T) {
	s, _ := store(t)
	ctx := context.Background()
	fingerprint := s.Fingerprint("203.0.113.5:1234")

	for i := 0; i < reports.RateLimit; i++ {
		if _, err := s.Create(ctx, submission(), fingerprint); err != nil {
			t.Fatalf("report %d: %v", i, err)
		}
	}
	if _, err := s.Create(ctx, submission(), fingerprint); !errors.Is(err, reports.ErrRateLimited) {
		t.Errorf("error = %v, want ErrRateLimited", err)
	}

	// Somebody else is not affected by that.
	if _, err := s.Create(ctx, submission(), s.Fingerprint("198.51.100.9:1234")); err != nil {
		t.Errorf("a different submitter was refused: %v", err)
	}
}

// The address itself is never stored; only a keyed digest of it is.
func TestTheSubmitterAddressIsNotStored(t *testing.T) {
	s, pool := store(t)
	ctx := context.Background()

	const address = "203.0.113.5:1234"
	if _, err := s.Create(ctx, submission(), s.Fingerprint(address)); err != nil {
		t.Fatal(err)
	}

	var stored []byte
	if err := pool.QueryRow(ctx, "SELECT submitter_hash FROM reports").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if string(stored) == address || string(stored) == "203.0.113.5" {
		t.Fatal("the address was stored in the clear")
	}
	if len(stored) != 16 {
		t.Errorf("stored %d bytes, want the truncated digest", len(stored))
	}
}

// A household is routinely given a whole IPv6 range; counting each address in
// it separately would make the limit meaningless there.
func TestIPv6AddressesInOnePrefixCountAsOneSubmitter(t *testing.T) {
	s, _ := store(t)

	first := s.Fingerprint("[2001:db8:1234:5678::1]:443")
	second := s.Fingerprint("[2001:db8:1234:5678:aaaa:bbbb:cccc:dddd]:443")
	other := s.Fingerprint("[2001:db8:1234:9999::1]:443")

	if string(first) != string(second) {
		t.Error("two addresses in the same /64 were counted as two submitters")
	}
	if string(first) == string(other) {
		t.Error("two different /64 prefixes were counted as one submitter")
	}
}

// Rotating the salt makes every stored fingerprint worthless, which is the
// point of having one.
func TestTheSaltChangesTheFingerprint(t *testing.T) {
	pool := dbtest.Pool(t)
	first := reports.NewStore(pool, []byte("salt-one"))
	second := reports.NewStore(pool, []byte("salt-two"))

	if string(first.Fingerprint("203.0.113.5:1")) == string(second.Fingerprint("203.0.113.5:1")) {
		t.Error("the fingerprint does not depend on the salt")
	}
}

func TestConfirmationsCountOncePerSubmitter(t *testing.T) {
	s, _ := store(t)
	ctx := context.Background()

	report, err := s.Create(ctx, submission(), s.Fingerprint("203.0.113.5:1"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Moderate(ctx, report.ID, reports.StatusApproved, ""); err != nil {
		t.Fatal(err)
	}

	other := s.Fingerprint("198.51.100.9:1")
	if err := s.Confirm(ctx, report.ID, other); err != nil {
		t.Fatalf("first confirmation: %v", err)
	}
	if err := s.Confirm(ctx, report.ID, other); !errors.Is(err, reports.ErrDuplicate) {
		t.Errorf("second confirmation by the same submitter: %v", err)
	}

	public, _ := s.ListApproved(ctx, [4]float64{12.0, 50.0, 13.0, 51.0})
	if public[0].Confirmations != 1 {
		t.Errorf("confirmations = %d, want 1", public[0].Confirmations)
	}
}

func TestConfirmingAnUnknownReport(t *testing.T) {
	s, _ := store(t)
	err := s.Confirm(context.Background(), uuid.New(), s.Fingerprint("203.0.113.5:1"))
	if !errors.Is(err, reports.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestTheQueueHoldsWhatIsWaiting(t *testing.T) {
	s, _ := store(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if _, err := s.Create(ctx, submission(), s.Fingerprint("203.0.113.5:1")); err != nil {
			t.Fatal(err)
		}
	}
	queue, err := s.ListByStatus(ctx, reports.StatusPending, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 3 {
		t.Errorf("queue holds %d reports, want 3", len(queue))
	}

	if err := s.Moderate(ctx, queue[0].ID, reports.StatusApproved, ""); err != nil {
		t.Fatal(err)
	}
	remaining, _ := s.ListByStatus(ctx, reports.StatusPending, 100)
	if len(remaining) != 2 {
		t.Errorf("queue holds %d reports after one decision, want 2", len(remaining))
	}
}

func TestModeratingAnUnknownReport(t *testing.T) {
	s, _ := store(t)
	err := s.Moderate(context.Background(), uuid.New(), reports.StatusApproved, "")
	if !errors.Is(err, reports.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestPendingIsNotAModerationDecision(t *testing.T) {
	s, _ := store(t)
	ctx := context.Background()
	report, err := s.Create(ctx, submission(), s.Fingerprint("203.0.113.5:1"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Moderate(ctx, report.ID, reports.StatusPending, ""); err == nil {
		t.Error("moderating a report back to pending was accepted")
	}
}
