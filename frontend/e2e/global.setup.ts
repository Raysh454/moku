import { test as setup, expect } from "@playwright/test";
import { seedDemoProject, SEED_PROJECT_NAME } from "./fixtures/seed";

// Runs once before the smoke specs (as a Playwright project dependency).
setup("seed demo data", async ({ request, page }) => {
  setup.setTimeout(360_000);
  await seedDemoProject(request);

  // Warm the lazy, Shiki-heavy workspace chunk so the first spec does not pay
  // the dev server's cold-compile cost. Best-effort: never fail the suite here.
  const errors: string[] = [];
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(message.text());
  });
  page.on("pageerror", (error) => errors.push(`pageerror: ${error.message}`));
  try {
    await page.goto("/");
    await page.getByText(SEED_PROJECT_NAME, { exact: true }).first().click();
    await expect(page.getByText("Compare", { exact: true })).toBeVisible({ timeout: 300_000 });
    console.log("WARMUP_OK", page.url());
  } catch (error) {
    console.log("WARMUP_FAIL url=", page.url(), "errors=", JSON.stringify(errors.slice(0, 8)), String(error).slice(0, 200));
  }
});
