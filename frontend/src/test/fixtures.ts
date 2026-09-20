import type { Accident, Hotspot, Institution } from "../api/types";

export const school: Institution = {
  id: 275,
  kind: "school",
  name: "Peter Breuer Gymnasium",
  schoolType: "Gymnasium",
  lon: 12.4954,
  lat: 50.7188,
  osmType: "way",
  osmId: 1,
};

export const accidents: Accident[] = [
  {
    id: 1,
    lon: 12.4956,
    lat: 50.719,
    severity: 1,
    year: 2025,
    month: 5,
    hour: 7,
    weekday: 3,
    pedestrian: true,
    bike: false,
    car: true,
    motorcycle: false,
    truck: false,
    distance: 120,
  },
  {
    id: 2,
    lon: 12.4958,
    lat: 50.7192,
    severity: 3,
    year: 2019,
    month: 11,
    hour: 16,
    weekday: 6,
    pedestrian: false,
    bike: true,
    car: true,
    motorcycle: false,
    truck: false,
    distance: 240,
  },
];

export const hotspots: Hotspot[] = [
  {
    id: 10,
    lon: 12.4957,
    lat: 50.7191,
    accidentCount: 20,
    score: 55.8,
    breakdown: { fatal: 0, serious: 3, slight: 17, pedestrian: 3, bike: 9 },
    firstYear: 2016,
    lastYear: 2025,
    distance: 286,
  },
  {
    id: 11,
    lon: 12.496,
    lat: 50.7195,
    accidentCount: 5,
    score: 30.7,
    breakdown: { fatal: 0, serious: 3, slight: 2, pedestrian: 1, bike: 3 },
    firstYear: 2018,
    lastYear: 2024,
    distance: 369,
  },
];

export const summary = {
  total: 2,
  fatal: 1,
  serious: 0,
  slight: 1,
  pedestrian: 1,
  bike: 1,
};
