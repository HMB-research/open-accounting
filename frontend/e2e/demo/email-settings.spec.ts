import { test, expect, type Page, type TestInfo } from "@playwright/test";
import {
  ensureAuthenticated,
  navigateTo,
  ensureDemoTenant,
  waitForRouteReady,
} from "./utils";

interface SMTPConfigResponse {
  smtp_host: string;
  smtp_port: number;
  smtp_username: string;
  smtp_from_email: string;
  smtp_from_name: string;
  smtp_use_tls: boolean;
}

function isSMTPConfigPath(path: string): boolean {
  return /\/api\/v1\/tenants\/[^/]+\/settings\/smtp$/.test(path);
}

function waitForSMTPConfig(page: Page) {
  return page.waitForResponse((response) => {
    const path = new URL(response.url()).pathname;
    return (
      response.request().method() === "GET" &&
      isSMTPConfigPath(path) &&
      response.status() === 200
    );
  });
}

function waitForSMTPUpdate(page: Page) {
  return page.waitForResponse((response) => {
    const path = new URL(response.url()).pathname;
    return (
      response.request().method() === "PUT" &&
      isSMTPConfigPath(path) &&
      response.status() === 200
    );
  });
}

async function openEmailSettings(page: Page, testInfo: TestInfo) {
  const smtpLoaded = waitForSMTPConfig(page);
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
  test("displays, saves, and reloads SMTP settings", async ({
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

    const suffix = `${Date.now()}-${testInfo.workerIndex}`;
    const expectedConfig: SMTPConfigResponse = {
      smtp_host: `smtp-${suffix}.example.test`,
      smtp_port: 2525,
      smtp_username: `robot-${suffix}`,
      smtp_from_email: `billing-${suffix}@example.test`,
      smtp_from_name: `Billing ${suffix}`,
      smtp_use_tls: false,
    };

    await page.locator("#smtp_host").fill(expectedConfig.smtp_host);
    await page.locator("#smtp_port").fill(String(expectedConfig.smtp_port));
    await page.locator("#smtp_username").fill(expectedConfig.smtp_username);
    await page.locator("#smtp_password").fill(`secret-${suffix}`);
    await page.locator("#smtp_from_email").fill(expectedConfig.smtp_from_email);
    await page.locator("#smtp_from_name").fill(expectedConfig.smtp_from_name);
    await page
      .locator('input[type="checkbox"]')
      .setChecked(expectedConfig.smtp_use_tls);

    const updateResponsePromise = waitForSMTPUpdate(page);
    await form
      .getByRole("button", { name: /save|update|apply/i })
      .first()
      .click();
    const updateResponse = await updateResponsePromise;
    expect(await updateResponse.json()).toMatchObject({ status: "updated" });
    await expect(page.locator(".alert-success")).toContainText(
      /saved|salvestatud/i,
    );

    const reloadResponsePromise = waitForSMTPConfig(page);
    await page.reload();
    const reloadResponse = await reloadResponsePromise;
    const reloadedConfig = (await reloadResponse.json()) as SMTPConfigResponse;
    expect(reloadedConfig).toMatchObject(expectedConfig);
    await waitForRouteReady(page, "main h1, .tabs, #smtp_host");

    await expect(page.locator("#smtp_host")).toHaveValue(
      expectedConfig.smtp_host,
    );
    await expect(page.locator("#smtp_port")).toHaveValue(
      String(expectedConfig.smtp_port),
    );
    await expect(page.locator("#smtp_username")).toHaveValue(
      expectedConfig.smtp_username,
    );
    await expect(page.locator("#smtp_from_email")).toHaveValue(
      expectedConfig.smtp_from_email,
    );
    await expect(page.locator("#smtp_from_name")).toHaveValue(
      expectedConfig.smtp_from_name,
    );
    await expect(page.locator('input[type="checkbox"]')).not.toBeChecked();
  });
});
