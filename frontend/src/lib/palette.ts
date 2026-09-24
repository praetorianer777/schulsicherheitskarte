/**
 * Map colours.
 *
 * Validated as a categorical palette over all pairs against a light map
 * surface (#f4f1ea): worst normal-vision ΔE 16.3, worst CVD ΔE 9.1. Yellow and
 * aqua sit below 3:1 against that surface, which the legend labels and the
 * table view relieve — no mark carries meaning by colour alone.
 *
 * Accident severity is deliberately **not** a colour scale. Three reds on a map
 * are neither separable nor contrasty enough; severity is carried by the radius
 * and spelled out in the list.
 */
export const mapColours = {
  accident: "#d03b3b",
  crossing: "#2a78d6",
  trafficSignals: "#eda100",
  trafficCalming: "#1baf7a",
  speedLimit: "#4a3aa7",
  /**
   * Reported places. Against aqua this pair sits in the 6–8 CVD band, which is
   * only allowed with a second encoding — the report marker is drawn larger
   * and with a thicker ring, and every report is in the list below the map.
   */
  report: "#e87ba4",
  institution: "#0b0b0b",
  /** A ring in the surface colour separates overlapping marks. */
  ring: "#ffffff",
} as const;

/** Radius in pixels. Bigger means worse, and the legend says so. */
export const severityRadius = { 1: 8, 2: 6, 3: 4 } as const;

/**
 * The overview's figure per institution: accidents within 500 m, over every
 * imported reporting year. The same radius as scoring.NearbyRadiusMetres in the
 * backend and the institution page's default, so a click on a point opens a
 * page that starts with the number the point stood for.
 */
export const nearbyRadiusMetres = 500;

/**
 * Binned into five steps of one hue, light to dark, and drawn larger as well as
 * darker — no step is told apart by colour alone.
 *
 * Violet on purpose. Not the accident red, and not a status palette: nothing
 * here is "good" or "critical", and a count without traffic volumes cannot say
 * that. Not blue either, which was tried first: basemap.de draws water and its
 * parking signs in blue, and a school next to a car park read as one more sign.
 * One hue in OKLCH (300°), validated as an ordinal ramp against the map surface
 * (#f4f1ea): monotone lightness, every adjacent step visibly apart, the
 * lightest at 2.9:1.
 */
export type NearbyStep = { from: number; to: number | null; colour: string; radius: number };

export const nearbySteps: readonly NearbyStep[] = [
  { from: 0, to: 0, colour: "#a07cdb", radius: 5 },
  { from: 1, to: 4, colour: "#825eb9", radius: 6 },
  { from: 5, to: 14, colour: "#68449c", radius: 8 },
  { from: 15, to: 39, colour: "#4f2980", radius: 10 },
  { from: 40, to: null, colour: "#351859", radius: 12 },
];
