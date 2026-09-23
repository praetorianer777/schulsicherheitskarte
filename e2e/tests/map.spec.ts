import { expect, test, type Page } from "@playwright/test";

import { colours, openSchool, pixelsOf, surroundingsMap, withoutBasemap } from "./support";

async function accidentPixels(page: Page): Promise<number> {
  return pixelsOf(surroundingsMap(page), colours.accident);
}

test.describe("Karte im gebauten Image", () => {
  // The start page opens on the map. The zoom controls are dark too, so only
  // the canvas is photographed, not the region around it.
  test("die Startseite zeigt die Einrichtungen auf der Karte", async ({ page }) => {
    await withoutBasemap(page);
    await page.goto("/");
    await expect(page.getByRole("region", { name: /Karte der Region/ })).toBeVisible();

    const canvas = page.locator("canvas.maplibregl-canvas").first();
    await expect.poll(() => pixelsOf(canvas, colours.institution), { timeout: 15_000 }).toBeGreaterThan(30);
  });

  // The map used to draw the basemap and the radius ring and nothing else:
  // everything that arrived while MapLibre was still assembling itself was
  // dropped. Every test stayed green, because they all looked at the list and
  // the table beside the canvas.
  test("zeichnet die Unfälle wirklich auf die Karte", async ({ page }) => {
    await withoutBasemap(page);
    await openSchool(page);

    await expect.poll(() => accidentPixels(page), { timeout: 15_000 }).toBeGreaterThan(50);
  });

  test("die Unfälle verschwinden, wenn die Ebene ausgeschaltet wird", async ({ page }) => {
    await withoutBasemap(page);
    await openSchool(page);
    await expect.poll(() => accidentPixels(page), { timeout: 15_000 }).toBeGreaterThan(50);

    // The hotspot rings are drawn in the same colour, so both layers go.
    await page.getByRole("checkbox", { name: "Unfälle", exact: true }).uncheck();
    await page.getByRole("checkbox", { name: "Unfallschwerpunkte" }).uncheck();
    await expect.poll(() => accidentPixels(page)).toBe(0);
  });

  // MapLibre asks for its worker by a path the bundler has to have emitted. It
  // had not, so the map stayed empty in every deployment while every test was
  // green: the map tests only ever touched the list and the table beside it.
  test("der Kartenworker wird ausgeliefert, nicht die Fehlerseite", async ({ page }) => {
    const workers: { url: string; status: number; type: string }[] = [];
    page.on("response", async (response) => {
      if (!response.url().includes("maplibre-gl-worker")) return;
      workers.push({
        url: response.url(),
        status: response.status(),
        type: response.headers()["content-type"] ?? "",
      });
    });

    await openSchool(page);
    await expect.poll(() => workers.length).toBeGreaterThan(0);

    for (const worker of workers) {
      expect(worker.status, `${worker.url} antwortete ${worker.status}`).toBe(200);
      expect(worker.type, `${worker.url} ist ${worker.type}`).toContain("javascript");
    }
  });

  // The same mistake with any other asset would be just as invisible, so the
  // check is not about this one file.
  test("kein Verweis der Seite läuft ins Leere", async ({ page, baseURL }) => {
    const missing: string[] = [];
    page.on("response", (response) => {
      if (!response.url().startsWith(baseURL!)) return;
      if (response.status() === 404) missing.push(new URL(response.url()).pathname);
    });

    await openSchool(page);
    expect(missing).toEqual([]);
  });
});
