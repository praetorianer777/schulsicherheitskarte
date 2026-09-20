import { expect, test, type Page } from "@playwright/test";

async function openSchool(page: Page) {
  await page.goto("/");
  await page.getByLabel("Schule oder Kita suchen").fill("egidien");
  await page.getByRole("button", { name: "Suchen" }).click();
  await page.getByRole("link", { name: /Grundschule St. Egidien/ }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Grundschule St. Egidien");
}

test.describe("Karte im gebauten Image", () => {
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
