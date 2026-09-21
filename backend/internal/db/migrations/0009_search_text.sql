-- "werdau" found one school: the one with the town in its name. The town is
-- the one thing a parent knows for certain, so the address has to be searchable
-- alongside the name.
--
-- Stored rather than computed per query so the trigram index can sit on it; the
-- word search and the similarity fallback then apply to it unchanged.
--
-- What this does not give: a town for every school. On the Landkreis Zwickau
-- import, 246 of 443 institutions carry addr:city and 249 addr:street — for
-- the rest OpenStreetMap has no address at all, and their town stays
-- unsearchable until it is derived from geometry (#48).
ALTER TABLE institutions ADD COLUMN search_text text
    GENERATED ALWAYS AS (search_name(
        coalesce(name, '') || ' ' ||
        coalesce(tags->>'addr:city', '') || ' ' ||
        coalesce(tags->>'addr:suburb', '') || ' ' ||
        coalesce(tags->>'addr:place', '') || ' ' ||
        coalesce(tags->>'addr:street', '')
    )) STORED;

DROP INDEX institutions_search_name_idx;
CREATE INDEX institutions_search_text_idx
    ON institutions USING gin (search_text gin_trgm_ops);
