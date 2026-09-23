import { expect, test, type Page } from "@playwright/test";

import { openSchool } from "./support";

/** The figure in the summary table of the sheet. */
function row(page: Page, label: string) {
  return page.getByRole("row", { name: new RegExp(`^${label}`) }).getByRole("cell");
}

async function openSheet(page: Page) {
  await openSchool(page);
  await page.getByRole("link", { name: "Faktenblatt zum Ausdrucken" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(
    "Schulwegsicherheit: Grundschule St. Egidien",
  );
}

test.describe("Faktenblatt", () => {
  test("wird für die Einrichtung erstellt", async ({ page }) => {
    await openSheet(page);

    await expect(page).toHaveURL(/\/einrichtung\/\d+\/faktenblatt\?radius=500/);
    await expect(page.getByText("An die Straßenverkehrsbehörde")).toBeVisible();
    await expect(row(page, "Unfälle insgesamt")).toHaveText("6");
    await expect(row(page, "davon mit Getöteten")).toHaveText("1");
    await expect(row(page, "mit Beteiligung zu Fuß")).toHaveText("2");
  });

  test("nennt Methode, Gewichte und Grenzen", async ({ page }) => {
    await openSheet(page);

    await expect(page.getByRole("heading", { name: "Wie der Gefahrenindex berechnet wird" })).toBeVisible();
    await expect(page.getByText(/Getötete × [\d,.]+, Schwerverletzte × [\d,.]+, Leichtverletzte × [\d,.]+/)).toBeVisible();
    await expect(page.getByText(/halbiert alle [\d,.]+ Jahre/)).toBeVisible();

    await expect(page.getByRole("heading", { name: "Was diese Zahlen nicht hergeben" })).toBeVisible();
    await expect(page.getByText(/keine\s+Risikoraten/)).toBeVisible();
  });

  test("nennt beide Lizenzen", async ({ page }) => {
    await openSheet(page);
    const sources = page.locator("article footer");

    await expect(sources).toContainText("dl-de/by-2-0");
    await expect(sources).toContainText("© OpenStreetMap-Mitwirkende");
  });

  // The seed holds accidents from 2018 to 2025. A sheet that says 2016–2026
  // claims years nobody has looked at.
  test("druckt nur Berichtsjahre, die die Daten abdecken", async ({ page }) => {
    await openSheet(page);
    await expect(page.getByText(/Berichtsjahre 2018–2025/)).toBeVisible();
  });

  test("dieselbe Adresse ergibt dieselben Zahlen", async ({ page }) => {
    await openSheet(page);
    const url = new URL(page.url());
    url.searchParams.set("radius", "250");

    await page.goto(url.pathname + url.search);
    await expect(page.getByText(/Umkreis 250 m/)).toBeVisible();
    await expect(row(page, "Unfälle insgesamt")).toHaveText("3");

    // The schoolway view from the institution page travels in the link too.
    url.searchParams.set("radius", "500");
    url.searchParams.set("modes", "foot,bike");
    await page.goto(url.pathname + url.search);
    await expect(row(page, "Unfälle insgesamt")).toHaveText("4");
  });

  test("zählt die Schwerpunkte, nicht den einzelnen Unfall", async ({ page }) => {
    await openSheet(page);
    const hotspots = page
      .getByRole("table")
      .filter({ has: page.getByRole("columnheader", { name: "Gefahrenindex" }) })
      .getByRole("row");

    // One header row plus the two hotspots; the lone accident 445 m out is none.
    await expect(hotspots).toHaveCount(3);
  });
});
