import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Route, Routes } from "react-router-dom";

import InstitutionPage from "./InstitutionPage";
import { axe, renderApp } from "../test/render";
import { describeViolations } from "../test/axe";
import { accidents, hotspots, school, summary } from "../test/fixtures";
import { calls } from "../test/stubs/maplibre";

const requested: string[] = [];

function routeFetch() {
  return vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);
    requested.push(url);
    const body = (() => {
      if (url.includes("/accidents")) {
        const onlyVulnerable = url.includes("modes=foot%2Cbike") || url.includes("modes=foot,bike");
        const list = onlyVulnerable ? accidents.filter((a) => a.pedestrian || a.bike) : accidents;
        return { accidents: list, summary: { ...summary, total: list.length }, radius: 500, sources: [] };
      }
      if (url.includes("/hotspots")) return { hotspots, radius: 500, sources: [] };
      if (url.includes("/infrastructure")) return { infrastructure: [], counts: {}, radius: 500, sources: [] };
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
    { route: "/einrichtung/275" },
  );
}

describe("Einrichtungsseite", () => {
  beforeEach(() => {
    requested.length = 0;
    calls.flyTo.length = 0;
    calls.jumpTo.length = 0;
    vi.stubGlobal("fetch", routeFetch());
  });
  afterEach(() => vi.unstubAllGlobals());

  it("zeigt Name, Kennzahlen und Schwerpunkte", async () => {
    renderPage();

    expect(await screen.findByRole("heading", { name: "Peter Breuer Gymnasium" })).toBeInTheDocument();
    expect(await screen.findByText("Unfälle insgesamt")).toBeInTheDocument();

    const list = await screen.findByRole("list", { name: /Unfallschwerpunkte/ }).catch(() => null);
    // The hotspots are an ordered list, ranked.
    const ranked = list ?? screen.getByRole("heading", { name: "Unfallschwerpunkte" });
    expect(ranked).toBeInTheDocument();
    expect(screen.getByText(/20 Unfälle, 286 m entfernt/)).toBeInTheDocument();
  });

  // The map is not the only way to the information, and the list is not a
  // consolation prize: selecting an entry uses the map.
  it("bietet die Schwerpunkte als bedienbare Liste an", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByRole("heading", { name: "Unfallschwerpunkte" });

    const entries = screen.getAllByRole("button", { name: /Auf der Karte zeigen/ });
    expect(entries).toHaveLength(hotspots.length);

    await user.click(entries[0]);
    expect(calls.flyTo.length + calls.jumpTo.length).toBe(1);
  });

  it("führt jeden Unfall auch als Tabellenzeile", async () => {
    renderPage();
    const table = await screen.findByRole("table");
    const rows = within(table).getAllByRole("row");
    // One header row plus one per accident.
    expect(rows).toHaveLength(accidents.length + 1);
    expect(within(table).getByText("Getötete")).toBeInTheDocument();
  });

  it("schickt den Radius aus dem Filter an die API", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByRole("heading", { name: "Peter Breuer Gymnasium" });

    await user.click(screen.getByRole("radio", { name: "250 m" }));

    await vi.waitFor(() =>
      expect(requested.some((url) => url.includes("/accidents") && url.includes("radius=250"))).toBe(
        true,
      ),
    );
  });

  it("schaltet auf die Schulweg-Sicht um", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByRole("heading", { name: "Peter Breuer Gymnasium" });

    await user.click(
      screen.getByRole("checkbox", { name: /Nur Unfälle mit Fuß- oder Radbeteiligung/ }),
    );

    await vi.waitFor(() =>
      expect(requested.some((url) => url.includes("/accidents") && url.includes("modes="))).toBe(true),
    );
  });

  // Without it, a fact sheet built on this page claims more than the data says.
  it("nennt die Grenzen der Daten auf der Seite", async () => {
    renderPage();
    expect(await screen.findByRole("heading", { name: "Was diese Zahlen nicht hergeben" })).toBeInTheDocument();
    expect(screen.getByText(/keine Risikoraten/)).toBeInTheDocument();
  });

  // The map is a region rather than an image: its zoom controls live inside it,
  // and an image has no exposed children, so they would disappear.
  it("beschreibt die Karte für Screenreader", async () => {
    renderPage();
    const map = await screen.findByRole("region", { name: /Karte der Umgebung/ });
    expect(map).toHaveAccessibleName(/Liste unter der Karte enthält dieselben Angaben/);
  });

  it("hat keine Barrierefreiheitsverstöße", async () => {
    const { container } = renderPage();
    await screen.findByRole("heading", { name: "Unfallschwerpunkte" });

    const violations = await axe(container);
    expect(describeViolations(violations)).toBe("");
  });
});
