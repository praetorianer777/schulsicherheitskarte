import { expect, test, type Page } from "@playwright/test";

import { colours, openSchool, pixelsOf, surroundingsMap, withoutBasemap } from "./support";

// run.sh starts the stack with this token.
const moderationToken = "e2e-moderation-token";

/**
 * A description nobody else writes, so the report can be told apart from the
 * ones a retried run left behind in the same stack.
 */
function uniqueDescription(label: string) {
  return `${label} ${Date.now().toString(36)}`;
}

async function submitReport(page: Page, description: string) {
  await openSchool(page);
  await page.getByRole("button", { name: /Gefahrenstelle melden/ }).click();

  // Away from the accidents, which all lie due north of the school.
  await page.getByRole("button", { name: "nach Osten" }).click();
  await page.getByRole("button", { name: "nach Osten" }).click();
  await expect(page.getByText("Gewählt: 50 m östlich der Einrichtung")).toBeVisible();

  await page.getByLabel("Worum geht es?").selectOption({ label: "Autos fahren zu schnell" });
  await page.getByLabel(/Beschreibung/).fill(description);
  await page.getByRole("button", { name: "Meldung abschicken" }).click();

  await expect(page.getByRole("status")).toContainText("Danke, die Meldung ist eingegangen.");
}

async function openQueue(page: Page) {
  await page.goto("/moderation");
  await page.getByLabel("Moderations-Token").fill(moderationToken);
  await page.getByRole("button", { name: "Warteschlange öffnen" }).click();
}

function queueEntry(page: Page, description: string) {
  return page.getByRole("listitem").filter({ hasText: description });
}

/**
 * Opens the school and waits for its reports to arrive, so that "not in the
 * list" is an answer from the server and not a list that has not loaded yet.
 */
async function openSchoolWithReports(page: Page) {
  const loaded = page.waitForResponse(
    (response) => /\/api\/institutions\/\d+\/reports/.test(response.url()) && response.ok(),
  );
  await openSchool(page);
  await loaded;
}

function publicEntry(page: Page, description: string) {
  return page
    .getByRole("region", { name: "Gemeldete Gefahrenstellen" })
    .getByRole("listitem")
    .filter({ hasText: description });
}

/** Waits until the map has drawn at all, so "no report pixels" means something. */
async function mapDrawn(page: Page) {
  await expect
    .poll(() => pixelsOf(surroundingsMap(page), colours.accident), { timeout: 15_000 })
    .toBeGreaterThan(50);
}

function reportPixels(page: Page) {
  return pixelsOf(surroundingsMap(page), colours.report);
}

test.describe("Gefahrenstelle melden", () => {
  test("eine Meldung erscheint erst nach der Freigabe, Bestätigungen zählen einmal", async ({ page }) => {
    const description = uniqueDescription("Rasende Autos vor dem Tor");
    await withoutBasemap(page);

    await test.step("melden", () => submitReport(page, description));

    await test.step("noch nicht auf der Karte", async () => {
      await openSchoolWithReports(page);
      await mapDrawn(page);
      await expect(publicEntry(page, description)).toHaveCount(0);
      expect(await reportPixels(page)).toBe(0);
    });

    await test.step("in der Moderation freigeben", async () => {
      await openQueue(page);
      const entry = queueEntry(page, description);
      await expect(entry).toBeVisible();
      await expect(entry).toContainText("Autos fahren zu schnell");
      await entry.getByRole("button", { name: "Freigeben" }).click();
      await expect(entry).toHaveCount(0);
    });

    await test.step("jetzt auf der Karte und in der Liste", async () => {
      await openSchool(page);
      await expect(publicEntry(page, description)).toBeVisible();
      await expect.poll(() => reportPixels(page), { timeout: 15_000 }).toBeGreaterThan(20);
    });

    await test.step("bestätigen", async () => {
      const entry = publicEntry(page, description);
      await expect(entry).toContainText("0 Personen bestätigen das auch");
      await entry.getByRole("button", { name: "Betrifft mich auch" }).click();
      await expect(entry.getByRole("button", { name: "Danke, notiert" })).toBeDisabled();
      await expect(entry).toContainText("1 Person bestätigt das auch");
    });

    // The button is disabled only until the page is reloaded; the server is
    // what keeps one person from counting twice.
    await test.step("ein zweites Mal bestätigen zählt nicht doppelt", async () => {
      await openSchool(page);
      const entry = publicEntry(page, description);
      await expect(entry).toContainText("1 Person bestätigt das auch");
      await entry.getByRole("button", { name: "Betrifft mich auch" }).click();
      await expect(entry.getByRole("button", { name: "Danke, notiert" })).toBeDisabled();
      await expect(entry).toContainText("1 Person bestätigt das auch");
    });
  });

  test("eine abgelehnte Meldung erscheint nie", async ({ page }) => {
    const description = uniqueDescription("Beschwerde über den Nachbarn");
    await submitReport(page, description);

    await openQueue(page);
    const entry = queueEntry(page, description);
    await expect(entry).toBeVisible();
    await entry.getByLabel(/Notiz/).fill("keine Gefahrenstelle");
    await entry.getByRole("button", { name: "Ablehnen" }).click();
    await expect(entry).toHaveCount(0);

    await openSchoolWithReports(page);
    await expect(publicEntry(page, description)).toHaveCount(0);
  });

  test("ein falsches Token öffnet die Warteschlange nicht", async ({ page }) => {
    await page.goto("/moderation");
    await page.getByLabel("Moderations-Token").fill("geraten");
    await page.getByRole("button", { name: "Warteschlange öffnen" }).click();

    await expect(page.getByText("Kein gültiges Moderations-Token.")).toBeVisible();
    await expect(page.getByRole("button", { name: "Freigeben" })).toHaveCount(0);
  });
});
