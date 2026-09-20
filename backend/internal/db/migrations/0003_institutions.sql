CREATE TABLE institutions (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    osm_type    text NOT NULL CHECK (osm_type IN ('node', 'way', 'relation')),
    osm_id      bigint NOT NULL,
    kind        text NOT NULL CHECK (kind IN ('school', 'kindergarten')),
    name        text,
    school_type text,
    geom        geography(Point, 4326) NOT NULL,
    tags        jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT institutions_osm_key UNIQUE (osm_type, osm_id)
);

CREATE INDEX institutions_geom_idx ON institutions USING GIST (geom);

-- Name search is what the start page does; unaccent is not available by
-- default, so searching stays on a trigram-free lower() match for now.
CREATE INDEX institutions_name_idx ON institutions (lower(name));
