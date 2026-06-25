import { test, expect } from "@playwright/test";
import { openSeededWorkspace } from "./fixtures/workspace";

test("user_opens_a_seeded_project_into_the_workspace", async ({ page }) => {
  await openSeededWorkspace(page);

  await expect(page.getByText("Explorer", { exact: true })).toBeVisible();
  await expect(page.getByRole("treeitem").first()).toBeVisible();

  await page.screenshot({ path: "e2e/.artifacts/workspace-loaded.png" });
});
