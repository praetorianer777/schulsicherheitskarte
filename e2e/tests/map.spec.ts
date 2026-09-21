import { expect, test, type Page } from "@playwright/test";
import { PNG } from "pngjs";

// The accident colour from frontend/src/lib/palette.ts. The check below is
// about what is on the screen, so it has to name the colour the user sees.
const accidentColour = { r: 0xd0, g: 0x3b, b: 0x3b };

// A style with nothing in it. The real basemap comes from an external service,
// and whether our own points are drawn must not depend on somebody else being
// up — nor should this test fail on the day they redesign their map.
const emptyStyle = {
  version: 8,
  sources: {},
  layers: [{ id: "background", type: "background", paint: { "background-color": "#ffffff" } }],
  glyphs: "https://example.invalid/{fontstack}/{range}.pbf",
};

async function withoutBasemap(page: Page) {
  await page.route("https://sgx.geodatenzentrum.de/**", (route) =>
    route.fulfill({ json: emptyStyle }),
  );
}

async function openSchool(page: Page) {
  await page.goto("/");
  await page.getByLabel("Schule oder Kita suchen").fill("egidien");
  await page.getByRole("button", { name: "Suchen" }).click();
  await page.getByRole("link", { name: /Grundschule St. Egidien/ }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Grundschule St. Egidien");
}

// The institution colour from the same palette.
const institutionColour = { r: 0x0b, g: 0x0b, b: 0x0b };

/** How many pixels of the given element carry the colour. */
async function pixelsOf(
  page: Page,
  selector: ReturnType<Page["locator"]>,
  colour: { r: number; g: number; b: number },
): Promise<number> {
  const png = PNG.sync.read(await selector.screenshot());
  let count = 0;
  for (let i = 0; i < png.data.length; i += 4) {
    const near = (value: number, want: number) => Math.abs(value - want) <= 12;
    if (near(png.data[i], colour.r) && near(png.data[i + 1], colour.g) && near(png.data[i + 2], colour.b)) {
      count += 1;
    }
  }
  return count;
}

async function accidentPixels(page: Page): Promise<number> {
  return pixelsOf(page, page.getByRole("region", { name: /Karte der Umgebung/ }), accidentColour);
}

test.describe("Karte im gebauten Image", () => {
  // The start page opens on the map. The zoom controls are dark too, so only
  // the canvas is photographed, not the region around it.
  test("die Startseite zeigt die Einrichtungen auf der Karte", async ({ page }) => {
    await withoutBasemap(page);
    await page.goto("/");
    await expect(page.getByRole("region", { name: /Karte der Region/ })).toBeVisible();

    const canvas = page.locator("canvas.maplibregl-canvas").first();
    await expect.poll(() => pixelsOf(page, canvas, institutionColour), { timeout: 15_000 }).toBeGreaterThan(30);
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
