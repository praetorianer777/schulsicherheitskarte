-- The overview shows every institution with one figure: the accidents with
-- personal injury within 500 m, over every imported reporting year — the same
-- number the institution page opens with. Counting it on every pan of the map
-- would repeat the same radius query for every point in view, so importer
-- hotspots computes it once and stores it here.
--
-- NULL means "not counted yet", for an institution imported after the last
-- hotspots run. It is not zero, and the map does not draw it as zero.
ALTER TABLE institutions ADD COLUMN accidents_nearby integer;
