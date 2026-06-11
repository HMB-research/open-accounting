import {
  test,
  expect,
  type Page,
  type Response,
  type TestInfo,
} from "@playwright/test";
import {
  ensureAuthenticated,
  navigateTo,
  ensureDemoTenant,
  waitForRouteReady,
  getDemoCredentials,
} from "./utils";

interface TenantResponse {
  id: string;
  name: string;
  settings?: {
    reg_code?: string;
    vat_number?: string;
    email?: string;
    phone?: string;
    bank_details?: string;
    timezone?: string;
  };
}

function responsePath(responseUrl: string): string {
  return new URL(responseUrl).pathname;
}

function tenantResponse(tenantId: string) {
  return (response: Response): boolean =>
    response.request().method() === "GET" &&
    response.status() === 200 &&
    responsePath(response.url()).endsWith(`/tenants/${tenantId}`);
}

function periodCloseEventsResponse(tenantId: string) {
  return (response: Response): boolean =>
    response.request().method() === "GET" &&
    response.status() === 200 &&
    responsePath(response.url()).endsWith(
      `/tenants/${tenantId}/period-close-events`,
    );
}

function updateTenantResponse(tenantId: string) {
  return (response: Response): boolean =>
    response.request().method() === "PUT" &&
    response.status() === 200 &&
    responsePath(response.url()).endsWith(`/tenants/${tenantId}`);
}

async function openSettingsOverview(page: Page, testInfo: TestInfo) {
  await navigateTo(page, "/settings", testInfo, {
    waitForNetworkIdle: false,
  });
  await waitForRouteReady(page, ".settings-grid");
}

async function openCompanySettingsFromOverview(
  page: Page,
  testInfo: TestInfo,
): Promise<TenantResponse> {
  const { tenantId } = getDemoCredentials(testInfo);
  const tenantLoaded = page.waitForResponse(tenantResponse(tenantId));
  const historyLoaded = page.waitForResponse(
    periodCloseEventsResponse(tenantId),
  );

  await page
    .locator(`a.settings-card[href="/settings/company?tenant=${tenantId}"]`)
    .click();

  const tenantResult = await tenantLoaded;
  await historyLoaded;
  await waitForRouteReady(page, "#company-settings-form");

  return (await tenantResult.json()) as TenantResponse;
}

test.describe("Demo Settings", () => {
  test.beforeEach(async ({ page }, testInfo) => {
    await ensureAuthenticated(page, testInfo);
    await ensureDemoTenant(page, testInfo);
  });

  test("verifies settings destinations and saves company settings", async ({
    page,
  }, testInfo) => {
    const { tenantId } = getDemoCredentials(testInfo);
    await openSettingsOverview(page, testInfo);

    await expect(
      page.getByRole("heading", { name: /settings|seaded/i, level: 1 }),
    ).toBeVisible();

    const expectedCardTargets = [
      "/settings/company",
      "/settings/email",
      "/settings/plugins",
      "/settings/interest",
      "/settings/audit",
      "/documents",
      "/settings/users",
    ];

    await expect(page.locator("a.settings-card")).toHaveCount(
      expectedCardTargets.length,
    );
    for (const target of expectedCardTargets) {
      await expect(
        page.locator(`a.settings-card[href="${target}?tenant=${tenantId}"]`),
      ).toBeVisible();
    }

    const loadedTenant = await openCompanySettingsFromOverview(page, testInfo);
    await expect(page).toHaveURL(
      new RegExp(`/settings/company\\?tenant=${tenantId}`),
    );
    await expect(page.locator("#companyName")).toHaveValue(loadedTenant.name);
    await expect(page.locator("#currency")).toBeDisabled();
    await expect(page.locator("#timezone")).toHaveValue(
      loadedTenant.settings?.timezone || "Europe/Tallinn",
    );
    await expect(page.locator("#regCode")).toBeVisible();
    await expect(page.locator("#vatNumber")).toBeVisible();
    await expect(page.locator("#email")).toBeVisible();
    await expect(page.locator("#phone")).toBeVisible();
    await expect(page.locator("#bankDetails")).toBeVisible();
    await expect(page.locator("#invoiceTerms")).toBeVisible();
    await expect(page.locator("#period-history")).toBeVisible();

    const suffix = `${testInfo.parallelIndex}${testInfo.repeatEachIndex}${Date.now()
      .toString()
      .slice(-4)}`;
    const regCode = `9${suffix}`.slice(0, 8);
    const vatNumber = `EE${regCode}`;
    const email = `settings-${suffix}@example.com`;
    const phone = `+372 55${suffix.slice(-6)}`;
    const bankDetails = `E2E settings bank reference ${suffix}`;

    await page.locator("#regCode").fill(regCode);
    await page.locator("#vatNumber").fill(vatNumber);
    await page.locator("#email").fill(email);
    await page.locator("#phone").fill(phone);
    await page.locator("#bankDetails").fill(bankDetails);

    const updateResponsePromise = page.waitForResponse(
      updateTenantResponse(tenantId),
    );
    await page
      .locator("form#company-settings-form button[type='submit']")
      .click();
    const updateResponse = await updateResponsePromise;
    const updatePayload = updateResponse.request().postDataJSON() as {
      settings?: Record<string, unknown>;
    };
    const updatedTenant = (await updateResponse.json()) as TenantResponse;

    expect(updatePayload.settings?.reg_code).toBe(regCode);
    expect(updatePayload.settings?.vat_number).toBe(vatNumber);
    expect(updatePayload.settings?.email).toBe(email);
    expect(updatePayload.settings?.phone).toBe(phone);
    expect(updatePayload.settings?.bank_details).toBe(bankDetails);
    expect(updatedTenant.settings?.reg_code).toBe(regCode);
    expect(updatedTenant.settings?.vat_number).toBe(vatNumber);
    expect(updatedTenant.settings?.email).toBe(email);
    expect(updatedTenant.settings?.phone).toBe(phone);
    expect(updatedTenant.settings?.bank_details).toBe(bankDetails);
    await expect(page.locator(".alert-success")).toContainText(
      /settings saved|seaded salvestatud/i,
    );
    await expect(page.locator("#regCode")).toHaveValue(regCode);
  });
});
