CREATE TABLE accidents (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

    -- Digits of the official municipality key (AGS), assembled from the source
    -- columns ULAND, UREGBEZ, UKREIS and UGEMEINDE. Kept as text because the
    -- leading zero of a Schleswig-Holstein key is part of the key.
    ags           text   NOT NULL,

    year          smallint NOT NULL,
    month         smallint NOT NULL CHECK (month BETWEEN 1 AND 12),
    hour          smallint NOT NULL CHECK (hour BETWEEN 0 AND 23),
    weekday       smallint NOT NULL CHECK (weekday BETWEEN 1 AND 7),

    -- UKATEGORIE: 1 fatal, 2 serious injury, 3 slight injury.
    severity      smallint NOT NULL CHECK (severity BETWEEN 1 AND 3),
    kind          smallint NOT NULL,   -- UART
    type          smallint NOT NULL,   -- UTYP1
    light         smallint,            -- ULICHTVERH
    road_condition smallint,           -- USTRZUSTAND / STRZUSTAND, absent in some years

    bike          boolean NOT NULL,
    car           boolean NOT NULL,
    pedestrian    boolean NOT NULL,
    motorcycle    boolean NOT NULL,
    truck         boolean NOT NULL,
    other         boolean NOT NULL,

    geom          geography(Point, 4326) NOT NULL,

    -- The Unfallatlas carries no identifier that is stable across reporting
    -- years, so re-running an import has to recognise a row by its content.
    -- source_hash is a digest over the parsed field values in a fixed order;
    -- two rows that are identical in every published attribute are the same
    -- accident as far as the published data can tell.
    source_hash   bytea NOT NULL,

    imported_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT accidents_source_hash_key UNIQUE (source_hash)
);

CREATE INDEX accidents_geom_idx ON accidents USING GIST (geom);
CREATE INDEX accidents_year_idx ON accidents (year);
CREATE INDEX accidents_ags_idx  ON accidents (ags);

-- The school route view of the data: almost every query filters on these two
-- flags, and they select a small fraction of the rows.
CREATE INDEX accidents_vulnerable_idx ON accidents USING GIST (geom)
    WHERE pedestrian OR bike;
