import { screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Route, Routes } from "react-router-dom";

import FactsheetPage from "./FactsheetPage";
import { axe, renderApp } from "../test/render";
import { describeViolations } from "../test/axe";
import { hotspots, school, summary } from "../test/fixtures";

const requested: string[] = [];

const sheet = {
  institution: school,
  radius: 500,
  years: { from: 2016, to: 2025 },
  summary,
  hotspots,
  infrastructureCounts: { crossing: 25, traffic_signals: 19, speed_limit: 138 },
  method: {
    weightFatal: 10,
    weightSerious: 5,
    weightSlight: 1,
    weightVulnerable: 3,
    halfLifeYears: 4,
    clusterRadiusMetres: 50,
    clusterMinAccidents: 2,
    referenceYear: 2025,
  },
  sources: [
    { name: "Unfallatlas der Statistischen Ämter", licence: "dl-de/by-2-0", url: "https://unfallatlas.statistikportal.de/" },
    { name: "© OpenStreetMap-Mitwirkende", licence: "ODbL", url: "https://www.openstreetmap.org/copyright" },
  ],
};

function renderSheet(search = "?radius=500&from=2016&to=2025") {
  return renderApp(
    <Routes>
      <Route path="/einrichtung/:id/faktenblatt" element={<FactsheetPage />} />
    </Routes>,
    { route: `/einrichtung/275/faktenblatt${search}` },
  );
}

describe("Faktenblatt", () => {
  beforeEach(() => {
    requested.length = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        requested.push(String(input));
        return { ok: true, status: 200, json: async () => sheet } as Response;
      }),
    );
  });
  afterEach(() => vi.unstubAllGlobals());

  it("nennt Einrichtung, Umkreis und Zeitraum", async () => {
    renderSheet();
    expect(
      await screen.findByRole("heading", { name: /Schulwegsicherheit: Peter Breuer Gymnasium/ }),
    ).toBeInTheDocument();
    expect(screen.getByText(/Umkreis 500 m · Berichtsjahre 2016–2025/)).toBeInTheDocument();
  });

  it("richtet sich an die Straßenverkehrsbehörde", async () => {
    renderSheet();
    expect(await screen.findByText("An die Straßenverkehrsbehörde")).toBeInTheDocument();
  });

  // Without the method the sheet states a number it cannot explain, and the
  // first question in the meeting has no answer.
  it("erklärt, wie der Gefahrenindex zustande kommt", async () => {
    renderSheet();
    await screen.findByRole("heading", { name: "Wie der Gefahrenindex berechnet wird" });
    expect(screen.getByText(/Getötete × 10/)).toBeInTheDocument();
    expect(screen.getByText(/halbiert alle 4 Jahre/)).toBeInTheDocument();
    expect(screen.getByText(/Berichtsjahr 2025/)).toBeInTheDocument();
  });

  // Without this section the sheet claims more than the data says, and gets
  // taken apart at the first road safety inspection.
  it("nennt die Grenzen der Daten", async () => {
    renderSheet();
    await screen.findByRole("heading", { name: "Was diese Zahlen nicht hergeben" });
    expect(screen.getByText(/keine Risikoraten/)).toBeInTheDocument();
    expect(screen.getByText(/belegt nicht, dass sie sicher ist/)).toBeInTheDocument();
  });

  // Both licences require attribution, and a sheet that is printed and handed
  // over carries no footer of the website with it.
  it("führt die Quellen samt Lizenzen auf", async () => {
    renderSheet();
    await screen.findByText(/Quellen:/);
    expect(screen.getByText(/dl-de\/by-2-0/)).toBeInTheDocument();
    expect(screen.getByText(/ODbL/)).toBeInTheDocument();
    expect(screen.getByText(/basemap.de/)).toBeInTheDocument();
  });

  it("führt die Schwerpunkte als Tabelle mit Rang", async () => {
    renderSheet();
    const table = (await screen.findAllByRole("table"))[1];
    const rows = within(table).getAllByRole("row");
    expect(rows).toHaveLength(hotspots.length + 1);
    expect(within(table).getByText("Gefahrenindex")).toBeInTheDocument();
  });

  // The link a parent sends on has to produce the sheet they saw, not a
  // default one.
  it("übernimmt Umkreis und Zeitraum aus der Adresse", async () => {
    renderSheet("?radius=250&from=2021&to=2025&modes=foot,bike");
    await screen.findByRole("heading", { name: /Schulwegsicherheit/ });

    const call = requested.find((url) => url.includes("/factsheet"));
    expect(call).toContain("radius=250");
    expect(call).toContain("from=2021");
    expect(call).toContain("modes=foot");
  });

  it("hat keine Barrierefreiheitsverstöße", async () => {
    const { container } = renderSheet();
    await screen.findByRole("heading", { name: /Schulwegsicherheit/ });
    expect(describeViolations(await axe(container))).toBe("");
  });
});
