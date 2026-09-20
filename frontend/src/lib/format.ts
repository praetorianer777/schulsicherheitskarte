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
