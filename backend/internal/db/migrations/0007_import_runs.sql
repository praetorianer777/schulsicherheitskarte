-- Every number shown on the map has to be traceable to the import that
-- produced it, including a failed one.
CREATE TABLE import_runs (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source       text NOT NULL CHECK (source IN ('accidents', 'osm', 'hotspots')),

    -- Which slice of the source: a reporting year, an Overpass query name.
    detail       text NOT NULL DEFAULT '',

    status       text NOT NULL DEFAULT 'running'
                     CHECK (status IN ('running', 'succeeded', 'failed')),
    started_at   timestamptz NOT NULL DEFAULT now(),
    finished_at  timestamptz,
    rows_read    integer NOT NULL DEFAULT 0,
    rows_written integer NOT NULL DEFAULT 0,

    -- Digest of the downloaded artifact, so an unchanged source is visible as
    -- such instead of looking like a fresh import.
    checksum     text,
    message      text
);

CREATE INDEX import_runs_source_idx ON import_runs (source, started_at DESC);
