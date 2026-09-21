-- People do not type the official spelling. "Bergschule St Egidien" without the
-- full stop, "bergschule egidien" without the middle word, "Strasse" for
-- "Straße" — all of those missed, because the search was one literal substring
-- over the whole name.
--
-- The normalisation has to apply to the stored name and to what was typed
-- alike, so it lives here rather than in a query built in Go, and it is
-- immutable so the index below can use it.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE FUNCTION search_name(value text) RETURNS text
LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE AS $$
    SELECT btrim(regexp_replace(
        replace(replace(replace(replace(replace(lower(value),
            'ä', 'ae'), 'ö', 'oe'), 'ü', 'ue'), 'ß', 'ss'), 'é', 'e'),
        '[^a-z0-9]+', ' ', 'g'))
$$;

-- Trigram index: it serves both the substring match per word and the similarity
-- fallback, and the table is 443 rows for one district but every school in
-- Germany once the import is widened.
CREATE INDEX institutions_search_name_idx
    ON institutions USING gin (search_name(name) gin_trgm_ops);
