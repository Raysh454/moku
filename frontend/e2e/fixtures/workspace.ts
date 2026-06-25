import { expect, type Page } from "@playwright/test";
import { SEED_PROJECT_NAME } from "./seed";

/** Opens the seeded project into the workspace and waits for the editor shell.
 * The first navigation compiles the lazy workspace chunk, so the wait is long. */
export async function openSeededWorkspace(page: Page): Promise<void> {
  await page.goto("/");
  await page.getByText(SEED_PROJECT_NAME, { exact: true }).first().click();
  await expect(page.getByText("Compare", { exact: true })).toBeVisible({ timeout: 180_000 });
}

/** Opens an endpoint by (sub)name from the explorer tree into its own tab. */
export async function openEndpoint(page: Page, name: RegExp): Promise<void> {
  await page.getByRole("treeitem", { name }).first().click();
  await expect(page.getByTestId("editor-tab").filter({ hasText: name })).toBeVisible();
}

/** True once the first diff host has painted content into its shadow root. */
export async function diffHasRendered(page: Page): Promise<boolean> {
  return page
    .locator(".moku-diff-host diffs-container")
    .first()
    .evaluate((element) => (element.shadowRoot?.textContent?.length ?? 0) > 50)
    .catch(() => false);
}
