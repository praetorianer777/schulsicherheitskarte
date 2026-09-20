import type { Accident, Institution } from "../api/types";

const numbers = new Intl.NumberFormat("de-DE");

export const formatNumber = (value: number) => numbers.format(value);

export const severityLabel: Record<1 | 2 | 3, string> = {
  1: "Getötete",
  2: "Schwerverletzte",
  3: "Leichtverletzte",
};

export const kindLabel: Record<Institution["kind"], string> = {
  school: "Schule",
  kindergarten: "Kita",
};

export const infrastructureLabel = {
  crossing: "Querungshilfe",
  traffic_signals: "Ampel",
  traffic_calming: "Verkehrsberuhigung",
  speed_limit: "Tempolimit",
} as const;

const weekdays = ["", "Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"];

export const weekdayLabel = (weekday: number) => weekdays[weekday] ?? "";

/** A one-line description used where the map cannot speak. */
export function describeAccident(accident: Accident): string {
  const involved: string[] = [];
  if (accident.pedestrian) involved.push("zu Fuß");
  if (accident.bike) involved.push("mit dem Rad");
  const who = involved.length > 0 ? involved.join(" und ") : "ohne Fuß- oder Radbeteiligung";
  return `${severityLabel[accident.severity]}, ${accident.year}, ${who}, ${formatNumber(accident.distance)} m entfernt`;
}

export function institutionName(institution: Institution): string {
  return institution.name?.trim() || "Einrichtung ohne Namen in OpenStreetMap";
}

export const reportCategoryLabel: Record<string, string> = {
  crossing_unsafe: "Querung unübersichtlich oder fehlt",
  speeding: "Autos fahren zu schnell",
  missing_sidewalk: "Gehweg fehlt oder ist zu schmal",
  blocked_view: "Sicht versperrt",
  parking: "Parkende Fahrzeuge behindern",
  school_run_traffic: "Elterntaxis vor der Einrichtung",
  other: "Sonstiges",
};

/**
 * Where a reported point sits relative to the institution, in words. Somebody
 * who cannot see the marker move still has to know where it went.
 */
export function describeOffset(
  point: { lon: number; lat: number },
  origin: { lon: number; lat: number },
): string {
  const north = (point.lat - origin.lat) * 111_320;
  const east = (point.lon - origin.lon) * 111_320 * Math.cos((origin.lat * Math.PI) / 180);
  const distance = Math.round(Math.hypot(north, east));
  if (distance < 10) return "an der Einrichtung";

  const parts: string[] = [];
  if (Math.abs(north) >= 10) parts.push(north > 0 ? "nördlich" : "südlich");
  if (Math.abs(east) >= 10) parts.push(east > 0 ? "östlich" : "westlich");
  return `${distance} m ${parts.join(" und ")} der Einrichtung`;
}
