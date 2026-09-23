import { expect, type Locator, type Page } from "@playwright/test";
import { PNG } from "pngjs";

/** The map colours from frontend/src/lib/palette.ts, as the user sees them. */
export const colours = {
  accident: { r: 0xd0, g: 0x3b, b: 0x3b },
  institution: { r: 0x0b, g: 0x0b, b: 0x0b },
  report: { r: 0xe8, g: 0x7b, b: 0xa4 },
  // Two steps of the overview's ramp.
  nearbyNone: { r: 0xa0, g: 0x7c, b: 0xdb },
  nearbyFiveToFourteen: { r: 0x68, g: 0x44, b: 0x9c },
};

type Colour = (typeof colours)[keyof typeof colours];

// A style with nothing in it. The real basemap comes from an external service,
// and whether our own points are drawn must not depend on somebody else being
// up — nor should a test fail on the day they redesign their map.
const emptyStyle = {
  version: 8,
  sources: {},
  layers: [{ id: "background", type: "background", paint: { "background-color": "#ffffff" } }],
  glyphs: "https://example.invalid/{fontstack}/{range}.pbf",
};

export async function withoutBasemap(page: Page) {
  await page.route("https://sgx.geodatenzentrum.de/**", (route) =>
    route.fulfill({ json: emptyStyle }),
  );
}

export async function openSchool(page: Page) {
  await page.goto("/");
  await page.getByLabel("Schule oder Kita suchen").fill("egidien");
  await page.getByRole("button", { name: "Suchen" }).click();
  await page.getByRole("link", { name: /Grundschule St. Egidien/ }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Grundschule St. Egidien");
}

/** How many pixels of the given element carry the colour. */
export async function pixelsOf(element: Locator, colour: Colour): Promise<number> {
  const png = PNG.sync.read(await element.screenshot());
  let count = 0;
  for (let i = 0; i < png.data.length; i += 4) {
    const near = (value: number, want: number) => Math.abs(value - want) <= 12;
    if (near(png.data[i], colour.r) && near(png.data[i + 1], colour.g) && near(png.data[i + 2], colour.b)) {
      count += 1;
    }
  }
  return count;
}

export function surroundingsMap(page: Page) {
  return page.getByRole("region", { name: /Karte der Umgebung/ });
}
