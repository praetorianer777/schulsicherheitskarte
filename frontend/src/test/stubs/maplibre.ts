/**
 * MapLibre needs WebGL, which jsdom does not have. The tests are about what the
 * page says and offers, not about what the canvas draws, so the map is replaced
 * by a stub that records the calls the components make.
 */
import { vi } from "vitest";

export const calls: { flyTo: unknown[]; jumpTo: unknown[]; layout: unknown[] } = {
  flyTo: [],
  jumpTo: [],
  layout: [],
};

class StubSource {
  setData = vi.fn();
}

export class Map {
  private sources: Record<string, StubSource> = {};
  private handlers: Record<string, () => void> = {};

  constructor(_options: unknown) {}

  addControl = vi.fn();
  on = (event: string, handler: () => void) => {
    this.handlers[event] = handler;
    if (event === "load") handler();
  };
  addSource = (id: string) => {
    this.sources[id] = new StubSource();
  };
  getSource = (id: string) => this.sources[id];
  addLayer = vi.fn();
  getLayer = () => ({});
  setLayoutProperty = (...args: unknown[]) => calls.layout.push(args);
  flyTo = (...args: unknown[]) => calls.flyTo.push(args);
  jumpTo = (...args: unknown[]) => calls.jumpTo.push(args);
  remove = vi.fn();
}

export class NavigationControl {
  constructor(_options: unknown) {}
}

export type GeoJSONSource = StubSource;
export default { Map, NavigationControl };
