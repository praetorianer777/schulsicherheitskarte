import { expect, test } from "@playwright/test";

// What the browser needs from the deployment, checked from the browser: the
// page and its data come from one origin, so no CORS rule exists anywhere and
// no host is hard-coded into the build.
test.describe("Deployment", () => {
  test("die Seite holt ihre Daten von derselben Herkunft", async ({ page, baseURL }) => {
    const origins = new Set<string>();
    page.on("request", (request) => {
      const url = new URL(request.url());
      if (url.pathname.startsWith("/api/")) origins.add(url.origin);
    });

    await page.goto("/");
    await page.getByLabel("Schule oder Kita suchen").fill("egidien");
    await page.getByRole("button", { name: "Suchen" }).click();
    await page.getByRole("link", { name: /Grundschule St. Egidien/ }).click();
    await expect(page.getByRole("heading", { level: 1 })).toHaveText("Grundschule St. Egidien");

    expect([...origins]).toEqual([new URL(baseURL!).origin]);
  });

  test("/healthz antwortet über den öffentlichen Einstieg", async ({ request }) => {
    const response = await request.get("/healthz");
    expect(response.ok()).toBe(true);
    expect(await response.json()).toEqual({ status: "ok" });
  });
});
