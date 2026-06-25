import { defineConfig, devices } from "@playwright/test";

/**
 * Playwright drives the revamped GUI for screenshot-based iteration and a small
 * set of behavioural smoke specs. Specs live in `e2e/` and are named
 * `*.spec.ts` so vitest (which collects `*.test.*`) never picks them up.
 *
 * Prerequisites for a real run (documented in DEMO.md): the Go API server on
 * :8080 (`MOKU_ALLOW_PRIVATE_HOSTS=1 go run .`) and the demo site on :9999
 * (`go run ./cmd/demoserver`). Playwright starts the Vite dev server itself.
 */
const PORT = 3000;
const BASE_URL = process.env.PLAYWRIGHT_BASE_URL ?? `http://localhost:${PORT}`;

export default defineConfig({
  testDir: "./e2e",
  testMatch: "**/*.spec.ts",
  outputDir: "./e2e/.artifacts",
  fullyParallel: false,
  workers: 1,
  // The workspace route lazy-loads a large Shiki-backed chunk that the dev
  // server compiles on first navigation, so allow a generous per-test budget.
  timeout: 240_000,
  expect: { timeout: 15_000 },
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  reporter: [["list"], ["html", { outputFolder: "e2e/.report", open: "never" }]],
  use: {
    baseURL: BASE_URL,
    headless: true,
    screenshot: "only-on-failure",
    trace: "on-first-retry",
    viewport: { width: 1440, height: 900 },
  },
  projects: [
    { name: "setup", testMatch: /global\.setup\.ts/ },
    { name: "chromium", use: { ...devices["Desktop Chrome"] }, dependencies: ["setup"] },
  ],
  webServer: {
    command: "npm run dev",
    url: BASE_URL,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
