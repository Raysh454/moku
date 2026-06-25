import { test, expect } from "@playwright/test";
import { diffHasRendered, openEndpoint, openSeededWorkspace } from "./fixtures/workspace";

test("diff_view_renders_word_level_changes_between_versions", async ({ page }) => {
  await openSeededWorkspace(page);
  await openEndpoint(page, /login/i); // a changed HTML endpoint

  await page.getByRole("tab", { name: "Diff" }).click();
  await expect.poll(() => diffHasRendered(page), { timeout: 30_000 }).toBe(true);
  await page.screenshot({ path: "e2e/.artifacts/body-diff-split.png" });

  // Switching the layout keeps the diff rendered.
  await page.getByRole("tab", { name: "Unified" }).click();
  await expect.poll(() => diffHasRendered(page), { timeout: 15_000 }).toBe(true);
  await page.screenshot({ path: "e2e/.artifacts/body-diff-unified.png" });
});
