// Package reports holds the hazard reports people send in — the near misses
// that never reach an accident statistic.
package reports

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
)

// Categories are fixed. A free-text category would be unsortable, and the list
// is what a road traffic authority can act on.
var Categories = []string{
	"crossing_unsafe",
	"speeding",
	"missing_sidewalk",
	"blocked_view",
	"parking",
	"school_run_traffic",
	"other",
}

func ValidCategory(category string) bool {
	for _, known := range Categories {
		if known == category {
			return true
		}
	}
	return false
}

// MaxDescription keeps a report a report rather than an essay, and keeps the
// moderation queue readable.
const MaxDescription = 1000

type Report struct {
	ID            uuid.UUID
	Lon           float64
	Lat           float64
	Category      string
	Description   string
	Status        Status
	CreatedAt     time.Time
	Confirmations int
}

type Submission struct {
	Lon         float64
	Lat         float64
	Category    string
	Description string
}

var (
	ErrRateLimited = errors.New("too many reports from this submitter")
	ErrNotFound    = errors.New("report not found")
	ErrDuplicate   = errors.New("already confirmed")
)

// Limits how often one submitter can report. Generous enough for a parent
// walking a route and noting several places, tight enough that one person
// cannot manufacture a pattern.
const (
	RateWindow = 24 * time.Hour
	RateLimit  = 10
)

type Store struct {
	pool *pgxpool.Pool
	salt []byte
}

func NewStore(pool *pgxpool.Pool, salt []byte) *Store {
	return &Store{pool: pool, salt: salt}
}

// Fingerprint turns a client address into the value stored with a report.
//
// It is an HMAC over the address with a rotatable salt, truncated to 16 bytes.
// The address itself is never stored: it is only needed to tell "the same
// submitter again" from "somebody else", and a keyed digest answers that
// without keeping the address around.
func (s *Store) Fingerprint(remoteAddr string) []byte {
	mac := hmac.New(sha256.New, s.salt)
	mac.Write([]byte(normaliseAddress(remoteAddr)))
	return mac.Sum(nil)[:16]
}

// normaliseAddress drops the port and, for IPv6, keeps only the /64 prefix —
// a single household is routinely given a range, and counting each address in
// it separately would make the limit meaningless there.
func normaliseAddress(remoteAddr string) string {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	if addr.Is6() && !addr.Is4In6() {
		if prefix, err := addr.Prefix(64); err == nil {
			return prefix.String()
		}
	}
	return addr.String()
}

// Create stores a report. It arrives pending: a report that were public on
// arrival would make moderation pointless.
func (s *Store) Create(ctx context.Context, submission Submission, fingerprint []byte) (*Report, error) {
	const recent = `
		SELECT count(*) FROM reports
		WHERE submitter_hash = $1 AND created_at > now() - $2::interval`
	var sent int
	if err := s.pool.QueryRow(ctx, recent, fingerprint, RateWindow.String()).Scan(&sent); err != nil {
		return nil, fmt.Errorf("count recent reports: %w", err)
	}
	if sent >= RateLimit {
		return nil, ErrRateLimited
	}

	const insert = `
		INSERT INTO reports (geom, category, description, submitter_hash)
		VALUES (ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3, $4, $5)
		RETURNING id, status, created_at`
	report := &Report{
		Lon: submission.Lon, Lat: submission.Lat,
		Category: submission.Category, Description: submission.Description,
	}
	err := s.pool.QueryRow(ctx, insert, submission.Lon, submission.Lat,
		submission.Category, submission.Description, fingerprint).
		Scan(&report.ID, &report.Status, &report.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert report: %w", err)
	}
	return report, nil
}

// Confirm records that somebody else is affected by the same place. One
// confirmation per submitter, enforced by the primary key.
func (s *Store) Confirm(ctx context.Context, id uuid.UUID, fingerprint []byte) error {
	const exists = `SELECT EXISTS (SELECT 1 FROM reports WHERE id = $1)`
	var found bool
	if err := s.pool.QueryRow(ctx, exists, id).Scan(&found); err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}

	const insert = `
		INSERT INTO report_confirmations (report_id, submitter_hash)
		VALUES ($1, $2) ON CONFLICT DO NOTHING`
	tag, err := s.pool.Exec(ctx, insert, id, fingerprint)
	if err != nil {
		return fmt.Errorf("confirm report: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDuplicate
	}
	return nil
}

const reportColumns = `
	r.id, ST_X(r.geom::geometry), ST_Y(r.geom::geometry),
	r.category, r.description, r.status, r.created_at,
	(SELECT count(*) FROM report_confirmations c WHERE c.report_id = r.id)`

func scan(rows pgx.Rows) (Report, error) {
	var r Report
	err := rows.Scan(&r.ID, &r.Lon, &r.Lat, &r.Category, &r.Description,
		&r.Status, &r.CreatedAt, &r.Confirmations)
	return r, err
}

// ListApproved returns what the public map shows. Pending reports are
// deliberately invisible until somebody has looked at them.
func (s *Store) ListApproved(ctx context.Context, box [4]float64) ([]Report, error) {
	const q = `SELECT ` + reportColumns + ` FROM reports r
		WHERE r.status = 'approved'
		  AND ST_Intersects(r.geom, ST_MakeEnvelope($1, $2, $3, $4, 4326)::geography)
		ORDER BY r.created_at DESC`
	return s.query(ctx, q, box[0], box[1], box[2], box[3])
}

// ListNear returns the approved reports around one institution.
func (s *Store) ListNear(ctx context.Context, institutionID int64, radius int) ([]Report, error) {
	const q = `SELECT ` + reportColumns + ` FROM reports r, institutions i
		WHERE i.id = $1 AND r.status = 'approved' AND ST_DWithin(r.geom, i.geom, $2)
		ORDER BY r.created_at DESC`
	return s.query(ctx, q, institutionID, radius)
}

// ListByStatus is the moderation queue.
func (s *Store) ListByStatus(ctx context.Context, status Status, limit int) ([]Report, error) {
	const q = `SELECT ` + reportColumns + ` FROM reports r
		WHERE r.status = $1 ORDER BY r.created_at LIMIT $2`
	return s.query(ctx, q, status, limit)
}

func (s *Store) query(ctx context.Context, sql string, args ...any) ([]Report, error) {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reports := []Report{}
	for rows.Next() {
		report, err := scan(rows)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, rows.Err()
}

// Moderate approves or rejects a report.
func (s *Store) Moderate(ctx context.Context, id uuid.UUID, status Status, note string) error {
	if status != StatusApproved && status != StatusRejected {
		return fmt.Errorf("%q is not a moderation decision", status)
	}
	const q = `
		UPDATE reports SET status = $2, moderated_at = now(), moderation_note = NULLIF($3, '')
		WHERE id = $1`
	tag, err := s.pool.Exec(ctx, q, id, status, note)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
