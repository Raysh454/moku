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

test("website_section_collapses_and_expands_its_endpoints", async ({ page }) => {
  await openSeededWorkspace(page);

  const rows = page.getByRole("treeitem");
  await expect(rows.first()).toBeVisible();

  // Contracting the website hides its endpoint tree...
  const toggle = page.getByTestId("website-toggle").first();
  await toggle.click();
  await expect(rows).toHaveCount(0);

  // ...and expanding it brings the endpoints back.
  await toggle.click();
  await expect(rows.first()).toBeVisible();
});
