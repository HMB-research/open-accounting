import { test, expect, type Page, type TestInfo } from "@playwright/test";
import {
  ensureAuthenticated,
  navigateTo,
  ensureDemoTenant,
  waitForRouteReady,
} from "./utils";

async function openEmailSettings(page: Page, testInfo: TestInfo) {
  const smtpLoaded = page.waitForResponse((response) => {
    const path = new URL(response.url()).pathname;
    return (
      response.request().method() === "GET" &&
      path.endsWith("/settings/smtp") &&
      response.status() === 200
    );
  });
  const templatesLoaded = page.waitForResponse((response) => {
    const path = new URL(response.url()).pathname;
    return (
      response.request().method() === "GET" &&
      path.endsWith("/email-templates") &&
      response.status() === 200
    );
  });
  const logLoaded = page.waitForResponse((response) => {
    const path = new URL(response.url()).pathname;
    return (
      response.request().method() === "GET" &&
      path.endsWith("/email-log") &&
      response.status() === 200
    );
  });

  await navigateTo(page, "/settings/email", testInfo, {
    waitForNetworkIdle: false,
  });
  await Promise.all([smtpLoaded, templatesLoaded, logLoaded]);
  await waitForRouteReady(page, "main h1, .tabs, #smtp_host");
  await expect(
    page.getByRole("heading", { name: /email|smtp|mail/i }).first(),
  ).toBeVisible();
  await expect(page.locator("#smtp_host")).toBeVisible();
}

test.describe("Email Settings View", () => {
  test("displays SMTP settings structure and controls", async ({
    page,
  }, testInfo) => {
    await ensureAuthenticated(page, testInfo);
    await ensureDemoTenant(page, testInfo);
    await openEmailSettings(page, testInfo);

    const tabs = page.locator(".tabs");
    await expect(tabs.getByRole("button", { name: /smtp/i })).toBeVisible();
    await expect(tabs.getByRole("button", { name: /template/i })).toBeVisible();
    await expect(tabs.getByRole("button", { name: /log/i })).toBeVisible();

    const form = page.locator("form").first();
    await expect(form).toBeVisible();
    await expect(page.locator("#smtp_host")).toBeVisible();
    await expect(page.locator("#smtp_port")).toBeVisible();
    await expect(page.locator("#smtp_username")).toBeVisible();
    await expect(page.locator("#smtp_from_email")).toBeVisible();

    await expect(
      form.getByRole("button", { name: /save|update|apply/i }).first(),
    ).toBeVisible();
    await expect(page.locator("#test_email")).toBeVisible();
    await expect(
      page.getByRole("button", { name: /test|send test/i }),
    ).toBeVisible();
  });
});
