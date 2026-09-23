import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

import { openSchool } from "./support";

/**
 * axe catches regressions — a missing label, a heading level skipped, a control
 * nobody can reach. It does not replace testing with a keyboard and a screen
 * reader, and nothing here should be read as saying it does.
 */
async function violationsOn(page: import("@playwright/test").Page) {
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
    // The map is a WebGL canvas with an accessible description; axe cannot see
    // inside it, and the list beside it is what carries the content.
    .exclude(".maplibregl-canvas-container")
    .analyze();
  return results.violations.map((v) => `${v.id}: ${v.help} (${v.nodes.length}×)`);
}

test("die Startseite hat keine Verstöße", async ({ page }) => {
  await page.goto("/");
  expect(await violationsOn(page)).toEqual([]);
});

test("die Suchergebnisse haben keine Verstöße", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("Schule oder Kita suchen").fill("egidien");
  await page.getByRole("button", { name: "Suchen" }).click();
  await expect(page.getByRole("link", { name: /Grundschule St. Egidien/ })).toBeVisible();

  expect(await violationsOn(page)).toEqual([]);
});

test("die Einrichtungsseite hat keine Verstöße", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("Schule oder Kita suchen").fill("egidien");
  await page.getByRole("button", { name: "Suchen" }).click();
  await page.getByRole("link", { name: /Grundschule St. Egidien/ }).click();
  await expect(page.getByRole("heading", { name: "Unfallschwerpunkte" })).toBeVisible();

  expect(await violationsOn(page)).toEqual([]);
});

test("das offene Meldeformular hat keine Verstöße", async ({ page }) => {
  await openSchool(page);
  await page.getByRole("button", { name: /Gefahrenstelle melden/ }).click();
  await expect(page.getByRole("button", { name: "Meldung abschicken" })).toBeVisible();

  expect(await violationsOn(page)).toEqual([]);
});

test("das Faktenblatt hat keine Verstöße", async ({ page }) => {
  await openSchool(page);
  await page.getByRole("link", { name: "Faktenblatt zum Ausdrucken" }).click();
  await expect(page.getByRole("heading", { name: "Wie der Gefahrenindex berechnet wird" })).toBeVisible();

  expect(await violationsOn(page)).toEqual([]);
});

test("die Moderation hat keine Verstöße", async ({ page }) => {
  await page.goto("/moderation");
  expect(await violationsOn(page)).toEqual([]);

  await page.getByLabel("Moderations-Token").fill("e2e-moderation-token");
  await page.getByRole("button", { name: "Warteschlange öffnen" }).click();
  await expect(page.getByText(/Warteschlange ist leer|Freigeben/).first()).toBeVisible();
  expect(await violationsOn(page)).toEqual([]);
});

// Every page has to be reachable past the header without tabbing through it.
test("der Sprunglink führt zum Inhalt", async ({ page }) => {
  await page.goto("/");
  await page.keyboard.press("Tab");

  const skip = page.getByRole("link", { name: "Zum Inhalt springen" });
  await expect(skip).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.locator("#inhalt")).toBeVisible();
});
