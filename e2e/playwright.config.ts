import { defineConfig, devices } from "@playwright/test";

const port = process.env.WEB_PORT ?? "8090";

export default defineConfig({
  testDir: "./tests",
  // The stack is shared, and the reporting journey changes what the others see,
  // so the specs run one after another.
  workers: 1,
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : [["list"]],
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    locale: "de-DE",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
