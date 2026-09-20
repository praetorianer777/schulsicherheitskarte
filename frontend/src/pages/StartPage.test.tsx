import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import StartPage from "./StartPage";
import { axe, renderApp } from "../test/render";
import { describeViolations } from "../test/axe";
import { school } from "../test/fixtures";

function respondWith(body: unknown, ok = true, status = 200) {
  return vi.fn().mockResolvedValue({
    ok,
    status,
    json: async () => body,
  } as Response);
}

describe("Startseite", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", respondWith({ institutions: [school], sources: [] }));
  });
  afterEach(() => vi.unstubAllGlobals());

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

    expect(await screen.findByText(/noch keine Daten importiert/)).toBeInTheDocument();
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
