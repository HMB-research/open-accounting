import { test, expect, type Page } from "@playwright/test";
import {
  ensureAuthenticated,
  navigateTo,
  ensureDemoTenant,
  waitForPageReady,
} from "./utils";

async function waitForEmailSettingsReady(page: Page) {
  await waitForPageReady(page);
  await page
    .getByText(/^Loading\.\.\.$/)
    .waitFor({ state: "hidden", timeout: 10000 })
    .catch(() => {});
  await expect(
    page.getByRole("heading", { name: /email|smtp|mail/i }).first(),
  ).toBeVisible();
  await expect(page.locator("#smtp_host")).toBeVisible();
}

test.describe("Email Settings View", () => {
  test.beforeEach(async ({ page }, testInfo) => {
    await ensureAuthenticated(page, testInfo);
    await ensureDemoTenant(page, testInfo);
  });

  test("displays email settings page with correct structure", async ({
    page,
  }, testInfo) => {
    await navigateTo(page, "/settings/email", testInfo);
    await waitForEmailSettingsReady(page);

    await expect(page.getByRole("button", { name: /smtp/i })).toBeVisible();
    await expect(page.locator("form")).toBeVisible();
  });

  test("has SMTP configuration form", async ({ page }, testInfo) => {
    await navigateTo(page, "/settings/email", testInfo);
    await waitForEmailSettingsReady(page);

    await expect(page.locator("#smtp_host")).toBeVisible();
    await expect(page.locator("#smtp_port")).toBeVisible();
    await expect(page.locator("#smtp_username")).toBeVisible();
    await expect(page.locator("#smtp_from_email")).toBeVisible();
  });

  test("has tabs for SMTP, Templates, and Log", async ({ page }, testInfo) => {
    await navigateTo(page, "/settings/email", testInfo);
    await waitForEmailSettingsReady(page);

    const tabs = page.locator(".tabs");
    await expect(tabs.getByRole("button", { name: /smtp/i })).toBeVisible();
    await expect(tabs.getByRole("button", { name: /template/i })).toBeVisible();
    await expect(tabs.getByRole("button", { name: /log/i })).toBeVisible();
  });

  test("has save button", async ({ page }, testInfo) => {
    await navigateTo(page, "/settings/email", testInfo);
    await waitForEmailSettingsReady(page);

    await expect(
      page.getByRole("button", { name: /save|update|apply/i }).first(),
    ).toBeVisible();
  });

  test("has test connection button", async ({ page }, testInfo) => {
    await navigateTo(page, "/settings/email", testInfo);
    await waitForEmailSettingsReady(page);

    await expect(page.locator("#test_email")).toBeVisible();
    await expect(
      page.getByRole("button", { name: /test|send test/i }),
    ).toBeVisible();
  });
});
