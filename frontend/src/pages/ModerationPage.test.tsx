import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import ModerationPage from "./ModerationPage";
import { axe, renderApp } from "../test/render";
import { describeViolations } from "../test/axe";

const pending = {
  id: "3f1d0a1e-0000-4000-8000-000000000001",
  lon: 12.62,
  lat: 50.79,
  category: "speeding",
  description: "Autos fahren morgens zu schnell.",
  status: "pending",
  createdAt: "2026-09-20T07:30:00Z",
  confirmations: 0,
};

const seen: { url: string; token: string | null; body: string | null }[] = [];

function api(ok = true) {
  return vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const headers = new Headers(init?.headers);
    seen.push({
      url: String(input),
      token: headers.get("Authorization"),
      body: (init?.body as string) ?? null,
    });
    if (!ok) {
      return {
        ok: false,
        status: 401,
        json: async () => ({ error: "Kein gültiges Moderations-Token." }),
      } as Response;
    }
    return {
      ok: true,
      status: 200,
      json: async () => ({ reports: [pending], categories: [] }),
    } as Response;
  });
}

describe("Moderation", () => {
  beforeEach(() => {
    seen.length = 0;
    vi.stubGlobal("fetch", api());
  });
  afterEach(() => vi.unstubAllGlobals());

  it("fragt erst nach dem Token", () => {
    renderApp(<ModerationPage />);
    expect(screen.getByLabelText("Moderations-Token")).toBeInTheDocument();
    expect(seen).toHaveLength(0);
  });

  it("schickt das Token als Bearer-Header mit", async () => {
    const user = userEvent.setup();
    renderApp(<ModerationPage />);

    await user.type(screen.getByLabelText("Moderations-Token"), "geheim");
    await user.click(screen.getByRole("button", { name: "Warteschlange öffnen" }));

    expect(await screen.findByText(/Autos fahren morgens zu schnell/)).toBeInTheDocument();
    expect(seen[0].token).toBe("Bearer geheim");
  });

  it("gibt eine Meldung frei", async () => {
    const user = userEvent.setup();
    renderApp(<ModerationPage />);
    await user.type(screen.getByLabelText("Moderations-Token"), "geheim");
    await user.click(screen.getByRole("button", { name: "Warteschlange öffnen" }));
    await screen.findByText(/Autos fahren morgens zu schnell/);

    await user.click(screen.getByRole("button", { name: "Freigeben" }));

    const decision = seen.find((call) => call.url.includes("/admin/reports/"));
    expect(decision?.body).toContain('"status":"approved"');
    expect(decision?.token).toBe("Bearer geheim");
  });

  it("zeigt die Begründung bei einem falschen Token", async () => {
    vi.stubGlobal("fetch", api(false));
    const user = userEvent.setup();
    renderApp(<ModerationPage />);

    await user.type(screen.getByLabelText("Moderations-Token"), "falsch");
    await user.click(screen.getByRole("button", { name: "Warteschlange öffnen" }));

    expect(await screen.findByText(/Kein gültiges Moderations-Token/)).toBeInTheDocument();
  });

  // A token in the URL or in storage stays behind on a shared machine.
  it("hält das Token nur im Formular", async () => {
    const user = userEvent.setup();
    renderApp(<ModerationPage />);
    await user.type(screen.getByLabelText("Moderations-Token"), "geheim");
    await user.click(screen.getByRole("button", { name: "Warteschlange öffnen" }));
    await screen.findByText(/Autos fahren morgens zu schnell/);

    expect(window.location.search).not.toContain("geheim");
    expect(window.localStorage.getItem("token")).toBeNull();
    expect(JSON.stringify({ ...window.sessionStorage })).not.toContain("geheim");
  });

  it("hat keine Barrierefreiheitsverstöße", async () => {
    const user = userEvent.setup();
    const { container } = renderApp(<ModerationPage />);
    await user.type(screen.getByLabelText("Moderations-Token"), "geheim");
    await user.click(screen.getByRole("button", { name: "Warteschlange öffnen" }));
    await screen.findByText(/Autos fahren morgens zu schnell/);

    expect(describeViolations(await axe(container))).toBe("");
  });
});
