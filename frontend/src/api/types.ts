import type { Geometry } from "geojson";

export type Source = { name: string; licence: string; url: string };

export type Institution = {
  id: number;
  kind: "school" | "kindergarten";
  name: string | null;
  schoolType?: string;
  lon: number;
  lat: number;
  osmType: string;
  osmId: number;
  tags?: Record<string, string>;
};

export type Accident = {
  id: number;
  lon: number;
  lat: number;
  /** 1 Getötete, 2 Schwerverletzte, 3 Leichtverletzte (UKATEGORIE). */
  severity: 1 | 2 | 3;
  year: number;
  month: number;
  hour: number;
  weekday: number;
  pedestrian: boolean;
  bike: boolean;
  car: boolean;
  motorcycle: boolean;
  truck: boolean;
  distance: number;
};

export type AccidentCounts = {
  total: number;
  fatal: number;
  serious: number;
  slight: number;
  pedestrian: number;
  bike: number;
};

export type Hotspot = {
  id: number;
  lon: number;
  lat: number;
  accidentCount: number;
  score: number;
  breakdown: Record<string, number>;
  firstYear: number;
  lastYear: number;
  distance: number;
};

export type InfrastructureKind =
  | "crossing"
  | "traffic_signals"
  | "traffic_calming"
  | "speed_limit";

export type Infrastructure = {
  id: number;
  kind: InfrastructureKind;
  geometry: Geometry;
  tags?: Record<string, string>;
  distance: number;
};

/** Where the imported institutions are; bbox is null on an empty database. */
export type Extent = {
  bbox: [number, number, number, number] | null;
  institutions: number;
};

export type InstitutionList = {
  institutions: Institution[];
  // Set when the list is empty because nothing has been imported yet, rather
  // than because nothing matched.
  nothingImported?: boolean;
  sources: Source[];
};
export type InstitutionDetail = { institution: Institution; sources: Source[] };
export type AccidentList = {
  accidents: Accident[];
  summary: AccidentCounts;
  radius: number;
  sources: Source[];
};
export type HotspotList = { hotspots: Hotspot[]; radius: number; sources: Source[] };
export type InfrastructureList = {
  infrastructure: Infrastructure[];
  counts: Partial<Record<InfrastructureKind, number>>;
  radius: number;
  sources: Source[];
};

export type ApiError = { error: string; parameter?: string };

export type ReportCategory =
  | "crossing_unsafe"
  | "speeding"
  | "missing_sidewalk"
  | "blocked_view"
  | "parking"
  | "school_run_traffic"
  | "other";

export type Report = {
  id: string;
  lon: number;
  lat: number;
  category: ReportCategory;
  description: string;
  status: "pending" | "approved" | "rejected";
  createdAt: string;
  confirmations: number;
};

export type ReportList = { reports: Report[]; categories: ReportCategory[] };
export type Method = {
  weightFatal: number;
  weightSerious: number;
  weightSlight: number;
  weightVulnerable: number;
  halfLifeYears: number;
  clusterRadiusMetres: number;
  clusterMinAccidents: number;
  referenceYear: number;
};

export type Factsheet = {
  institution: Institution;
  radius: number;
  years: { from: number; to: number };
  summary: AccidentCounts;
  hotspots: Hotspot[];
  infrastructureCounts: Partial<Record<InfrastructureKind, number>>;
  method: Method;
  sources: Source[];
};
