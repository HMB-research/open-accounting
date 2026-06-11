import { test, expect, type Page, type TestInfo } from "@playwright/test";
import { ensureAuthenticated, navigateTo } from "./utils";

interface OrderResponse {
  id: string;
  order_number: string;
  status: string;
  converted_to_invoice_id?: string;
}

interface OrderInvoiceConversionResponse {
  order: OrderResponse;
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

async function waitForOrdersAndContacts(page: Page) {
  const ordersResponsePromise = page.waitForResponse((response) => {
    return (
      response.request().method() === "GET" &&
      /\/api\/v1\/tenants\/[^/]+\/orders$/.test(responsePath(response.url()))
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

  const [ordersResponse, contactsResponse] = await Promise.all([
    ordersResponsePromise,
    contactsResponsePromise,
  ]);
  expect(ordersResponse.ok()).toBeTruthy();
  expect(contactsResponse.ok()).toBeTruthy();
}

async function waitForOrdersLoaded(page: Page) {
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

async function openOrders(page: Page, testInfo: TestInfo) {
  const loadedPromise = waitForOrdersAndContacts(page);
  await navigateTo(page, "/orders", testInfo, { waitForNetworkIdle: false });
  await loadedPromise;
  await waitForOrdersLoaded(page);
}

function orderRow(page: Page, orderNumber: string) {
  return page.locator("table tbody tr").filter({ hasText: orderNumber });
}

function statusFilter(page: Page) {
  return page.locator(".filters select").first();
}

async function selectStatusFilter(page: Page, status: string) {
  const loadedPromise = waitForOrdersAndContacts(page);
  await statusFilter(page).selectOption(status);
  await loadedPromise;
  await waitForOrdersLoaded(page);
}

async function createOrder(page: Page, label: string): Promise<OrderResponse> {
  const unique = Date.now().toString(36);
  await page
    .getByRole("button", { name: /new order|uus tellimus|\+/i })
    .click();

  const modal = page.locator('[role="dialog"], .modal').first();
  await expect(modal).toBeVisible({ timeout: 5000 });
  await modal.locator("#contact").selectOption({ index: 1 });
  await modal.locator("#order-date").fill("2026-03-25");
  await modal.locator("#expected-delivery").fill("2026-03-31");
  await modal.locator(".line-description").fill(`${label} service ${unique}`);
  await modal.locator(".line-qty").fill("2");
  await modal.locator(".line-price").fill("150");
  await modal.locator(".line-vat").fill("22");
  await modal.locator("#notes").fill(`${label} E2E ${unique}`);

  const createResponsePromise = page.waitForResponse((response) => {
    return (
      response.request().method() === "POST" &&
      /\/api\/v1\/tenants\/[^/]+\/orders$/.test(responsePath(response.url()))
    );
  });
  await modal
    .getByRole("button", { name: /create order|loo tellimus/i })
    .click();
  const createResponse = await createResponsePromise;
  expect(createResponse.ok()).toBeTruthy();
  const order = (await createResponse.json()) as OrderResponse;

  const row = orderRow(page, order.order_number);
  await expect(row).toBeVisible({ timeout: 10000 });
  await expect(row).toContainText(/pending|ootel/i);

  return order;
}

async function waitForOrderActionReload(
  page: Page,
  action: () => Promise<void>,
) {
  const loadedPromise = waitForOrdersAndContacts(page);
  await action();
  await loadedPromise;
}

test.describe("Orders View", () => {
  test("covers order table, filters, deletion, lifecycle, and invoice conversion", async ({
    page,
  }, testInfo) => {
    await ensureAuthenticated(page, testInfo);
    await openOrders(page, testInfo);

    await expect(
      page.getByRole("heading", { name: /orders|tellimused/i }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /new order|uus tellimus|\+/i }),
    ).toBeVisible();
    await expect(page.locator("table tbody tr").first()).toBeVisible();
    await expect(page.locator("table")).toContainText(
      /pending|confirmed|processing|shipped|delivered|ootel|kinnitatud|töötlemisel|saadetud|tarnitud/i,
    );

    const filteredOrder = await createOrder(page, "Order filter");

    await selectStatusFilter(page, "PENDING");
    await expect(orderRow(page, filteredOrder.order_number)).toBeVisible({
      timeout: 10000,
    });

    await selectStatusFilter(page, "CONFIRMED");
    await expect(orderRow(page, filteredOrder.order_number)).toHaveCount(0);

    await selectStatusFilter(page, "");
    const filteredOrderRow = orderRow(page, filteredOrder.order_number);
    await expect(filteredOrderRow).toBeVisible({ timeout: 10000 });

    page.once("dialog", (dialog) => dialog.accept());
    const deleteResponsePromise = page.waitForResponse((response) => {
      return (
        response.request().method() === "DELETE" &&
        response.url().includes(`/orders/${filteredOrder.id}`)
      );
    });
    await filteredOrderRow
      .getByRole("button", { name: /delete|kustuta/i })
      .click();
    const deleteResponse = await deleteResponsePromise;
    expect(deleteResponse.ok()).toBeTruthy();
    await expect(filteredOrderRow).toHaveCount(0);

    const order = await createOrder(page, "Order conversion");
    let row = orderRow(page, order.order_number);

    const confirmResponsePromise = page.waitForResponse((response) => {
      return (
        response.request().method() === "POST" &&
        response.url().includes(`/orders/${order.id}/confirm`)
      );
    });
    await waitForOrderActionReload(page, () =>
      row.getByRole("button", { name: /confirm|kinnita/i }).click(),
    );
    const confirmResponse = await confirmResponsePromise;
    expect(confirmResponse.ok()).toBeTruthy();
    row = orderRow(page, order.order_number);
    await expect(row).toContainText(/confirmed|kinnitatud/i, {
      timeout: 10000,
    });

    row = orderRow(page, order.order_number);
    const processResponsePromise = page.waitForResponse((response) => {
      return (
        response.request().method() === "POST" &&
        response.url().includes(`/orders/${order.id}/process`)
      );
    });
    await waitForOrderActionReload(page, () =>
      row.getByRole("button", { name: /process|töötle/i }).click(),
    );
    const processResponse = await processResponsePromise;
    expect(processResponse.ok()).toBeTruthy();
    row = orderRow(page, order.order_number);
    await expect(row).toContainText(/processing|töötlemisel/i, {
      timeout: 10000,
    });

    row = orderRow(page, order.order_number);
    const shipResponsePromise = page.waitForResponse((response) => {
      return (
        response.request().method() === "POST" &&
        response.url().includes(`/orders/${order.id}/ship`)
      );
    });
    await waitForOrderActionReload(page, () =>
      row.getByRole("button", { name: /ship|saada/i }).click(),
    );
    const shipResponse = await shipResponsePromise;
    expect(shipResponse.ok()).toBeTruthy();
    row = orderRow(page, order.order_number);
    await expect(row).toContainText(/shipped|saadetud/i, {
      timeout: 10000,
    });

    row = orderRow(page, order.order_number);
    const deliverResponsePromise = page.waitForResponse((response) => {
      return (
        response.request().method() === "POST" &&
        response.url().includes(`/orders/${order.id}/deliver`)
      );
    });
    await waitForOrderActionReload(page, () =>
      row.getByRole("button", { name: /deliver|tarni/i }).click(),
    );
    const deliverResponse = await deliverResponsePromise;
    expect(deliverResponse.ok()).toBeTruthy();
    row = orderRow(page, order.order_number);
    await expect(row).toContainText(/delivered|tarnitud/i, {
      timeout: 10000,
    });

    row = orderRow(page, order.order_number);
    await expect(
      row.getByRole("button", { name: /convert to invoice|teisenda arveks/i }),
    ).toBeVisible();

    const conversionResponsePromise = page.waitForResponse((response) => {
      return (
        response.request().method() === "POST" &&
        response.url().includes(`/orders/${order.id}/convert-to-invoice`)
      );
    });
    await waitForOrderActionReload(page, () =>
      row
        .getByRole("button", { name: /convert to invoice|teisenda arveks/i })
        .click(),
    );
    const conversionResponse = await conversionResponsePromise;
    expect(conversionResponse.status()).toBe(201);
    const result =
      (await conversionResponse.json()) as OrderInvoiceConversionResponse;
    expect(result.order.status).toBe("DELIVERED");
    expect(result.order.order_number).toBe(order.order_number);
    expect(result.order.converted_to_invoice_id).toBe(result.invoice.id);
    expect(result.invoice.id).toBeTruthy();
    expect(result.invoice.invoice_number).toBeTruthy();
    expect(result.invoice.reference).toBe(order.order_number);
    expect(result.invoice.status).toBe("DRAFT");

    row = orderRow(page, order.order_number);
    await expect(
      row.getByRole("button", { name: /convert to invoice|teisenda arveks/i }),
    ).toHaveCount(0);
  });
});
