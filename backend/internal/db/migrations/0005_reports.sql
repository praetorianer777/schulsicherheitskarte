CREATE TABLE reports (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    geom        geography(Point, 4326) NOT NULL,
    category    text NOT NULL CHECK (category IN (
                    'crossing_unsafe',
                    'speeding',
                    'missing_sidewalk',
                    'blocked_view',
                    'parking',
                    'school_run_traffic',
                    'other')),
    description text NOT NULL DEFAULT '',
    status      text NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'approved', 'rejected')),

    -- Salted, truncated digest of the submitter's IP, used for rate limiting
    -- only. The salt lives in the environment and can be rotated, which makes
    -- the stored value worthless on its own.
    submitter_hash bytea NOT NULL,

    created_at     timestamptz NOT NULL DEFAULT now(),
    moderated_at   timestamptz,
    moderation_note text
);

-- The public map reads approved reports by area; the moderation queue reads
-- pending ones by age. Neither should scan the whole table.
CREATE INDEX reports_public_idx ON reports USING GIST (geom) WHERE status = 'approved';
CREATE INDEX reports_queue_idx  ON reports (created_at) WHERE status = 'pending';
CREATE INDEX reports_rate_idx   ON reports (submitter_hash, created_at);

CREATE TABLE report_confirmations (
    report_id      uuid NOT NULL REFERENCES reports (id) ON DELETE CASCADE,
    submitter_hash bytea NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (report_id, submitter_hash)
);
