import { test, expect } from "@playwright/test";
import { openSeededWorkspace } from "./fixtures/workspace";

test("opening_multiple_endpoints_keeps_separate_tabs", async ({ page }) => {
  await openSeededWorkspace(page);

  // Open two distinct, unambiguous endpoints.
  await page.getByRole("treeitem", { name: /login/i }).first().click();
  await page.getByRole("treeitem", { name: /contact/i }).first().click();

  expect(await page.getByTestId("editor-tab").count()).toBeGreaterThanOrEqual(2);
  // The last-opened endpoint keeps its own tab.
  await expect(page.getByTestId("editor-tab").filter({ hasText: "/contact" })).toBeVisible();

  await page.screenshot({ path: "e2e/.artifacts/multi-tab.png" });
});
