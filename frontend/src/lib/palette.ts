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
  institution: "#0b0b0b",
  /** A ring in the surface colour separates overlapping marks. */
  ring: "#ffffff",
} as const;

/** Radius in pixels. Bigger means worse, and the legend says so. */
export const severityRadius = { 1: 8, 2: 6, 3: 4 } as const;
