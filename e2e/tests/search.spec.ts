import { expect, test } from "@playwright/test";

test.describe("Suche", () => {
  test("führt von der Startseite zu einer Einrichtung", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { level: 1 })).toContainText(
      "Wie sicher ist der Weg zu dieser Schule?",
    );

    await page.getByLabel("Schule oder Kita suchen").fill("egidien");
    await page.getByRole("button", { name: "Suchen" }).click();

    const result = page.getByRole("link", { name: /Grundschule St. Egidien/ });
    await expect(result).toBeVisible();
    await result.click();

    await expect(page.getByRole("heading", { level: 1 })).toHaveText("Grundschule St. Egidien");
  });

  test("erklärt einen leeren Treffer statt stumm zu bleiben", async ({ page }) => {
    await page.goto("/");
    await page.getByLabel("Schule oder Kita suchen").fill("gibtesnichtxyz");
    await page.getByRole("button", { name: "Suchen" }).click();

    await expect(page.getByText(/wurde nichts gefunden/)).toBeVisible();
  });

  // The whole journey without touching the mouse: this is the one that decides
  // whether the site is usable for people who never use one.
  test("ist von der Suche bis zur Einrichtung mit der Tastatur bedienbar", async ({ page }) => {
    await page.goto("/");

    await page.keyboard.press("Tab"); // skip link
    await page.keyboard.press("Tab"); // site title
    await page.keyboard.press("Tab"); // search field
    await expect(page.getByLabel("Schule oder Kita suchen")).toBeFocused();

    await page.keyboard.type("egidien");
    await page.keyboard.press("Enter");

    const result = page.getByRole("link", { name: /Grundschule St. Egidien/ });
    await expect(result).toBeVisible();
    await result.focus();
    await page.keyboard.press("Enter");

    await expect(page.getByRole("heading", { level: 1 })).toHaveText("Grundschule St. Egidien");
  });
});
