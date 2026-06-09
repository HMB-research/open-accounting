import { test, expect, type Page } from "@playwright/test";
import {
  ensureAuthenticated,
  navigateTo,
  ensureDemoTenant,
  waitForPageReady,
} from "./utils";

async function waitForAssetsReady(page: Page) {
  await waitForPageReady(page);
  await page
    .getByText(/^Loading\.\.\.$/)
    .waitFor({ state: "hidden", timeout: 10000 })
    .catch(() => {});
  await expect(
    page.getByRole("heading", { name: /fixed assets|assets/i }),
  ).toBeVisible();
  await expect(async () => {
    const hasTable = await page
      .locator("table")
      .isVisible()
      .catch(() => false);
    const hasEmptyState = await page
      .locator(".empty-state")
      .isVisible()
      .catch(() => false);
    expect(hasTable || hasEmptyState).toBeTruthy();
  }).toPass({ timeout: 10000 });
}

async function assetRowCount(page: Page): Promise<number> {
  return page
    .locator("table tbody tr")
    .count()
    .catch(() => 0);
}

test.describe("Fixed Assets View", () => {
  test.beforeEach(async ({ page }, testInfo) => {
    await ensureAuthenticated(page, testInfo);
    await ensureDemoTenant(page, testInfo);
  });

  test("displays fixed assets page with correct structure", async ({
    page,
  }, testInfo) => {
    await navigateTo(page, "/assets", testInfo);
    await waitForAssetsReady(page);

    await expect(page.locator(".filters select").first()).toBeVisible();
    await expect(
      page.getByRole("button", { name: /new asset|new|create|add/i }),
    ).toBeVisible();
    if ((await assetRowCount(page)) > 0) {
      await expect(page.getByText(/FA-\d{4}-\d{3}/i).first()).toBeVisible();
    }
  });

  test("displays asset statuses in table when data exists", async ({
    page,
  }, testInfo) => {
    await navigateTo(page, "/assets", testInfo);
    await waitForAssetsReady(page);

    const table = page.locator("table");
    if ((await assetRowCount(page)) > 0) {
      const statusTexts = ["active", "draft", "disposed", "sold", "scrapped"];
      let foundStatus = false;
      for (const status of statusTexts) {
        const hasStatus = await table
          .getByText(new RegExp(status, "i"))
          .first()
          .isVisible()
          .catch(() => false);
        if (hasStatus) {
          foundStatus = true;
          break;
        }
      }
      expect(foundStatus).toBe(true);
    }
  });

  test("shows asset categories when data exists", async ({
    page,
  }, testInfo) => {
    await navigateTo(page, "/assets", testInfo);
    await waitForAssetsReady(page);

    if ((await assetRowCount(page)) > 0) {
      await expect(
        page.locator("table thead").getByText(/category/i),
      ).toBeVisible();
      await expect(
        page.locator("table tbody tr").first().locator("td").nth(3),
      ).toBeVisible();
    }
  });

  test("displays asset details when data exists", async ({
    page,
  }, testInfo) => {
    await navigateTo(page, "/assets", testInfo);
    await waitForAssetsReady(page);

    if ((await assetRowCount(page)) > 0) {
      const firstRow = page.locator("table tbody tr").first();
      await expect(firstRow).toBeVisible();
      await expect(firstRow.locator("td").nth(0)).toBeVisible();
      await expect(firstRow.locator("td").nth(1)).toBeVisible();
    }
  });

  test("can filter assets by status", async ({ page }, testInfo) => {
    await navigateTo(page, "/assets", testInfo);
    await waitForAssetsReady(page);

    const statusFilter = page.locator(".filters select").first();
    await expect(statusFilter).toBeVisible();
    await statusFilter.selectOption({ index: 1 });
    await expect(statusFilter).not.toHaveValue("");
    await expect(page.locator("table, .empty-state").first()).toBeVisible();
  });

  test("has New Asset button", async ({ page }, testInfo) => {
    await navigateTo(page, "/assets", testInfo);

    // Verify New button exists
    const newButton = page
      .getByRole("button", { name: /new|create|add/i })
      .or(page.getByRole("link", { name: /new|create|add/i }));
    await expect(newButton).toBeVisible();
  });
});
