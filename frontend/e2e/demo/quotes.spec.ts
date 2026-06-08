import { test, expect, type Page, type TestInfo } from "@playwright/test";
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from "./utils";

interface QuoteResponse {
  id: string;
  quote_number: string;
  status: string;
}

interface QuoteInvoiceConversionResponse {
  quote: QuoteResponse;
  invoice: {
    id: string;
    invoice_number: string;
    reference?: string;
    status: string;
  };
}

async function waitForQuotesLoaded(page: Page) {
  await expect(async () => {
    const isLoading = await page
      .getByText(/^Loading\.\.\.$/i)
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
    expect(isLoading === false && (hasTable || hasEmpty)).toBeTruthy();
  }).toPass({ timeout: 15000 });
}

async function openQuotes(page: Page, testInfo: TestInfo) {
  await navigateTo(page, "/quotes", testInfo);
  await waitForQuotesLoaded(page);
}

function quoteRow(page: Page, quoteNumber: string) {
  return page.locator("table tbody tr").filter({ hasText: quoteNumber });
}

function statusFilter(page: Page) {
  return page.locator(".filters select").first();
}

async function createQuote(page: Page): Promise<QuoteResponse> {
  const unique = Date.now().toString(36);
  await page
    .getByRole("button", { name: /new quote|uus pakkumine|\+/i })
    .click();

  const modal = page.locator('[role="dialog"], .modal').first();
  await expect(modal).toBeVisible({ timeout: 5000 });
  await modal.locator("#contact").selectOption({ index: 1 });
  await modal
    .locator(".line-description")
    .fill(`Quote conversion service ${unique}`);
  await modal.locator(".line-qty").fill("2");
  await modal.locator(".line-price").fill("125");
  await modal.locator(".line-vat").fill("22");
  await modal.locator("#notes").fill(`Quote lifecycle E2E ${unique}`);

  const createResponsePromise = page.waitForResponse((response) => {
    return (
      response.request().method() === "POST" &&
      /\/api\/v1\/tenants\/[^/]+\/quotes$/.test(
        new URL(response.url()).pathname,
      )
    );
  });
  await modal
    .getByRole("button", { name: /create quote|loo pakkumine/i })
    .click();
  const createResponse = await createResponsePromise;
  expect(createResponse.ok()).toBeTruthy();
  const quote = (await createResponse.json()) as QuoteResponse;

  const row = quoteRow(page, quote.quote_number);
  await expect(row).toBeVisible({ timeout: 10000 });
  await expect(row).toContainText(/draft|mustand/i);

  return quote;
}

test.describe("Quotes View", () => {
  test.beforeEach(async ({ page }, testInfo) => {
    await ensureAuthenticated(page, testInfo);
    await ensureDemoTenant(page, testInfo);
    await openQuotes(page, testInfo);
  });

  test("displays seeded quotes with statuses and controls", async ({
    page,
  }) => {
    await expect(
      page.getByRole("heading", { name: /quotes|pakkumised/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /new quote|uus pakkumine|\+/i }),
    ).toBeVisible();
    await expect(page.locator("table tbody tr").first()).toBeVisible();
    await expect(page.locator("table")).toContainText(
      /draft|sent|accepted|converted|mustand|saadetud|kinnitatud|teisendatud/i,
    );
  });

  test("filters quotes by status", async ({ page }) => {
    const quote = await createQuote(page);

    await statusFilter(page).selectOption("DRAFT");
    await waitForQuotesLoaded(page);
    await expect(quoteRow(page, quote.quote_number)).toBeVisible({
      timeout: 10000,
    });

    await statusFilter(page).selectOption("SENT");
    await waitForQuotesLoaded(page);
    await expect(quoteRow(page, quote.quote_number)).toHaveCount(0);
  });

  test("creates and deletes a draft quote", async ({ page }) => {
    const quote = await createQuote(page);
    const row = quoteRow(page, quote.quote_number);

    page.once("dialog", (dialog) => dialog.accept());
    const deleteResponsePromise = page.waitForResponse((response) => {
      return (
        response.request().method() === "DELETE" &&
        response.url().includes(`/quotes/${quote.id}`)
      );
    });
    await row.getByRole("button", { name: /delete|kustuta/i }).click();
    const deleteResponse = await deleteResponsePromise;
    expect(deleteResponse.ok()).toBeTruthy();
    await expect(row).toHaveCount(0);
  });

  test("sends, accepts, and converts a quote into a draft invoice", async ({
    page,
  }) => {
    const quote = await createQuote(page);
    let row = quoteRow(page, quote.quote_number);

    const sendResponsePromise = page.waitForResponse((response) => {
      return (
        response.request().method() === "POST" &&
        response.url().includes(`/quotes/${quote.id}/send`)
      );
    });
    await row.getByRole("button", { name: /send|saada/i }).click();
    const sendResponse = await sendResponsePromise;
    expect(sendResponse.ok()).toBeTruthy();
    await expect(row).toContainText(/sent|saadetud/i, { timeout: 10000 });

    const acceptResponsePromise = page.waitForResponse((response) => {
      return (
        response.request().method() === "POST" &&
        response.url().includes(`/quotes/${quote.id}/accept`)
      );
    });
    await row.getByRole("button", { name: /accept|kinnita/i }).click();
    const acceptResponse = await acceptResponsePromise;
    expect(acceptResponse.ok()).toBeTruthy();
    await expect(row).toContainText(/accepted|kinnitatud/i, { timeout: 10000 });

    row = quoteRow(page, quote.quote_number);
    await expect(
      row.getByRole("button", { name: /convert to invoice|teisenda arveks/i }),
    ).toBeVisible();

    const conversionResponsePromise = page.waitForResponse((response) => {
      return (
        response.request().method() === "POST" &&
        response.url().includes(`/quotes/${quote.id}/convert-to-invoice`)
      );
    });
    await row
      .getByRole("button", { name: /convert to invoice|teisenda arveks/i })
      .click();
    const conversionResponse = await conversionResponsePromise;
    expect(conversionResponse.status()).toBe(201);
    const result =
      (await conversionResponse.json()) as QuoteInvoiceConversionResponse;
    expect(result.quote.status).toBe("CONVERTED");
    expect(result.quote.quote_number).toBe(quote.quote_number);
    expect(result.invoice.id).toBeTruthy();
    expect(result.invoice.invoice_number).toBeTruthy();
    expect(result.invoice.reference).toBe(quote.quote_number);
    expect(result.invoice.status).toBe("DRAFT");

    await expect(quoteRow(page, quote.quote_number)).toContainText(
      /converted|teisendatud/i,
      { timeout: 10000 },
    );
  });
});
