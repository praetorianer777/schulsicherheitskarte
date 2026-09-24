package osm

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Boundary is one municipality or district, still as the loose ways that
// OpenStreetMap stores it as. The database assembles the area from them.
type Boundary struct {
	OSMID      int64
	Kind       string // municipality | district
	Name       string
	AGS        string
	AdminLevel int
	Lines      [][]Point
}

var municipalityKey = regexp.MustCompile(`^[0-9]{8}$`)

// ParseBoundaries reads the relations BoundariesQuery returns.
//
// A relation without a name cannot name a town, and one without a single way
// cannot become an area; both are left out. So is a level-6 relation without an
// eight-digit Gemeindeschlüssel — the query already excludes those, but a cached
// response from an older query must not turn a Landkreis into a town.
func ParseBoundaries(body []byte) ([]Boundary, error) {
	elements, err := decode(body)
	if err != nil {
		return nil, err
	}

	var out []Boundary
	for _, e := range elements {
		if e.Type != "relation" || e.Tags["boundary"] != "administrative" {
			continue
		}
		name := strings.TrimSpace(e.Tags["name"])
		level, err := strconv.Atoi(e.Tags["admin_level"])
		if name == "" || err != nil {
			continue
		}
		ags := strings.TrimSpace(e.Tags["de:amtlicher_gemeindeschluessel"])

		var kind string
		switch {
		case level == 8 || (level == 6 && municipalityKey.MatchString(ags)):
			kind = "municipality"
		case level == 9:
			kind = "district"
		default:
			continue
		}

		var lines [][]Point
		for _, m := range e.Members {
			// Only the rings: a relation also lists its admin_centre and label
			// nodes, and sometimes its districts as subarea relations.
			if m.Type != "way" || (m.Role != "outer" && m.Role != "inner" && m.Role != "") || len(m.Geometry) < 2 {
				continue
			}
			line := make([]Point, 0, len(m.Geometry))
			for _, g := range m.Geometry {
				line = append(line, Point{Lon: g.Lon, Lat: g.Lat})
			}
			lines = append(lines, line)
		}
		if len(lines) == 0 {
			continue
		}

		out = append(out, Boundary{
			OSMID: e.ID, Kind: kind, Name: name, AGS: ags, AdminLevel: level, Lines: lines,
		})
	}
	return out, nil
}

type BoundaryResult struct {
	Written int
	Pruned  int
	// Broken counts relations whose ways do not close into rings — an edit in
	// progress in OpenStreetMap, usually. They are skipped rather than stored
	// as something that is not an area.
	Broken int
}

// ImportBoundaries writes the boundaries and then gives every institution in
// the box the town and district it lies in.
func ImportBoundaries(ctx context.Context, pool *pgxpool.Pool, items []Boundary, box BBox) (*BoundaryResult, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	start, err := transactionStart(ctx, tx)
	if err != nil {
		return nil, err
	}

	const staging = `
		CREATE TEMP TABLE staging_boundaries (
			osm_id bigint, kind text, name text, ags text, admin_level smallint, wkt text
		) ON COMMIT DROP`
	if _, err := tx.Exec(ctx, staging); err != nil {
		return nil, fmt.Errorf("create staging table: %w", err)
	}

	rows := make([][]any, 0, len(items))
	for _, item := range items {
		rows = append(rows, []any{
			item.OSMID, item.Kind, item.Name, nullable(item.AGS), item.AdminLevel, multiLineWKT(item.Lines),
		})
	}
	if len(rows) > 0 {
		columns := []string{"osm_id", "kind", "name", "ags", "admin_level", "wkt"}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"staging_boundaries"}, columns, pgx.CopyFromRows(rows)); err != nil {
			return nil, fmt.Errorf("stage boundaries: %w", err)
		}
	}

	// ST_BuildArea turns the rings into an area and nested rings into holes,
	// which is what an enclave is. Ways that do not close give no area at all.
	const assemble = `
		CREATE TEMP TABLE assembled_boundaries ON COMMIT DROP AS
		SELECT DISTINCT ON (osm_id) osm_id, kind, name, ags, admin_level,
		       ST_Multi(ST_BuildArea(ST_GeomFromText(wkt, 4326))) AS area
		FROM staging_boundaries`
	if _, err := tx.Exec(ctx, assemble); err != nil {
		return nil, fmt.Errorf("assemble boundaries: %w", err)
	}

	result := &BoundaryResult{}
	const broken = `SELECT count(*) FROM assembled_boundaries WHERE area IS NULL OR ST_IsEmpty(area)`
	if err := tx.QueryRow(ctx, broken).Scan(&result.Broken); err != nil {
		return nil, fmt.Errorf("count broken boundaries: %w", err)
	}

	const upsert = `
		INSERT INTO boundaries (osm_id, kind, name, ags, admin_level, geom, updated_at)
		SELECT osm_id, kind, name, ags, admin_level, area::geography, now()
		FROM assembled_boundaries
		WHERE area IS NOT NULL AND NOT ST_IsEmpty(area)
		ON CONFLICT (osm_id) DO UPDATE SET
			kind = EXCLUDED.kind, name = EXCLUDED.name, ags = EXCLUDED.ags,
			admin_level = EXCLUDED.admin_level, geom = EXCLUDED.geom, updated_at = now()`
	tag, err := tx.Exec(ctx, upsert)
	if err != nil {
		return nil, fmt.Errorf("upsert boundaries: %w", err)
	}
	result.Written = int(tag.RowsAffected())

	if result.Pruned, err = prune(ctx, tx, "boundaries", start, box); err != nil {
		return nil, err
	}

	// A boundary that moved or was renamed changes the town of institutions
	// that were not re-imported themselves.
	const relocate = `
		UPDATE institutions SET
			town = boundary_at(geom, 'municipality'),
			district = boundary_at(geom, 'district')
		WHERE ST_Intersects(geom, ST_MakeEnvelope($1, $2, $3, $4, 4326)::geography)`
	if _, err := tx.Exec(ctx, relocate, box[0], box[1], box[2], box[3]); err != nil {
		return nil, fmt.Errorf("assign towns: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

// TownCoverage says how many institutions in the box can be found by their
// town, and from where that town comes.
type TownCoverage struct {
	Total    int
	Address  int // addr:city is set
	Boundary int // no addr:city, but the boundaries give a town
	None     int
}

func CountTownCoverage(ctx context.Context, pool *pgxpool.Pool, box BBox) (*TownCoverage, error) {
	const q = `
		SELECT count(*),
		       count(*) FILTER (WHERE coalesce(tags->>'addr:city', '') <> ''),
		       count(*) FILTER (WHERE coalesce(tags->>'addr:city', '') = '' AND town IS NOT NULL),
		       count(*) FILTER (WHERE coalesce(tags->>'addr:city', '') = '' AND town IS NULL)
		FROM institutions
		WHERE ST_Intersects(geom, ST_MakeEnvelope($1, $2, $3, $4, 4326)::geography)`
	var c TownCoverage
	if err := pool.QueryRow(ctx, q, box[0], box[1], box[2], box[3]).Scan(&c.Total, &c.Address, &c.Boundary, &c.None); err != nil {
		return nil, fmt.Errorf("count town coverage: %w", err)
	}
	return &c, nil
}

func multiLineWKT(lines [][]Point) string {
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		coords := make([]string, 0, len(line))
		for _, p := range line {
			coords = append(coords, fmt.Sprintf("%.7f %.7f", p.Lon, p.Lat))
		}
		parts = append(parts, "("+strings.Join(coords, ",")+")")
	}
	return "MULTILINESTRING(" + strings.Join(parts, ",") + ")"
}
