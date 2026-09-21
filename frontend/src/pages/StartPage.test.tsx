import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import StartPage from "./StartPage";
import { axe, renderApp } from "../test/render";
import { describeViolations } from "../test/axe";
import { school } from "../test/fixtures";
import { calls, moveStubMap, resetCalls, sourceData } from "../test/stubs/maplibre";

const region = { bbox: [12.2263668, 50.54656, 12.8061082, 50.9242066], institutions: 3 };

/** Answers the extent with the region and everything else with `body`. */
function respondWith(body: unknown, ok = true, status = 200, extent: unknown = region) {
  return vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);
    const answer = url.includes("/extent") ? extent : body;
    return { ok, status, json: async () => answer } as Response;
  });
}

describe("Startseite", () => {
  beforeEach(() => {
    resetCalls();
    vi.stubGlobal("fetch", respondWith({ institutions: [school], sources: [] }));
  });
  afterEach(() => vi.unstubAllGlobals());

  it("öffnet die Karte auf dem importierten Gebiet", async () => {
    renderApp(<StartPage />);
    const map = await screen.findByRole("region", { name: /Karte der Region/ });
    expect(map).toBeInTheDocument();
    // What is in view comes from the map's own bounds, not from the search.
    await waitFor(() => expect(sourceData("institutions")).toHaveLength(1));
  });

  it("lädt nach, was beim Verschieben in den Ausschnitt kommt", async () => {
    renderApp(<StartPage />);
    await screen.findByRole("region", { name: /Karte der Region/ });
    await waitFor(() => expect(sourceData("institutions")).toHaveLength(1));

    const fetchMock = globalThis.fetch as ReturnType<typeof vi.fn>;
    const before = fetchMock.mock.calls.length;
    moveStubMap({ west: 12.4, south: 50.6, east: 12.5, north: 50.7 });
    await waitFor(() => expect(fetchMock.mock.calls.length).toBeGreaterThan(before));
    const last = String(fetchMock.mock.calls.at(-1)?.[0]);
    expect(last).toContain("bbox=12.4");
  });

  it("ein Treffer zoomt die Karte dorthin", async () => {
    const user = userEvent.setup();
    renderApp(<StartPage />);
    await screen.findByRole("region", { name: /Karte der Region/ });

    await user.type(screen.getByLabelText("Schule oder Kita suchen"), "breuer");
    await user.click(screen.getByRole("button", { name: "Suchen" }));
    await screen.findByRole("link", { name: /Peter Breuer/ });

    await waitFor(() => expect(calls.flyTo.length + calls.jumpTo.length).toBeGreaterThan(0));
    const target = (calls.flyTo[0] ?? calls.jumpTo[0]) as [{ center: [number, number] }];
    expect(target[0].center).toEqual([school.lon, school.lat]);
  });

  it("meldet die leere Datenbank statt einer leeren Karte", async () => {
    vi.stubGlobal(
      "fetch",
      respondWith({ institutions: [], sources: [] }, true, 200, { bbox: null, institutions: 0 }),
    );
    renderApp(<StartPage />);
    expect(await screen.findByText(/noch keine Daten importiert/)).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: /Karte der Region/ })).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Schule oder Kita suchen")).not.toBeInTheDocument();
  });

  it("findet eine Schule über das Suchfeld", async () => {
    const user = userEvent.setup();
    renderApp(<StartPage />);

    await user.type(screen.getByLabelText("Schule oder Kita suchen"), "breuer");
    await user.click(screen.getByRole("button", { name: "Suchen" }));

    expect(await screen.findByRole("link", { name: /Peter Breuer Gymnasium/ })).toHaveAttribute(
      "href",
      "/einrichtung/275",
    );
  });

  // Somebody who cannot see the list appearing below the field still has to be
  // told that it did.
  it("meldet Ergebnisse in einer Live-Region", async () => {
    const user = userEvent.setup();
    const { container } = renderApp(<StartPage />);

    await user.type(screen.getByLabelText("Schule oder Kita suchen"), "breuer");
    await user.click(screen.getByRole("button", { name: "Suchen" }));
    await screen.findByRole("link", { name: /Peter Breuer/ });

    const live = container.querySelector("[aria-live='polite']");
    expect(live).not.toBeNull();
    expect(live).toHaveTextContent(/Peter Breuer/);
  });

  it("erklärt, wenn nichts gefunden wurde", async () => {
    vi.stubGlobal("fetch", respondWith({ institutions: [], sources: [] }));
    const user = userEvent.setup();
    renderApp(<StartPage />);

    await user.type(screen.getByLabelText("Schule oder Kita suchen"), "gibtsnicht");
    await user.click(screen.getByRole("button", { name: "Suchen" }));

    expect(await screen.findByText(/wurde nichts gefunden/)).toBeInTheDocument();
  });

  // An installation that has not been imported answers every search this way,
  // and "nothing found" would send the operator looking at the search instead
  // of at the import.
  it("unterscheidet eine leere Datenbank von einem leeren Treffer", async () => {
    vi.stubGlobal("fetch", respondWith({ institutions: [], nothingImported: true, sources: [] }));
    const user = userEvent.setup();
    renderApp(<StartPage />);

    await user.type(screen.getByLabelText("Schule oder Kita suchen"), "grundschule");
    await user.click(screen.getByRole("button", { name: "Suchen" }));

    expect(await screen.findByText(/noch keine Daten importiert, deshalb/)).toBeInTheDocument();
    expect(screen.queryByText(/wurde nichts gefunden/)).not.toBeInTheDocument();
  });

  // The API names the parameter at fault and why; throwing that away would
  // leave the person with nothing to act on.
  it("zeigt die Begründung der API bei einem Fehler", async () => {
    vi.stubGlobal(
      "fetch",
      respondWith({ error: "99999 m is outside 50–2000 m", parameter: "radius" }, false, 400),
    );
    const user = userEvent.setup();
    renderApp(<StartPage />);

    await user.type(screen.getByLabelText("Schule oder Kita suchen"), "breuer");
    await user.click(screen.getByRole("button", { name: "Suchen" }));

    expect(await screen.findByText(/99999 m is outside/)).toBeInTheDocument();
  });

  it("hat keine Barrierefreiheitsverstöße", async () => {
    const user = userEvent.setup();
    const { container } = renderApp(<StartPage />);
    await user.type(screen.getByLabelText("Schule oder Kita suchen"), "breuer");
    await user.click(screen.getByRole("button", { name: "Suchen" }));
    await screen.findByRole("link", { name: /Peter Breuer/ });

    const violations = await axe(container);
    expect(describeViolations(violations)).toBe("");
  });

  it("ist vollständig mit der Tastatur bedienbar", async () => {
    const user = userEvent.setup();
    renderApp(<StartPage />);

    await user.tab();
    expect(screen.getByLabelText("Schule oder Kita suchen")).toHaveFocus();
    await user.keyboard("breuer{Enter}");

    await waitFor(() =>
      expect(screen.getByRole("link", { name: /Peter Breuer/ })).toBeInTheDocument(),
    );
  });
});
