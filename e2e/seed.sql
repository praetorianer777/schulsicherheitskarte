-- Deterministic data for the end-to-end suite.
--
-- Importing the real sources would take minutes, need the network and change
-- from one reporting year to the next; a test that asserts "12 accidents" has
-- to be able to rely on there being twelve.
--
-- The geography is real: St. Egidien in the pilot region.

TRUNCATE accidents, institutions, infrastructure, hotspots, reports, report_confirmations
  RESTART IDENTITY CASCADE;

INSERT INTO institutions (osm_type, osm_id, kind, name, school_type, geom, tags) VALUES
  ('way',  1001, 'school',       'Grundschule St. Egidien', 'Grundschule',
   ST_MakePoint(12.6210, 50.7900)::geography, '{"amenity":"school"}'),
  ('node', 1002, 'kindergarten', 'Kita Sonnenschein', NULL,
   ST_MakePoint(12.6400, 50.7900)::geography, '{"amenity":"kindergarten"}'),
  ('way',  1003, 'school',       'Oberschule Lichtenstein', 'Oberschule',
   ST_MakePoint(12.6300, 50.7600)::geography, '{"amenity":"school"}');

-- Around the Grundschule, all due north of it so the distances are easy to
-- follow (0.0001 degrees of latitude is 11.1 m):
--   three accidents within 11 m of each other, ~100 m out  -> one hotspot
--   two accidents within 6 m of each other,   ~300 m out  -> a second hotspot
--   one accident on its own,                  ~445 m out  -> no hotspot, because
--                                                            one accident is not
--                                                            a pattern
INSERT INTO accidents (ags, year, month, hour, weekday, severity, kind, type,
                       bike, car, pedestrian, motorcycle, truck, other, geom, source_hash) VALUES
  ('14524280', 2025,  5,  7, 3, 1, 3, 4, false, true, true,  false, false, false, ST_MakePoint(12.62100, 50.79090)::geography, '\x01'),
  ('14524280', 2024,  9, 16, 5, 2, 3, 4, true,  true, false, false, false, false, ST_MakePoint(12.62100, 50.79095)::geography, '\x02'),
  ('14524280', 2023,  3, 13, 2, 3, 5, 2, false, true, false, false, false, false, ST_MakePoint(12.62100, 50.79100)::geography, '\x03'),
  ('14524280', 2022,  6,  8, 1, 2, 3, 4, false, true, true,  false, false, false, ST_MakePoint(12.62100, 50.79270)::geography, '\x04'),
  ('14524280', 2021, 11, 17, 4, 3, 1, 1, true,  true, false, false, false, false, ST_MakePoint(12.62100, 50.79275)::geography, '\x05'),
  ('14524280', 2018,  1, 15, 6, 3, 2, 3, false, true, false, false, false, false, ST_MakePoint(12.62100, 50.79400)::geography, '\x06');

INSERT INTO infrastructure (osm_type, osm_id, kind, geom, tags) VALUES
  ('node', 2001, 'crossing',        ST_GeogFromText('SRID=4326;POINT(12.62120 50.79050)'), '{"highway":"crossing"}'),
  ('node', 2002, 'traffic_signals', ST_GeogFromText('SRID=4326;POINT(12.62150 50.79070)'), '{"highway":"traffic_signals"}'),
  ('way',  2003, 'speed_limit',     ST_GeogFromText('SRID=4326;LINESTRING(12.6205 50.7902,12.6215 50.7912)'), '{"highway":"residential","maxspeed":"30"}');
