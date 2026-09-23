-- 197 of 443 institutions in the pilot region carry no addr:city, so searching
-- by town missed almost half of them. Where a school sits is known for every
-- one of them, and so is where each municipality ends: the town is looked up
-- from the administrative boundaries instead of relying on the address.
--
-- municipality: admin_level 8, and admin_level 6 for a kreisfreie Stadt, both
--               recognised by their eight-digit Gemeindeschlüssel — a Landkreis
--               is level 6 too, but carries only five digits.
-- district:     admin_level 9, the Ortsteil. Parents say "Wüstenbrand", not
--               "Hohenstein-Ernstthal".
CREATE TABLE boundaries (
    osm_id      bigint PRIMARY KEY,
    kind        text NOT NULL CHECK (kind IN ('municipality', 'district')),
    name        text NOT NULL,
    ags         text,
    admin_level smallint NOT NULL,
    geom        geography(MultiPolygon, 4326) NOT NULL,
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX boundaries_geom_idx ON boundaries USING GIST (geom);

-- Where boundaries nest, the innermost one names the place.
CREATE FUNCTION boundary_at(point geography, wanted text) RETURNS text
LANGUAGE sql STABLE STRICT PARALLEL SAFE AS $$
    SELECT name FROM boundaries
    WHERE kind = wanted AND ST_Covers(geom, point)
    ORDER BY admin_level DESC, ST_Area(geom) ASC
    LIMIT 1
$$;

ALTER TABLE institutions ADD COLUMN town text, ADD COLUMN district text;

-- A generated column cannot be altered in place; dropping it drops its index.
ALTER TABLE institutions DROP COLUMN search_text;
ALTER TABLE institutions ADD COLUMN search_text text
    GENERATED ALWAYS AS (search_name(
        coalesce(name, '') || ' ' ||
        coalesce(tags->>'addr:city', '') || ' ' ||
        coalesce(tags->>'addr:suburb', '') || ' ' ||
        coalesce(tags->>'addr:place', '') || ' ' ||
        coalesce(tags->>'addr:street', '') || ' ' ||
        coalesce(town, '') || ' ' ||
        coalesce(district, '')
    )) STORED;

CREATE INDEX institutions_search_text_idx
    ON institutions USING gin (search_text gin_trgm_ops);
