import { Map as MapLibreMap } from "maplibre-gl";
import type { GeoJSONSource } from "maplibre-gl";
import { useEffect, useRef } from "react";
import type { FeatureCollection } from "geojson";

import type { Hotspot, Institution } from "../api/types";
import { institutionName } from "../lib/format";
import { mapColours } from "../lib/palette";

const styleURL = "https://sgx.geodatenzentrum.de/gdz_basemapde_vektor/styles/bm_web_col.json";

type Props = {
  institution: Institution;
  hotspots: Hotspot[];
  radius: number;
};

/**
 * The map on the printed sheet: the institution, the radius it is measured
 * over, and the hotspots numbered to match the table.
 *
 * preserveDrawingBuffer keeps the WebGL canvas readable after it has been
 * drawn, which is what makes it come out on paper at all — without it the map
 * prints blank.
 */
export default function FactsheetMap({ institution, hotspots, radius }: Props) {
  const container = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!container.current) return;

    const map = new MapLibreMap({
      container: container.current,
      style: styleURL,
      center: [institution.lon, institution.lat],
      zoom: zoomForRadius(radius),
      interactive: false,
      canvasContextAttributes: { preserveDrawingBuffer: true },
      attributionControl: false,
    });

    map.on("load", () => {
      map.addSource("ring", { type: "geojson", data: ringFeature(institution, radius) });
      map.addSource("hotspots", { type: "geojson", data: hotspotFeatures(hotspots) });

      map.addLayer({
        id: "ring-line",
        type: "line",
        source: "ring",
        paint: { "line-color": mapColours.institution, "line-width": 1.5, "line-dasharray": [3, 3] },
      });
      map.addLayer({
        id: "hotspot-circles",
        type: "circle",
        source: "hotspots",
        paint: {
          "circle-radius": 12,
          "circle-color": mapColours.accident,
          "circle-opacity": 0.25,
          "circle-stroke-color": mapColours.accident,
          "circle-stroke-width": 2,
        },
      });
      map.addLayer({
        id: "hotspot-rank",
        type: "symbol",
        source: "hotspots",
        layout: { "text-field": ["get", "rank"], "text-size": 13, "text-font": ["Noto Sans Bold"] },
        paint: { "text-color": "#7f1d1d", "text-halo-color": mapColours.ring, "text-halo-width": 2 },
      });

      const source = map.getSource("hotspots") as GeoJSONSource | undefined;
      source?.setData(hotspotFeatures(hotspots));
    });

    return () => map.remove();
  }, [institution, hotspots, radius]);

  return (
    <div
      ref={container}
      role="img"
      aria-label={`Karte der Umgebung von ${institutionName(institution)} mit den ${hotspots.length} stärksten Unfallschwerpunkten, nummeriert wie in der Tabelle darunter.`}
      className="h-[70mm] w-full border border-line"
    />
  );
}

/** Roughly the zoom at which the radius fills the width of the printed map. */
function zoomForRadius(radius: number): number {
  if (radius <= 250) return 16.5;
  if (radius <= 500) return 15.5;
  return 14.5;
}

function hotspotFeatures(hotspots: Hotspot[]): FeatureCollection {
  return {
    type: "FeatureCollection",
    features: hotspots.map((hotspot, index) => ({
      type: "Feature",
      geometry: { type: "Point", coordinates: [hotspot.lon, hotspot.lat] },
      properties: { rank: String(index + 1) },
    })),
  };
}

function ringFeature(institution: Institution, radius: number): FeatureCollection {
  const steps = 64;
  const coordinates: [number, number][] = [];
  const latitudeDegrees = radius / 111_320;
  const longitudeDegrees = latitudeDegrees / Math.cos((institution.lat * Math.PI) / 180);
  for (let i = 0; i <= steps; i++) {
    const angle = (i / steps) * 2 * Math.PI;
    coordinates.push([
      institution.lon + longitudeDegrees * Math.cos(angle),
      institution.lat + latitudeDegrees * Math.sin(angle),
    ]);
  }
  return {
    type: "FeatureCollection",
    features: [
      { type: "Feature", geometry: { type: "Polygon", coordinates: [coordinates] }, properties: {} },
    ],
  };
}
