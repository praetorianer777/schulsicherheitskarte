CREATE TABLE infrastructure (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    osm_type   text NOT NULL CHECK (osm_type IN ('node', 'way', 'relation')),
    osm_id     bigint NOT NULL,

    -- A single OSM node can be a crossing and a signal at once
    -- (highway=crossing + crossing=traffic_signals), so kind is part of the key.
    kind       text NOT NULL CHECK (kind IN ('crossing', 'traffic_signals', 'traffic_calming', 'speed_limit')),

    -- Crossings and signals are points, speed limits are the road they apply to.
    geom       geography NOT NULL,
    tags       jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT infrastructure_osm_key UNIQUE (osm_type, osm_id, kind)
);

CREATE INDEX infrastructure_geom_idx ON infrastructure USING GIST (geom);
CREATE INDEX infrastructure_kind_idx ON infrastructure (kind);
