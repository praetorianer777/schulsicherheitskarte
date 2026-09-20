package osm

import "fmt"

// BBox is minLon, minLat, maxLon, maxLat — the order the region configuration
// uses. Overpass wants south, west, north, east, which is why every query goes
// through this type rather than formatting the box at the call site.
type BBox [4]float64

func (b BBox) overpass() string {
	return fmt.Sprintf("%g,%g,%g,%g", b[1], b[0], b[3], b[2])
}

// The queries are split by what they return rather than merged into one, so a
// service timeout costs one of them instead of all, and so each one caches and
// replays on its own.
const (
	QueryInstitutions = "institutions"
	QueryCrossings    = "crossings"
	QuerySpeedLimits  = "speed_limits"
)

// InstitutionsQuery asks for schools and kindergartens. They are mapped as
// nodes, as grounds (ways) and occasionally as relations, so all three are
// requested and reduced to their centre.
func InstitutionsQuery(b BBox) string {
	box := b.overpass()
	return fmt.Sprintf(`[out:json][timeout:180];
(
  node["amenity"~"^(school|kindergarten)$"](%[1]s);
  way["amenity"~"^(school|kindergarten)$"](%[1]s);
  relation["amenity"~"^(school|kindergarten)$"](%[1]s);
);
out center tags;`, box)
}

// CrossingsQuery asks for everything that helps somebody cross a road, plus
// what slows traffic down: crossings, pedestrian signals and traffic calming.
func CrossingsQuery(b BBox) string {
	box := b.overpass()
	return fmt.Sprintf(`[out:json][timeout:180];
(
  node["highway"="crossing"](%[1]s);
  node["highway"="traffic_signals"](%[1]s);
  node["traffic_calming"](%[1]s);
  way["traffic_calming"](%[1]s);
);
out geom tags;`, box)
}

// SpeedLimitsQuery asks for the roads that carry an explicit limit. The
// geometry is needed, not the centre: a limit applies to a stretch of road.
func SpeedLimitsQuery(b BBox) string {
	box := b.overpass()
	return fmt.Sprintf(`[out:json][timeout:180];
way["highway"]["maxspeed"](%[1]s);
out geom tags;`, box)
}
