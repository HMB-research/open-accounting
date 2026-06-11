import { test, expect, type Page, type TestInfo } from "@playwright/test";
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from "./utils";

interface AccountResponse {
  id: string;
  code: string;
  name: string;
  account_type: "ASSET" | "LIABILITY" | "EQUITY" | "REVENUE" | "EXPENSE";
  description?: string;
  is_active: boolean;
  is_system: boolean;
}

async function openAccounts(page: Page, testInfo: TestInfo) {
  await navigateTo(page, "/accounts", testInfo);
  await expect(page.locator("table tbody tr").first()).toBeVisible({
    timeout: 10000,
  });
}

function accountRow(page: Page, text: string) {
  return page.locator("table tbody tr").filter({ hasText: text });
}

async function createAccount(
  page: Page,
  overrides: Partial<AccountResponse> = {},
): Promise<AccountResponse> {
  const unique = Date.now().toString(36).slice(-5).toUpperCase();
  const code = overrides.code ?? `89${unique.slice(-3)}`;
  const name = overrides.name ?? `Workflow Account ${unique}`;

  await page.getByRole("button", { name: /new account|uus konto/i }).click();
  const modal = page.locator('[role="dialog"]').last();
  await expect(modal).toBeVisible({ timeout: 5000 });
  await modal.locator("#code").fill(code);
  await modal.locator("#name").fill(name);
  await modal
    .locator("#type")
    .selectOption(overrides.account_type ?? "EXPENSE");
  await modal
    .locator("#description")
    .fill(overrides.description ?? "Created by accounts workflow E2E");

  const createResponsePromise = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return (
      response.request().method() === "POST" &&
      /\/api\/v1\/tenants\/[^/]+\/accounts$/.test(url.pathname)
    );
  });
  await modal.getByRole("button", { name: /create|loo/i }).click();
  const response = await createResponsePromise;
  expect(response.ok()).toBeTruthy();
  const account = (await response.json()) as AccountResponse;
  await expect(accountRow(page, account.code)).toContainText(account.name);
  return account;
}

test.describe("Demo Chart of Accounts", () => {
  test.beforeEach(async ({ page }, testInfo) => {
    await ensureAuthenticated(page, testInfo);
    await ensureDemoTenant(page, testInfo);
    await openAccounts(page, testInfo);
  });

  test("displays seeded accounts and manages a custom account", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: /chart of accounts|kontoplaan/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /new account|uus konto/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /import accounts|impordi kontod/i }),
    ).toBeVisible();
    await expect(page.getByRole("cell", { name: "Cash" })).toBeVisible();
    await expect(
      page.getByRole("cell", { name: /Bank Account.*EUR/i }),
    ).toBeVisible();
    await expect(page.locator("table tbody tr")).toHaveCount(33, {
      timeout: 10000,
    });
    await expect(
      page.getByRole("cell", { name: /1[0-9]{3}/ }).first(),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: /assets|varad/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: /liabilities|kohustused/i }),
    ).toBeVisible();

    const unique = Date.now().toString(36).slice(-5).toUpperCase();
    const account = await createAccount(page, {
      code: `88${unique.slice(-3)}`,
      name: `Workflow Expense ${unique}`,
      account_type: "EXPENSE",
      description: "Initial workflow description",
    });

    let row = accountRow(page, account.code);
    await expect(row).toContainText(account.name);
    await row.getByRole("button", { name: /edit|muuda/i }).click();

    const updatedCode = `87${unique.slice(-3)}`;
    const updatedName = `Workflow Edited ${unique}`;
    const modal = page.locator('[role="dialog"]').last();
    await expect(
      modal.getByRole("heading", { name: /edit account|muuda kontot/i }),
    ).toBeVisible();
    await modal.locator("#code").fill(updatedCode);
    await modal.locator("#name").fill(updatedName);
    await modal.locator("#description").fill("Updated workflow description");

    const updateResponsePromise = page.waitForResponse(
      (response) =>
        response.request().method() === "PUT" &&
        response.url().includes(`/accounts/${account.id}`),
    );
    await modal.getByRole("button", { name: /save|salvesta/i }).click();
    const updateResponse = await updateResponsePromise;
    expect(updateResponse.ok()).toBeTruthy();
    const updated = (await updateResponse.json()) as AccountResponse;
    expect(updated.code).toBe(updatedCode);
    expect(updated.name).toBe(updatedName);
    expect(updated.description).toBe("Updated workflow description");

    row = accountRow(page, updatedCode);
    await expect(row).toContainText(updatedName);

    page.once("dialog", (dialog) => dialog.accept());
    const deleteResponsePromise = page.waitForResponse(
      (response) =>
        response.request().method() === "DELETE" &&
        response.url().includes(`/accounts/${account.id}`),
    );
    await row.getByRole("button", { name: /delete|kustuta/i }).click();
    const deleteResponse = await deleteResponsePromise;
    expect(deleteResponse.ok()).toBeTruthy();
    const deactivated = (await deleteResponse.json()) as AccountResponse;
    expect(deactivated.is_active).toBe(false);

    row = accountRow(page, updatedCode);
    await expect(row).toContainText(/inactive|mitteaktiivne/i);
    await expect(
      row.getByRole("button", { name: /delete|kustuta/i }),
    ).toHaveCount(0);
  });
});
