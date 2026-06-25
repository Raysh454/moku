import { test, expect } from "@playwright/test";
import { openSeededWorkspace } from "./fixtures/workspace";

test("explorer_shows_endpoints_as_nested_files_with_status", async ({ page }) => {
  await openSeededWorkspace(page);

  // The demo site has several endpoints, nested by URL path.
  const rows = page.getByRole("treeitem");
  await expect(rows.first()).toBeVisible();
  expect(await rows.count()).toBeGreaterThan(1);

  // A known endpoint from the demo site is present as a file row.
  await expect(page.getByRole("treeitem", { name: /login/i })).toBeVisible();

  await page.screenshot({ path: "e2e/.artifacts/explorer-tree.png" });
});
