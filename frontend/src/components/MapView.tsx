import { Map as MapLibreMap, NavigationControl } from "maplibre-gl";
import type { GeoJSONSource } from "maplibre-gl";
import { useEffect, useRef } from "react";
import type { FeatureCollection } from "geojson";

import type { Accident, Hotspot, Infrastructure, Institution } from "../api/types";
import { mapColours, severityRadius } from "../lib/palette";
import { institutionName } from "../lib/format";

/** The official German basemap: no key, and it carries its own attribution. */
const styleURL = "https://sgx.geodatenzentrum.de/gdz_basemapde_vektor/styles/bm_web_col.json";

export type LayerVisibility = {
  accidents: boolean;
  hotspots: boolean;
  infrastructure: boolean;
};

type Props = {
  institution: Institution;
  accidents: Accident[];
  hotspots: Hotspot[];
  infrastructure: Infrastructure[];
  radius: number;
  layers: LayerVisibility;
  focus?: { lon: number; lat: number } | null;
};

const empty: FeatureCollection = { type: "FeatureCollection", features: [] };

function accidentFeatures(accidents: Accident[]): FeatureCollection {
  return {
    type: "FeatureCollection",
    features: accidents.map((a) => ({
      type: "Feature",
      geometry: { type: "Point", coordinates: [a.lon, a.lat] },
      properties: { radius: severityRadius[a.severity] },
    })),
  };
}

function hotspotFeatures(hotspots: Hotspot[]): FeatureCollection {
  const strongest = Math.max(1, ...hotspots.map((h) => h.score));
  return {
    type: "FeatureCollection",
    features: hotspots.map((h, index) => ({
      type: "Feature",
      geometry: { type: "Point", coordinates: [h.lon, h.lat] },
      properties: {
        // Area, not radius, follows the score: a circle drawn with twice the
        // radius looks four times as bad as it is.
        radius: 10 + 26 * Math.sqrt(h.score / strongest),
        rank: index < 5 ? String(index + 1) : "",
      },
    })),
  };
}

function infrastructureFeatures(items: Infrastructure[]): FeatureCollection {
  return {
    type: "FeatureCollection",
    features: items.map((item) => ({
      type: "Feature",
      geometry: item.geometry,
      properties: { kind: item.kind, maxspeed: item.tags?.maxspeed ?? "" },
    })),
  };
}

/** A ring of the chosen radius, so "within 500 m" is something you can see. */
function radiusFeature(institution: Institution, radius: number): FeatureCollection {
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
    features: [{ type: "Feature", geometry: { type: "Polygon", coordinates: [coordinates] }, properties: {} }],
  };
}

export default function MapView({
  institution,
  accidents,
  hotspots,
  infrastructure,
  radius,
  layers,
  focus,
}: Props) {
  const container = useRef<HTMLDivElement>(null);
  const map = useRef<MapLibreMap | null>(null);
  const ready = useRef(false);

  useEffect(() => {
    if (!container.current || map.current) return;

    const instance = new MapLibreMap({
      container: container.current,
      style: styleURL,
      center: [institution.lon, institution.lat],
      zoom: 15,
      // The canvas is not reachable by keyboard in any useful way; the list
      // beside it is the equivalent, and it is announced as such.
      attributionControl: { compact: true },
    });
    instance.addControl(new NavigationControl({ showCompass: false }), "top-right");
    map.current = instance;

    instance.on("load", () => {
      instance.addSource("radius", { type: "geojson", data: empty });
      instance.addSource("hotspots", { type: "geojson", data: empty });
      instance.addSource("accidents", { type: "geojson", data: empty });
      instance.addSource("infrastructure", { type: "geojson", data: empty });

      instance.addLayer({
        id: "radius-fill",
        type: "fill",
        source: "radius",
        paint: { "fill-color": mapColours.institution, "fill-opacity": 0.04 },
      });
      instance.addLayer({
        id: "radius-line",
        type: "line",
        source: "radius",
        paint: { "line-color": mapColours.institution, "line-width": 1.5, "line-dasharray": [3, 3] },
      });

      instance.addLayer({
        id: "speed-limits",
        type: "line",
        source: "infrastructure",
        filter: ["==", ["get", "kind"], "speed_limit"],
        paint: { "line-color": mapColours.speedLimit, "line-width": 3, "line-opacity": 0.75 },
      });

      instance.addLayer({
        id: "hotspot-circles",
        type: "circle",
        source: "hotspots",
        paint: {
          "circle-radius": ["get", "radius"],
          "circle-color": mapColours.accident,
          "circle-opacity": 0.18,
          "circle-stroke-color": mapColours.accident,
          "circle-stroke-width": 2,
        },
      });
      instance.addLayer({
        id: "hotspot-rank",
        type: "symbol",
        source: "hotspots",
        layout: { "text-field": ["get", "rank"], "text-size": 13, "text-font": ["Noto Sans Bold"] },
        paint: { "text-color": "#7f1d1d", "text-halo-color": mapColours.ring, "text-halo-width": 2 },
      });

      instance.addLayer({
        id: "infrastructure-points",
        type: "circle",
        source: "infrastructure",
        filter: ["!=", ["get", "kind"], "speed_limit"],
        paint: {
          "circle-radius": 5,
          "circle-color": [
            "match",
            ["get", "kind"],
            "crossing",
            mapColours.crossing,
            "traffic_signals",
            mapColours.trafficSignals,
            "traffic_calming",
            mapColours.trafficCalming,
            mapColours.crossing,
          ],
          "circle-stroke-color": mapColours.ring,
          "circle-stroke-width": 1.5,
        },
      });

      instance.addLayer({
        id: "accident-points",
        type: "circle",
        source: "accidents",
        paint: {
          "circle-radius": ["get", "radius"],
          "circle-color": mapColours.accident,
          // A ring in the surface colour keeps overlapping points apart.
          "circle-stroke-color": mapColours.ring,
          "circle-stroke-width": 1.5,
        },
      });

      instance.addLayer({
        id: "institution-point",
        type: "circle",
        source: "radius",
        paint: { "circle-radius": 0 },
      });

      ready.current = true;
      setData(instance, "radius", radiusFeature(institution, radius));
      setData(instance, "accidents", accidentFeatures(accidents));
      setData(instance, "hotspots", hotspotFeatures(hotspots));
      setData(instance, "infrastructure", infrastructureFeatures(infrastructure));
    });

    return () => {
      instance.remove();
      map.current = null;
      ready.current = false;
    };
    // The map is created once for an institution; everything else is pushed in
    // through the effects below rather than by rebuilding it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [institution.id]);

  useEffect(() => {
    if (map.current && ready.current) {
      setData(map.current, "radius", radiusFeature(institution, radius));
    }
  }, [institution, radius]);

  useEffect(() => {
    if (map.current && ready.current) setData(map.current, "accidents", accidentFeatures(accidents));
  }, [accidents]);

  useEffect(() => {
    if (map.current && ready.current) setData(map.current, "hotspots", hotspotFeatures(hotspots));
  }, [hotspots]);

  useEffect(() => {
    if (map.current && ready.current) {
      setData(map.current, "infrastructure", infrastructureFeatures(infrastructure));
    }
  }, [infrastructure]);

  useEffect(() => {
    const instance = map.current;
    if (!instance || !ready.current) return;
    setVisible(instance, ["accident-points"], layers.accidents);
    setVisible(instance, ["hotspot-circles", "hotspot-rank"], layers.hotspots);
    setVisible(instance, ["infrastructure-points", "speed-limits"], layers.infrastructure);
  }, [layers]);

  useEffect(() => {
    const instance = map.current;
    if (!instance || !focus) return;
    const reduced = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
    const target = { center: [focus.lon, focus.lat] as [number, number], zoom: 17 };
    if (reduced) instance.jumpTo(target);
    else instance.flyTo({ ...target, duration: 800 });
  }, [focus]);

  return (
    <div
      ref={container}
      role="img"
      aria-label={`Karte der Umgebung von ${institutionName(institution)} mit ${accidents.length} Unfällen im Umkreis von ${radius} Metern. Die Liste unter der Karte enthält dieselben Angaben.`}
      className="h-[420px] w-full rounded border border-line md:h-[560px]"
    />
  );
}

function setData(instance: MapLibreMap, id: string, data: FeatureCollection) {
  const source = instance.getSource(id) as GeoJSONSource | undefined;
  source?.setData(data);
}

function setVisible(instance: MapLibreMap, ids: string[], visible: boolean) {
  for (const id of ids) {
    if (instance.getLayer(id)) {
      instance.setLayoutProperty(id, "visibility", visible ? "visible" : "none");
    }
  }
}
