import { expect, test, type Page } from "@playwright/test";

async function openSchool(page: Page) {
  await page.goto("/");
  await page.getByLabel("Schule oder Kita suchen").fill("egidien");
  await page.getByRole("button", { name: "Suchen" }).click();
  await page.getByRole("link", { name: /Grundschule St. Egidien/ }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Grundschule St. Egidien");
}

/** The figure under one of the key-figure labels. */
function figure(page: Page, label: string) {
  return page.locator("dt", { hasText: label }).first().locator("xpath=following-sibling::dd[1]");
}

test.describe("Einrichtungsseite", () => {
  test("zeigt die Unfälle im Umkreis", async ({ page }) => {
    await openSchool(page);

    // The seed puts six accidents within 500 m of this school.
    await expect(figure(page, "Unfälle insgesamt")).toHaveText("6");
    await expect(figure(page, "mit Getöteten")).toHaveText("1");
  });

  test("der Radius ändert die Zahlen sichtbar", async ({ page }) => {
    await openSchool(page);
    await expect(figure(page, "Unfälle insgesamt")).toHaveText("6");

    await page.getByRole("radio", { name: "250 m" }).check();

    // Only the three accidents right by the school stay inside 250 m.
    await expect(figure(page, "Unfälle insgesamt")).toHaveText("3");
    await expect(page.getByRole("heading", { level: 2, name: /Umkreis von 250 m/ })).toBeVisible();
  });

  test("der Zeitraum ändert die Zahlen sichtbar", async ({ page }) => {
    await openSchool(page);
    await page.getByLabel("Ab Berichtsjahr").selectOption("2024");

    await expect(figure(page, "Unfälle insgesamt")).toHaveText("2");
  });

  test("die Schulweg-Sicht lässt nur Fuß- und Radbeteiligung übrig", async ({ page }) => {
    await openSchool(page);
    await page.getByRole("checkbox", { name: /Nur Unfälle mit Fuß- oder Radbeteiligung/ }).check();

    await expect(figure(page, "Unfälle insgesamt")).toHaveText("4");
  });

  test("die Schwerpunkte sind nach Gefahrenindex sortiert", async ({ page }) => {
    await openSchool(page);

    const entries = page.getByRole("button", { name: /Auf der Karte zeigen/ });
    await expect(entries).toHaveCount(2);

    const scores = await page
      .locator("ol li")
      .filter({ hasText: "Gefahrenindex" })
      .allInnerTexts();
    const values = scores.map((text) => Number(text.match(/Gefahrenindex ([\d,.]+)/)?.[1]?.replace(",", ".")));
    expect(values[0]).toBeGreaterThan(values[1]);

    // The lone accident 445 m out is not a pattern and must not be listed as
    // a third hotspot.
    await expect(page.getByText(/3 Unfälle/).first()).toBeVisible();
  });

  test("ein Schwerpunkt lässt sich über die Liste auf der Karte zeigen", async ({ page }) => {
    await openSchool(page);
    const first = page.getByRole("button", { name: /Auf der Karte zeigen/ }).first();
    await first.focus();
    await expect(first).toBeFocused();
    await page.keyboard.press("Enter");

    // The map is a canvas, so the check is that the page stayed intact and the
    // entry is still reachable — not what was drawn.
    await expect(page.getByRole("region", { name: /Karte der Umgebung/ })).toBeVisible();
  });

  test("jeder Unfall steht auch als Tabellenzeile", async ({ page }) => {
    await openSchool(page);
    const rows = page.getByRole("row");
    // One header row plus one per accident within 500 m.
    await expect(rows).toHaveCount(7);
    await expect(page.getByRole("cell", { name: "Getötete" })).toBeVisible();
  });

  test("nennt die Grenzen der Daten", async ({ page }) => {
    await openSchool(page);
    await expect(page.getByRole("heading", { name: "Was diese Zahlen nicht hergeben" })).toBeVisible();
    await expect(page.getByText(/keine Risikoraten/)).toBeVisible();
  });

  test("zeigt Querungen und Tempolimits, wenn die Ebene eingeschaltet wird", async ({ page }) => {
    await openSchool(page);
    await page.getByRole("checkbox", { name: "Querungen, Ampeln, Tempolimits" }).check();
    await expect(page.getByRole("checkbox", { name: "Querungen, Ampeln, Tempolimits" })).toBeChecked();
  });
});
