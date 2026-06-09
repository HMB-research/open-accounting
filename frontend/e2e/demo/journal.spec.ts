import { test, expect, type Locator } from "@playwright/test";
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from "./utils";

async function selectAccountByText(select: Locator, text: string) {
  const value = await select.evaluate((element, optionText) => {
    const selectElement = element as HTMLSelectElement;
    const option = Array.from(selectElement.options).find((candidate) =>
      candidate.textContent?.includes(optionText),
    );
    return option?.value || "";
  }, text);
  expect(value, `account option containing "${text}"`).not.toBe("");
  await select.selectOption(value);
}

test.describe("Demo Journal Entries - Page Structure Verification", () => {
  test.beforeEach(async ({ page }, testInfo) => {
    await ensureAuthenticated(page, testInfo);
    await ensureDemoTenant(page, testInfo);
    const accountsLoaded = page.waitForResponse(
      (response) =>
        response.url().includes("/accounts?active_only=true") &&
        response.status() === 200,
    );
    const entriesLoaded = page.waitForResponse(
      (response) =>
        response.url().includes("/journal-entries?limit=50") &&
        response.status() === 200,
    );
    await navigateTo(page, "/journal", testInfo);
    await Promise.all([accountsLoaded, entriesLoaded]);
    await expect(page.locator(".entry-card").first()).toBeVisible({
      timeout: 15000,
    });
  });

  test("displays journal entries page heading", async ({ page }) => {
    // Verify page loads with heading (level 1)
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible({
      timeout: 10000,
    });
  });

  test("shows new entry button or empty state", async ({ page }) => {
    // Page should have either entries or the create entry prompt
    const hasNewEntryButton = await page
      .getByRole("button")
      .first()
      .isVisible()
      .catch(() => false);
    const hasEmptyState = await page
      .getByText(/no.*entries|create|journal/i)
      .isVisible()
      .catch(() => false);

    expect(hasNewEntryButton || hasEmptyState).toBeTruthy();
  });

  test("page structure is correct", async ({ page }) => {
    // Journal page should have heading and action buttons
    const hasHeading = await page
      .getByRole("heading", { level: 1 })
      .isVisible()
      .catch(() => false);
    const hasButton = await page
      .getByRole("button")
      .first()
      .isVisible()
      .catch(() => false);

    expect(hasHeading && hasButton).toBeTruthy();
  });

  test("creates, posts, and voids a manual journal entry", async ({ page }) => {
    const description = `E2E journal lifecycle ${Date.now()}`;
    const reference = `E2E-${Date.now()}`;
    const voidReason = "E2E void after lifecycle verification";

    await page
      .getByRole("button", { name: /new entry|create journal entry/i })
      .first()
      .click();

    const dialog = page.getByRole("dialog", { name: /create journal entry/i });
    await expect(dialog).toBeVisible();
    await dialog.locator("#description").fill(description);
    await dialog.locator("#reference").fill(reference);

    const lineRows = dialog.locator("tbody tr");
    await expect(lineRows).toHaveCount(2);

    const debitRow = lineRows.nth(0);
    await selectAccountByText(debitRow.locator("select"), "1000 - Cash");
    await debitRow.locator('input[type="text"]').fill("Lifecycle debit");
    await debitRow.locator('input[type="number"]').first().fill("125.50");

    const creditRow = lineRows.nth(1);
    await selectAccountByText(
      creditRow.locator("select"),
      "4000 - Sales Revenue",
    );
    await creditRow.locator('input[type="text"]').fill("Lifecycle credit");
    await creditRow.locator('input[type="number"]').nth(1).fill("125.50");

    await expect(dialog.getByText(/balanced/i)).toBeVisible();

    await Promise.all([
      page.waitForResponse(
        (response) =>
          response.request().method() === "POST" &&
          response.url().includes("/journal-entries") &&
          response.status() === 201,
      ),
      dialog.getByRole("button", { name: /^create entry$/i }).click(),
    ]);

    const entryCard = page
      .locator(".entry-card")
      .filter({ hasText: description })
      .first();
    await expect(entryCard).toBeVisible();
    await expect(entryCard).toContainText(reference);
    await expect(entryCard).toContainText("DRAFT");
    await expect(entryCard).toContainText("Lifecycle debit");
    await expect(entryCard).toContainText("Lifecycle credit");

    await Promise.all([
      page.waitForResponse(
        (response) =>
          response.request().method() === "POST" &&
          response.url().includes("/post") &&
          response.status() === 200,
      ),
      entryCard.getByRole("button", { name: /^post$/i }).click(),
    ]);

    await expect(entryCard).toContainText("POSTED");

    page.once("dialog", (prompt) => prompt.accept(voidReason));
    await Promise.all([
      page.waitForResponse(
        (response) =>
          response.request().method() === "POST" &&
          response.url().includes("/void") &&
          response.status() === 200,
      ),
      entryCard.getByRole("button", { name: /^void$/i }).click(),
    ]);

    await expect(entryCard).toContainText("VOIDED");
    await expect(entryCard).toContainText(voidReason);
  });
});
