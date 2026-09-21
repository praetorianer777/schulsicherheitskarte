import { Map as MapLibreMap, NavigationControl } from "maplibre-gl";
import type { GeoJSONSource, MapMouseEvent } from "maplibre-gl";
import { useEffect, useRef, useState } from "react";
import type { FeatureCollection } from "geojson";

import type { Institution } from "../api/types";
import { mapColours } from "../lib/palette";

/** The official German basemap: no key, and it carries its own attribution. */
const styleURL = "https://sgx.geodatenzentrum.de/gdz_basemapde_vektor/styles/bm_web_col.json";

export type BBox = [number, number, number, number];

type Props = {
  /** Where to open: the extent of the imported data. */
  extent: BBox;
  /** The institutions inside the current view. */
  institutions: Institution[];
  /** Search hits to bring into view; one hit zooms in, several are framed. */
  focus: Institution[] | null;
  onViewChange: (bbox: BBox) => void;
  onSelect: (id: number) => void;
};

function institutionFeatures(institutions: Institution[]): FeatureCollection {
  return {
    type: "FeatureCollection",
    features: institutions.map((i) => ({
      type: "Feature",
      id: i.id,
      geometry: { type: "Point", coordinates: [i.lon, i.lat] },
      properties: { id: i.id, kind: i.kind, name: i.name ?? "" },
    })),
  };
}

function boundsOf(points: { lon: number; lat: number }[]): BBox {
  const lons = points.map((p) => p.lon);
  const lats = points.map((p) => p.lat);
  return [Math.min(...lons), Math.min(...lats), Math.max(...lons), Math.max(...lats)];
}

export default function OverviewMap({ extent, institutions, focus, onViewChange, onSelect }: Props) {
  const container = useRef<HTMLDivElement>(null);
  const map = useRef<MapLibreMap | null>(null);
  // State, not a ref, for the same reason as in MapView: what arrives while the
  // map is still being built has to be pushed in once it is ready.
  const [ready, setReady] = useState(false);
  const select = useRef(onSelect);
  const viewChange = useRef(onViewChange);
  useEffect(() => {
    select.current = onSelect;
    viewChange.current = onViewChange;
  }, [onSelect, onViewChange]);

  useEffect(() => {
    if (!container.current || map.current) return;

    const instance = new MapLibreMap({
      container: container.current,
      style: styleURL,
      bounds: [
        [extent[0], extent[1]],
        [extent[2], extent[3]],
      ],
      fitBoundsOptions: { padding: 24 },
      attributionControl: { compact: true },
    });
    instance.addControl(new NavigationControl({ showCompass: false }), "top-right");
    map.current = instance;

    instance.on("load", () => {
      // Clustered, or the district is one smear of dots at the opening zoom.
      instance.addSource("institutions", {
        type: "geojson",
        data: institutionFeatures([]),
        cluster: true,
        clusterRadius: 40,
        clusterMaxZoom: 13,
      });

      instance.addLayer({
        id: "institution-clusters",
        type: "circle",
        source: "institutions",
        filter: ["has", "point_count"],
        paint: {
          "circle-color": mapColours.institution,
          "circle-opacity": 0.85,
          "circle-radius": ["step", ["get", "point_count"], 14, 10, 18, 50, 24],
          "circle-stroke-color": mapColours.ring,
          "circle-stroke-width": 2,
        },
      });
      instance.addLayer({
        id: "institution-cluster-count",
        type: "symbol",
        source: "institutions",
        filter: ["has", "point_count"],
        layout: {
          "text-field": ["get", "point_count_abbreviated"],
          "text-size": 12,
          "text-font": ["Noto Sans Bold"],
        },
        paint: { "text-color": mapColours.ring },
      });
      instance.addLayer({
        id: "institution-points",
        type: "circle",
        source: "institutions",
        filter: ["!", ["has", "point_count"]],
        paint: {
          "circle-radius": 7,
          "circle-color": mapColours.institution,
          "circle-stroke-color": mapColours.ring,
          "circle-stroke-width": 2,
        },
      });

      instance.on("click", "institution-points", (event: MapMouseEvent) => {
        const feature = instance.queryRenderedFeatures(event.point, {
          layers: ["institution-points"],
        })[0];
        const id = feature?.properties?.id;
        if (id != null) select.current(Number(id));
      });
      instance.on("click", "institution-clusters", (event: MapMouseEvent) => {
        instance.easeTo({ center: event.lngLat, zoom: instance.getZoom() + 2 });
      });
      for (const layer of ["institution-points", "institution-clusters"]) {
        instance.on("mouseenter", layer, () => {
          instance.getCanvas().style.cursor = "pointer";
        });
        instance.on("mouseleave", layer, () => {
          instance.getCanvas().style.cursor = "";
        });
      }

      instance.on("moveend", () => {
        const b = instance.getBounds();
        viewChange.current([b.getWest(), b.getSouth(), b.getEast(), b.getNorth()]);
      });

      setReady(true);
      const b = instance.getBounds();
      viewChange.current([b.getWest(), b.getSouth(), b.getEast(), b.getNorth()]);
    });

    return () => {
      instance.remove();
      map.current = null;
      setReady(false);
    };
    // Created once; the extent is where it opens, not something it follows.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!map.current || !ready) return;
    const source = map.current.getSource("institutions") as GeoJSONSource | undefined;
    source?.setData(institutionFeatures(institutions));
  }, [institutions, ready]);

  useEffect(() => {
    const instance = map.current;
    if (!instance || !ready || !focus || focus.length === 0) return;
    const reduced = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
    if (focus.length === 1) {
      const target = { center: [focus[0].lon, focus[0].lat] as [number, number], zoom: 15 };
      if (reduced) instance.jumpTo(target);
      else instance.flyTo({ ...target, duration: 800 });
      return;
    }
    const box = boundsOf(focus);
    instance.fitBounds(
      [
        [box[0], box[1]],
        [box[2], box[3]],
      ],
      { padding: 48, duration: reduced ? 0 : 800, maxZoom: 15 },
    );
  }, [focus, ready]);

  return (
    <div
      ref={container}
      // A region rather than an image, so the zoom controls inside stay
      // reachable — see MapView.
      role="region"
      aria-label={`Karte der Region mit ${institutions.length} Schulen und Kitas im Ausschnitt. Die Suche daneben führt zu denselben Einrichtungen.`}
      className="h-[420px] w-full rounded border border-line lg:h-[620px]"
    />
  );
}
