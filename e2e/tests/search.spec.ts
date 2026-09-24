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
    // The same figure the point on the map stands for, and the same one the
    // institution page opens with.
    await expect(result).toContainText("6 Unfälle im Umkreis von 500 m");
    await result.click();

    await expect(page.getByRole("heading", { level: 1 })).toHaveText("Grundschule St. Egidien");
  });

  test("ein Treffer lässt sich auf der Karte zeigen", async ({ page }) => {
    await page.goto("/");
    await page.getByLabel("Schule oder Kita suchen").fill("egidien");
    await page.getByRole("button", { name: "Suchen" }).click();

    const show = page.getByRole("button", { name: "Auf der Karte zeigen" }).first();
    await show.focus();
    await expect(show).toBeFocused();
    await page.keyboard.press("Enter");
    // The map is a canvas; what can be checked is that the page stayed intact
    // and the link beside the button still leads where it says.
    await expect(page.getByRole("link", { name: /Grundschule St. Egidien/ })).toBeVisible();
  });

  test("erklärt einen leeren Treffer statt stumm zu bleiben", async ({ page }) => {
    await page.goto("/");
    await page.getByLabel("Schule oder Kita suchen").fill("gibtesnichtxyz");
    await page.getByRole("button", { name: "Suchen" }).click();

    await expect(page.getByText(/wurde nichts gefunden/)).toBeVisible();
  });

  // The Kita has neither the town in its name nor an address in
  // OpenStreetMap; only the municipal boundary it lies in says where it is.
  test("findet eine Kita über ihren Ort, auch ohne Adresse", async ({ page }) => {
    await page.goto("/");
    await page.getByLabel("Schule oder Kita suchen").fill("egidien");
    await page.getByRole("button", { name: "Suchen" }).click();

    const kita = page.getByRole("link", { name: /Kita Sonnenschein/ });
    await expect(kita).toBeVisible();
    await expect(kita).toContainText("St. Egidien");

    await kita.click();
    await expect(page.getByRole("heading", { level: 1 })).toHaveText("Kita Sonnenschein");
    await expect(page.getByText("Kita · St. Egidien")).toBeVisible();
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
