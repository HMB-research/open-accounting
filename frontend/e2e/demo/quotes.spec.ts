import { test, expect, type Page, type TestInfo } from "@playwright/test";
import { ensureAuthenticated, navigateTo } from "./utils";

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

function responsePath(responseUrl: string) {
  return new URL(responseUrl).pathname;
}

async function waitForQuotesAndContacts(page: Page) {
  const quotesResponsePromise = page.waitForResponse((response) => {
    return (
      response.request().method() === "GET" &&
      /\/api\/v1\/tenants\/[^/]+\/quotes$/.test(responsePath(response.url()))
    );
  });
  const contactsResponsePromise = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return (
      response.request().method() === "GET" &&
      /\/api\/v1\/tenants\/[^/]+\/contacts$/.test(url.pathname) &&
      url.searchParams.get("active_only") === "true"
    );
  });

  const [quotesResponse, contactsResponse] = await Promise.all([
    quotesResponsePromise,
    contactsResponsePromise,
  ]);
  expect(quotesResponse.ok()).toBeTruthy();
  expect(contactsResponse.ok()).toBeTruthy();
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
  const loadedPromise = waitForQuotesAndContacts(page);
  await navigateTo(page, "/quotes", testInfo, { waitForNetworkIdle: false });
  await loadedPromise;
  await waitForQuotesLoaded(page);
}

function quoteRow(page: Page, quoteNumber: string) {
  return page.locator("table tbody tr").filter({ hasText: quoteNumber });
}

function statusFilter(page: Page) {
  return page.locator(".filters select").first();
}

async function selectStatusFilter(page: Page, status: string) {
  const loadedPromise = waitForQuotesAndContacts(page);
  await statusFilter(page).selectOption(status);
  await loadedPromise;
  await waitForQuotesLoaded(page);
}

async function createQuote(page: Page, label: string): Promise<QuoteResponse> {
  const unique = Date.now().toString(36);
  await page
    .getByRole("button", { name: /new quote|uus pakkumine|\+/i })
    .click();

  const modal = page.getByRole("dialog", { name: /new quote|uus pakkumine/i });
  await expect(modal).toBeVisible({ timeout: 5000 });
  await modal.locator("#contact").selectOption({ index: 1 });
  await modal
    .locator(".line-description")
    .fill(`${label} service ${unique}`);
  await modal.locator(".line-qty").fill("2");
  await modal.locator(".line-price").fill("125");
  await modal.locator(".line-vat").fill("22");
  await modal.locator("#notes").fill(`${label} E2E ${unique}`);

  const createResponsePromise = page.waitForResponse((response) => {
    return (
      response.request().method() === "POST" &&
      /\/api\/v1\/tenants\/[^/]+\/quotes$/.test(responsePath(response.url()))
    );
  });
  await modal
    .getByRole("button", { name: /create quote|loo pakkumine/i })
    .click();
  const createResponse = await createResponsePromise;
  expect(createResponse.status()).toBe(201);
  const quote = (await createResponse.json()) as QuoteResponse;

  const row = quoteRow(page, quote.quote_number);
  await expect(row).toBeVisible({ timeout: 10000 });
  await expect(row).toContainText(/draft|mustand/i);

  return quote;
}

async function waitForQuoteActionReload(page: Page, action: () => Promise<void>) {
  const loadedPromise = waitForQuotesAndContacts(page);
  await action();
  await loadedPromise;
}

test.describe("Quotes View", () => {
  test("covers quote table, filters, deletion, and invoice conversion", async ({
    page,
  }, testInfo) => {
    await ensureAuthenticated(page, testInfo);
    await openQuotes(page, testInfo);

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

    const filteredQuote = await createQuote(page, "Quote filter");

    await selectStatusFilter(page, "DRAFT");
    await expect(quoteRow(page, filteredQuote.quote_number)).toBeVisible({
      timeout: 10000,
    });

    await selectStatusFilter(page, "SENT");
    await expect(quoteRow(page, filteredQuote.quote_number)).toHaveCount(0);

    await selectStatusFilter(page, "");
    const filteredQuoteRow = quoteRow(page, filteredQuote.quote_number);
    await expect(filteredQuoteRow).toBeVisible({ timeout: 10000 });
    page.once("dialog", (dialog) => dialog.accept());
    const deleteResponsePromise = page.waitForResponse((response) => {
      return (
        response.request().method() === "DELETE" &&
        response.url().includes(`/quotes/${filteredQuote.id}`)
      );
    });
    await filteredQuoteRow
      .getByRole("button", { name: /delete|kustuta/i })
      .click();
    const deleteResponse = await deleteResponsePromise;
    expect(deleteResponse.ok()).toBeTruthy();
    await expect(filteredQuoteRow).toHaveCount(0);

    const conversionQuote = await createQuote(page, "Quote conversion");
    const conversionRow = quoteRow(page, conversionQuote.quote_number);

    const sendResponsePromise = page.waitForResponse((response) => {
      return (
        response.request().method() === "POST" &&
        response.url().includes(`/quotes/${conversionQuote.id}/send`)
      );
    });
    await waitForQuoteActionReload(page, () =>
      conversionRow.getByRole("button", { name: /send|saada/i }).click(),
    );
    const sendResponse = await sendResponsePromise;
    expect(sendResponse.ok()).toBeTruthy();
    await expect(conversionRow).toContainText(/sent|saadetud/i, {
      timeout: 10000,
    });

    const acceptResponsePromise = page.waitForResponse((response) => {
      return (
        response.request().method() === "POST" &&
        response.url().includes(`/quotes/${conversionQuote.id}/accept`)
      );
    });
    await waitForQuoteActionReload(page, () =>
      conversionRow.getByRole("button", { name: /accept|kinnita/i }).click(),
    );
    const acceptResponse = await acceptResponsePromise;
    expect(acceptResponse.ok()).toBeTruthy();
    await expect(conversionRow).toContainText(/accepted|kinnitatud/i, {
      timeout: 10000,
    });
    await expect(
      conversionRow.getByRole("button", {
        name: /convert to invoice|teisenda arveks/i,
      }),
    ).toBeVisible();

    const conversionResponsePromise = page.waitForResponse((response) => {
      return (
        response.request().method() === "POST" &&
        response
          .url()
          .includes(`/quotes/${conversionQuote.id}/convert-to-invoice`)
      );
    });
    await waitForQuoteActionReload(page, () =>
      conversionRow
        .getByRole("button", {
          name: /convert to invoice|teisenda arveks/i,
        })
        .click(),
    );
    const conversionResponse = await conversionResponsePromise;
    expect(conversionResponse.status()).toBe(201);
    const result =
      (await conversionResponse.json()) as QuoteInvoiceConversionResponse;
    expect(result.quote.status).toBe("CONVERTED");
    expect(result.quote.quote_number).toBe(conversionQuote.quote_number);
    expect(result.invoice.id).toBeTruthy();
    expect(result.invoice.invoice_number).toBeTruthy();
    expect(result.invoice.reference).toBe(conversionQuote.quote_number);
    expect(result.invoice.status).toBe("DRAFT");

    await expect(conversionRow).toContainText(/converted|teisendatud/i, {
      timeout: 10000,
    });
  });
});
