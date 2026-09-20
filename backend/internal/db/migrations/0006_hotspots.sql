-- Precomputed accident clusters. Recomputed as a whole by the importer, so the
-- table is replaced rather than updated row by row.
CREATE TABLE hotspots (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    geom               geography(Point, 4326) NOT NULL,
    accident_count     integer NOT NULL,
    score              double precision NOT NULL,

    -- Counts per severity, so the fact sheet can state "two serious, five
    -- slight" without querying the accidents again.
    severity_breakdown jsonb NOT NULL,

    first_year         smallint NOT NULL,
    last_year          smallint NOT NULL,
    computed_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX hotspots_geom_idx  ON hotspots USING GIST (geom);
CREATE INDEX hotspots_score_idx ON hotspots (score DESC);
