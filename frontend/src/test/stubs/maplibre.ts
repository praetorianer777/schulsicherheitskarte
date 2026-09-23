/**
 * MapLibre needs WebGL, which jsdom does not have. The tests are about what the
 * page says and offers, not about what the canvas draws, so the map is replaced
 * by a stub that records the calls the components make.
 */
import { vi } from "vitest";

export const calls: {
  flyTo: unknown[];
  jumpTo: unknown[];
  fitBounds: unknown[];
  layout: unknown[];
  paint: unknown[];
  data: { source: string; features: unknown[] }[];
} = {
  flyTo: [],
  jumpTo: [],
  fitBounds: [],
  layout: [],
  paint: [],
  data: [],
};

/** The features last pushed into a source, or null if it was never filled. */
export function sourceData(source: string): unknown[] | null {
  const last = [...calls.data].reverse().find((entry) => entry.source === source);
  return last ? last.features : null;
}

// A real map is ready only after its style has been fetched, which takes far
// longer than the page's own requests. Tests that care about that order hold
// the load event back and release it themselves.
let heldHandlers: (() => void)[] = [];
let holding = false;

export function holdLoad() {
  holding = true;
  heldHandlers = [];
}

export function releaseLoad() {
  holding = false;
  const handlers = heldHandlers;
  heldHandlers = [];
  for (const handler of handlers) handler();
}

export function resetCalls() {
  holding = false;
  heldHandlers = [];
  calls.flyTo.length = 0;
  calls.jumpTo.length = 0;
  calls.fitBounds.length = 0;
  calls.layout.length = 0;
  calls.paint.length = 0;
  calls.data.length = 0;
}

class StubSource {
  constructor(private readonly id: string) {}

  setData = (data: { features?: unknown[] }) => {
    calls.data.push({ source: this.id, features: data.features ?? [] });
  };
}

/** The view a stub map reports; tests move it with `moveStubMap`. */
let bounds = { west: 12.2, south: 50.5, east: 12.8, north: 50.9 };

export function moveStubMap(next: typeof bounds) {
  bounds = next;
  for (const instance of instances) instance.fire("moveend");
}

const instances: Map[] = [];

export class Map {
  private sources: Record<string, StubSource> = {};
  private handlers: Record<string, (event?: unknown) => void> = {};

  constructor(_options: unknown) {
    instances.push(this);
  }

  fire = (event: string, payload?: unknown) => this.handlers[event]?.(payload);

  addControl = vi.fn();
  on = (event: string, layerOrHandler: unknown, maybeHandler?: (event?: unknown) => void) => {
    // Layer-scoped listeners pass the layer id first.
    const handler = (maybeHandler ?? layerOrHandler) as (event?: unknown) => void;
    this.handlers[event] = handler;
    // A browser fires load after the current task, never from inside on().
    // Firing it synchronously made the map ready before the first effect ever
    // ran, which hid the race that dropped everything arriving in between.
    if (event !== "load") return;
    if (holding) heldHandlers.push(handler);
    else queueMicrotask(handler);
  };
  addSource = (id: string) => {
    this.sources[id] = new StubSource(id);
  };
  getSource = (id: string) => this.sources[id];
  addLayer = vi.fn();
  getLayer = () => ({});
  setLayoutProperty = (...args: unknown[]) => calls.layout.push(args);
  setPaintProperty = (...args: unknown[]) => calls.paint.push(args);
  flyTo = (...args: unknown[]) => calls.flyTo.push(args);
  jumpTo = (...args: unknown[]) => calls.jumpTo.push(args);
  fitBounds = (...args: unknown[]) => calls.fitBounds.push(args);
  easeTo = vi.fn();
  getZoom = () => 10;
  getCanvas = () => ({ style: {} as CSSStyleDeclaration });
  queryRenderedFeatures = () => [];
  getBounds = () => ({
    getWest: () => bounds.west,
    getSouth: () => bounds.south,
    getEast: () => bounds.east,
    getNorth: () => bounds.north,
  });
  remove = () => {
    const at = instances.indexOf(this);
    if (at >= 0) instances.splice(at, 1);
  };
}

export class NavigationControl {
  constructor(_options: unknown) {}
}

export type GeoJSONSource = StubSource;

export class Popup {
  constructor(_options?: unknown) {}
  setLngLat = () => this;
  setText = () => this;
  addTo = () => this;
  remove = () => this;
}

export default { Map, NavigationControl, Popup };

