import { test, expect, type Page, type TestInfo } from "@playwright/test";
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from "./utils";

interface ContactResponse {
  id: string;
  name: string;
  contact_type: "CUSTOMER" | "SUPPLIER" | "BOTH";
  email?: string;
  phone?: string;
  vat_number?: string;
  payment_terms_days?: number;
}

async function waitForContactsLoaded(page: Page) {
  await expect(async () => {
    const isLoading = await page
      .getByText(/^Loading\.\.\.$|^Laadimine\.\.\.$/i)
      .first()
      .isVisible()
      .catch(() => false);
    const hasTable = await page
      .locator("table tbody tr")
      .first()
      .isVisible()
      .catch(() => false);
    const hasEmpty = await page
      .locator(".empty-state")
      .isVisible()
      .catch(() => false);
    expect(!isLoading && (hasTable || hasEmpty)).toBeTruthy();
  }).toPass({ timeout: 15000 });
}

async function openContacts(page: Page, testInfo: TestInfo) {
  await navigateTo(page, "/contacts", testInfo);
  await waitForContactsLoaded(page);
}

function contactRow(page: Page, name: string) {
  return page.locator("table tbody tr").filter({ hasText: name });
}

function typeFilter(page: Page) {
  return page.locator(".filters select").first();
}

function searchInput(page: Page) {
  return page.locator(".filters input.search-input");
}

async function searchContacts(page: Page, query: string) {
  await searchInput(page).fill(query);
  await page.getByRole("button", { name: /search|otsi/i }).click();
  await waitForContactsLoaded(page);
}

async function createContact(
  page: Page,
  overrides: Partial<ContactResponse> = {},
): Promise<ContactResponse> {
  const unique = Date.now().toString(36);
  const name = overrides.name ?? `Workflow Contact ${unique}`;
  await page.getByRole("button", { name: /new contact|uus kontakt/i }).click();

  const modal = page.locator('[role="dialog"], .modal').first();
  await expect(modal).toBeVisible({ timeout: 5000 });
  await modal.locator("#contact-name").fill(name);
  await modal
    .locator("#contact-type")
    .selectOption(overrides.contact_type ?? "CUSTOMER");
  await modal
    .locator("#contact-email")
    .fill(overrides.email ?? `workflow-${unique}@example.com`);
  await modal.locator("#contact-phone").fill(overrides.phone ?? "+3725550101");
  await modal
    .locator("#contact-vat")
    .fill(overrides.vat_number ?? "EE123456789");
  await modal.locator("#contact-address").fill("Workflow Street 1");
  await modal.locator("#contact-city").fill("Tallinn");
  await modal.locator("#contact-postal").fill("10111");
  await modal.locator("#contact-country").selectOption("EE");
  await modal
    .locator("#contact-payment-days")
    .fill(String(overrides.payment_terms_days ?? 21));

  const createResponsePromise = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return (
      response.request().method() === "POST" &&
      /\/api\/v1\/tenants\/[^/]+\/contacts$/.test(url.pathname)
    );
  });
  await modal.getByRole("button", { name: /create|loo/i }).click();
  const response = await createResponsePromise;
  expect(response.ok()).toBeTruthy();
  const contact = (await response.json()) as ContactResponse;

  await expect(contactRow(page, contact.name)).toBeVisible({ timeout: 10000 });
  return contact;
}

test.describe("Demo Contacts", () => {
  test.beforeEach(async ({ page }, testInfo) => {
    await ensureAuthenticated(page, testInfo);
    await ensureDemoTenant(page, testInfo);
    await openContacts(page, testInfo);
  });

  test("displays seeded contacts, filters, edits, and deletes", async ({
    page,
  }) => {
    await expect(
      page.getByRole("heading", { name: /contacts|kontaktid/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /new contact|uus kontakt/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /import contacts|impordi kontaktid/i }),
    ).toBeVisible();
    await expect(page.locator("table tbody tr").first()).toBeVisible();
    await expect(page.getByText("TechStart").first()).toBeVisible();
    await expect(page.getByText("Nordic").first()).toBeVisible();
    await expect(page.getByText("Office Supplies Ltd")).toBeVisible();
    await expect(page.locator("table tbody tr")).toHaveCount(7, {
      timeout: 10000,
    });
    await expect(page.locator("table")).toContainText("@");
    await expect(
      page
        .locator("table")
        .getByRole("button", { name: /edit|muuda/i })
        .first(),
    ).toBeVisible();
    await expect(
      page
        .locator("table")
        .getByRole("button", { name: /delete|kustuta/i })
        .first(),
    ).toBeVisible();

    const unique = Date.now().toString(36);
    const contact = await createContact(page, {
      name: `Workflow Customer ${unique}`,
      contact_type: "CUSTOMER",
      email: `customer-${unique}@example.com`,
      payment_terms_days: 21,
    });

    await searchContacts(page, contact.name);
    await expect(contactRow(page, contact.name)).toBeVisible({
      timeout: 10000,
    });

    await typeFilter(page).selectOption("CUSTOMER");
    await waitForContactsLoaded(page);
    await expect(contactRow(page, contact.name)).toBeVisible({
      timeout: 10000,
    });

    await typeFilter(page).selectOption("SUPPLIER");
    await waitForContactsLoaded(page);
    await expect(contactRow(page, contact.name)).toHaveCount(0);

    await typeFilter(page).selectOption("");
    await waitForContactsLoaded(page);
    await searchContacts(page, "");

    const editableContact = await createContact(page, {
      name: `Workflow Editable ${unique}`,
      contact_type: "CUSTOMER",
      email: `editable-${unique}@example.com`,
      phone: "+3725550202",
      payment_terms_days: 14,
    });

    const updatedName = `Workflow Edited ${unique}`;
    const updatedEmail = `edited-${unique}@example.com`;
    let row = contactRow(page, editableContact.name);
    await row.getByRole("button", { name: /edit|muuda/i }).click();

    const modal = page.locator('[role="dialog"], .modal').first();
    await expect(modal).toBeVisible({ timeout: 5000 });
    await expect(
      modal.getByRole("heading", { name: /edit contact|muuda kontakti/i }),
    ).toBeVisible();
    await modal.locator("#contact-name").fill(updatedName);
    await modal.locator("#contact-email").fill(updatedEmail);
    await modal.locator("#contact-phone").fill("+3725550303");
    await modal.locator("#contact-payment-days").fill("30");

    const updateResponsePromise = page.waitForResponse(
      (response) =>
        response.request().method() === "PUT" &&
        response.url().includes(`/contacts/${editableContact.id}`),
    );
    await modal.getByRole("button", { name: /save|salvesta/i }).click();
    const updateResponse = await updateResponsePromise;
    expect(updateResponse.ok()).toBeTruthy();
    const updated = (await updateResponse.json()) as ContactResponse;
    expect(updated.name).toBe(updatedName);
    expect(updated.email).toBe(updatedEmail);
    expect(updated.payment_terms_days).toBe(30);

    row = contactRow(page, updatedName);
    await expect(row).toBeVisible({ timeout: 10000 });
    await expect(row).toContainText(updatedEmail);
    await expect(row).toContainText(/30/);

    page.once("dialog", (dialog) => dialog.accept());
    const deleteResponsePromise = page.waitForResponse(
      (response) =>
        response.request().method() === "DELETE" &&
        response.url().includes(`/contacts/${editableContact.id}`),
    );
    await row.getByRole("button", { name: /delete|kustuta/i }).click();
    const deleteResponse = await deleteResponsePromise;
    expect(deleteResponse.ok()).toBeTruthy();
    await expect(contactRow(page, updatedName)).toHaveCount(0);
  });
});
