import { test, expect, Page } from "@playwright/test";
import {
  ensureAuthenticated,
  ensureDemoTenant,
  navigateTo,
} from "../demo/utils";

async function expectVisible(page: Page, selectors: string[]) {
  for (const selector of selectors) {
    const locator = page.locator(selector).first();
    if (await locator.isVisible().catch(() => false)) {
      return;
    }
  }

  throw new Error(
    `Expected one of these selectors to be visible: ${selectors.join(", ")}`,
  );
}

test.describe("Smoke - Core Accountant Flow", () => {
  test.beforeEach(async ({ page }, testInfo) => {
    await ensureAuthenticated(page, testInfo);
    await ensureDemoTenant(page, testInfo);
  });

  test("dashboard loads for an authenticated tenant", async ({
    page,
  }, testInfo) => {
    await navigateTo(page, "/dashboard", testInfo);

    await expect(page).toHaveURL(/dashboard/);
    await expectVisible(page, [
      "main h1",
      ".dashboard-header",
      '[data-testid="dashboard"]',
    ]);
  });

  test("accounts page exposes import and create actions", async ({
    page,
  }, testInfo) => {
    await navigateTo(page, "/accounts", testInfo);

    await expect(page.getByRole("heading", { level: 1 })).toBeVisible({
      timeout: 10000,
    });
    await expect(
      page
        .getByRole("button", { name: /import accounts|impordi kontod/i })
        .first(),
    ).toBeVisible({ timeout: 10000 });
    await expect(
      page.getByRole("button", { name: /new account|uus konto|\+/i }).first(),
    ).toBeVisible({ timeout: 10000 });
    await expectVisible(page, ["table", ".empty-state"]);
  });

  test("journal page exposes opening-balance import and create actions", async ({
    page,
  }, testInfo) => {
    await navigateTo(page, "/journal", testInfo);

    await expect(page.getByRole("heading", { level: 1 })).toBeVisible({
      timeout: 10000,
    });
    await expect(
      page
        .getByRole("button", {
          name: /import opening balances|impordi algsaldod/i,
        })
        .first(),
    ).toBeVisible({ timeout: 10000 });
    await expect(
      page
        .getByRole("button", { name: /new entry|uus kanne|\+ new entry/i })
        .first(),
    ).toBeVisible({ timeout: 10000 });
    await expectVisible(page, [".entries-list", ".empty-state"]);
  });

  test("invoices page exposes the primary create action", async ({
    page,
  }, testInfo) => {
    await navigateTo(page, "/invoices", testInfo);

    await expect(page.getByRole("heading", { level: 1 })).toBeVisible({
      timeout: 10000,
    });
    await expect(
      page.getByRole("button", { name: /new invoice|uus arve|\+/i }).first(),
    ).toBeVisible({ timeout: 10000 });
    await expectVisible(page, ["table", ".empty-state", ".invoice-list"]);
  });

  test("reports page renders report controls", async ({ page }, testInfo) => {
    await navigateTo(page, "/reports", testInfo);

    await expect(page.getByRole("heading", { level: 1 })).toBeVisible({
      timeout: 10000,
    });
    await expectVisible(page, ["select", 'input[type="date"]', "button"]);
  });

  test("banking page renders account and transaction views", async ({
    page,
  }, testInfo) => {
    await navigateTo(page, "/banking", testInfo);

    await expect(page.getByRole("heading", { level: 1 })).toBeVisible({
      timeout: 10000,
    });
    await expectVisible(page, [
      "table tbody tr",
      ".bank-account-card",
      ".empty-state",
    ]);
  });

  test("payroll page renders payroll workflow controls", async ({
    page,
  }, testInfo) => {
    await navigateTo(page, "/payroll", testInfo);

    await expect(page.getByRole("heading", { level: 1 })).toBeVisible({
      timeout: 10000,
    });
    await expectVisible(page, [".header .btn-primary", "button.btn-primary"]);
    await expectVisible(page, ["main select", '[role="combobox"]']);
  });
});
