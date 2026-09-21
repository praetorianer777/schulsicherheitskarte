import { act, screen, waitFor } from "@testing-library/react";
import { Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import InstitutionPage from "../pages/InstitutionPage";
import { renderApp } from "../test/render";
import { accidents, hotspots, school, summary } from "../test/fixtures";
import { holdLoad, releaseLoad, resetCalls, sourceData } from "../test/stubs/maplibre";

// The page is driven through its real requests rather than through props,
// because what matters here is *when* each answer arrives.
let releaseFigures: () => void;

function routeFetch(figures: Promise<void>) {
  return vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);

    // The institution answers first — the map is built from it. The figures
    // follow while the map is still assembling itself, which is the order a
    // browser produces and the order in which the map stayed empty.
    if (!url.endsWith(`/${school.id}`)) await figures;

    const body = (() => {
      if (url.includes("/accidents")) return { accidents, summary, radius: 500, sources: [] };
      if (url.includes("/hotspots")) return { hotspots, radius: 500, sources: [] };
      if (url.includes("/infrastructure")) {
        return { infrastructure: [], counts: {}, radius: 500, sources: [] };
      }
      if (url.includes("/reports")) return { reports: [], categories: [], sources: [] };
      return { institution: school, sources: [] };
    })();
    return { ok: true, status: 200, json: async () => body } as Response;
  });
}

function renderPage() {
  return renderApp(
    <Routes>
      <Route path="/einrichtung/:id" element={<InstitutionPage />} />
    </Routes>,
    { route: `/einrichtung/${school.id}` },
  );
}

describe("Karte der Einrichtungsseite", () => {
  beforeEach(() => {
    resetCalls();
    const figures = new Promise<void>((resolve) => {
      releaseFigures = resolve;
    });
    vi.stubGlobal("fetch", routeFetch(figures));
  });
  afterEach(() => vi.unstubAllGlobals());

  // Everything that arrived while MapLibre was still building used to be
  // dropped: the effect found the map not ready, and the load handler then
  // filled the sources from the props captured when the map was created. The
  // page showed a basemap and nothing on it.
  it("zeichnet die Unfälle, die während des Kartenaufbaus eintreffen", async () => {
    holdLoad();
    renderPage();
    await screen.findByRole("heading", { level: 1, name: school.name! });

    await act(async () => releaseFigures());
    await screen.findByText(String(summary.total));
    expect(sourceData("accidents")).toBeNull();

    await act(async () => releaseLoad());

    await waitFor(() => expect(sourceData("accidents")).toHaveLength(accidents.length));
  });

  it("zeichnet die Schwerpunkte, die während des Kartenaufbaus eintreffen", async () => {
    holdLoad();
    renderPage();
    await screen.findByRole("heading", { level: 1, name: school.name! });

    await act(async () => releaseFigures());
    await screen.findByText(String(summary.total));

    await act(async () => releaseLoad());

    await waitFor(() => expect(sourceData("hotspots")).toHaveLength(hotspots.length));
  });
});
