import { test, expect } from "@playwright/test";
import { openEndpoint, openSeededWorkspace } from "./fixtures/workspace";

test("analysis_tab_surfaces_security_score_and_findings", async ({ page }) => {
  await openSeededWorkspace(page);
  await openEndpoint(page, /login/i);

  await page.getByRole("tab", { name: "Analysis" }).click();

  // Summary strip + scoring sections render from the security diff.
  // (exact match: "Posture" must not collide with the "POSTURE SCORE" metric.)
  await expect(page.getByText("Posture", { exact: true })).toBeVisible();
  await expect(page.getByText("Security scoring", { exact: true })).toBeVisible();
  await expect(page.getByText("Attack surface elements", { exact: true })).toBeVisible();

  await page.screenshot({ path: "e2e/.artifacts/analysis-overview.png" });
});
