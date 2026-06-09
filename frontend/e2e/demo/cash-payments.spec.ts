import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from './utils';

interface PaymentResponse {
	id: string;
	payment_number: string;
	payment_type: 'RECEIVED' | 'MADE';
	amount: number | string;
	reference?: string;
	payment_method?: string;
}

async function waitForCashPaymentsLoaded(page: Page) {
	await expect(async () => {
		const isLoading = await page
			.getByText(/^Loading\.\.\.$/i)
			.first()
			.isVisible()
			.catch(() => false);
		const hasTable = await page
			.locator('table tbody tr')
			.first()
			.isVisible()
			.catch(() => false);
		const hasEmpty = await page
			.locator('.empty-state')
			.isVisible()
			.catch(() => false);
		expect(isLoading === false && (hasTable || hasEmpty)).toBeTruthy();
	}).toPass({ timeout: 15000 });
}

async function openCashPayments(page: Page, testInfo: TestInfo) {
	const paymentsLoaded = page.waitForResponse((response) => {
		const url = new URL(response.url());
		return (
			response.request().method() === 'GET' &&
			url.pathname.match(/\/api\/v1\/tenants\/[^/]+\/payments$/) !== null &&
			url.searchParams.get('method') === 'CASH' &&
			response.status() === 200
		);
	});
	const contactsLoaded = page.waitForResponse(
		(response) =>
			response.url().includes('/contacts?active_only=true') &&
			response.request().method() === 'GET' &&
			response.status() === 200
	);

	await navigateTo(page, '/payments/cash', testInfo);
	await Promise.all([paymentsLoaded, contactsLoaded]);
	await waitForCashPaymentsLoaded(page);
}

function cashPaymentRow(page: Page, paymentNumber: string) {
	return page.locator('table tbody tr').filter({ hasText: paymentNumber });
}

async function selectTypeFilter(page: Page, type: '' | 'RECEIVED' | 'MADE') {
	const responsePromise = page.waitForResponse((response) => {
		const url = new URL(response.url());
		return (
			response.request().method() === 'GET' &&
			url.pathname.match(/\/api\/v1\/tenants\/[^/]+\/payments$/) !== null &&
			url.searchParams.get('method') === 'CASH' &&
			(type ? url.searchParams.get('type') === type : !url.searchParams.has('type')) &&
			response.status() === 200
		);
	});

	await page.locator('.filters select').first().selectOption(type);
	await responsePromise;
	await waitForCashPaymentsLoaded(page);
}

async function recordCashPayment(
	page: Page,
	options: {
		type: 'RECEIVED' | 'MADE';
		amount: string;
		reference: string;
		notes: string;
	}
) {
	await page.getByRole('button', { name: /new cash payment/i }).click();
	const dialog = page.getByRole('dialog', { name: /record cash payment/i });
	await expect(dialog).toBeVisible();

	await dialog.locator('#type').selectOption(options.type);
	await dialog.locator('#contact').selectOption({ index: 1 });
	await dialog.locator('#date').fill('2026-06-09');
	await dialog.locator('#amount').fill(options.amount);
	await dialog.locator('#reference').fill(options.reference);
	await dialog.locator('#notes').fill(options.notes);

	const createResponsePromise = page.waitForResponse((response) => {
		const path = new URL(response.url()).pathname;
		return (
			response.request().method() === 'POST' &&
			/\/api\/v1\/tenants\/[^/]+\/payments$/.test(path)
		);
	});
	await dialog.getByRole('button', { name: /^record cash payment$/i }).click();
	const createResponse = await createResponsePromise;
	expect(createResponse.status()).toBe(201);
	await expect(dialog).toBeHidden();

	return (await createResponse.json()) as PaymentResponse;
}

test.describe('Cash Payments View', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await openCashPayments(page, testInfo);
	});

	test('displays cash payments page with correct structure', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /cash/i })).toBeVisible();
		await expect(page.locator('.summary-card.received')).toContainText(/total received/i);
		await expect(page.locator('.summary-card.made')).toContainText(/total paid/i);
		await expect(page.locator('.summary-card.balance')).toContainText(/cash balance/i);
	});

	test('has new payment button', async ({ page }) => {
		await expect(page.getByRole('button', { name: /new cash payment/i })).toBeVisible();
	});

	test('has payment type filter', async ({ page }) => {
		const filterSelect = page.locator('.filters select').first();
		await expect(filterSelect).toBeVisible();
		await expect(filterSelect.locator('option')).toHaveCount(3);
	});

	test('displays table or empty state after cash payments load', async ({ page }) => {
		const table = page.locator('table');
		const emptyState = page.locator('.empty-state');
		const hasTable = await table.isVisible().catch(() => false);
		const hasEmpty = await emptyState.isVisible().catch(() => false);

		expect(hasTable || hasEmpty).toBe(true);
	});

	test('records cash in and cash out payments and filters by type', async ({ page }) => {
		const suffix = Date.now();
		const cashInReference = `E2E-CASH-IN-${suffix}`;
		const cashOutReference = `E2E-CASH-OUT-${suffix}`;

		const cashIn = await recordCashPayment(page, {
			type: 'RECEIVED',
			amount: '42.35',
			reference: cashInReference,
			notes: `Cash receipt workflow ${suffix}`
		});
		expect(cashIn.payment_type).toBe('RECEIVED');
		expect(Number(cashIn.amount)).toBe(42.35);
		expect(cashIn.payment_method).toBe('CASH');
		expect(cashIn.reference).toBe(cashInReference);

		const cashInRow = cashPaymentRow(page, cashIn.payment_number);
		await expect(cashInRow).toBeVisible({ timeout: 10000 });
		await expect(cashInRow).toContainText(cashInReference);
		await expect(cashInRow).toContainText(/received/i);
		await expect(cashInRow).toContainText(/42[,.]35/);

		const cashOut = await recordCashPayment(page, {
			type: 'MADE',
			amount: '15.10',
			reference: cashOutReference,
			notes: `Cash payout workflow ${suffix}`
		});
		expect(cashOut.payment_type).toBe('MADE');
		expect(Number(cashOut.amount)).toBe(15.1);
		expect(cashOut.payment_method).toBe('CASH');
		expect(cashOut.reference).toBe(cashOutReference);

		const cashOutRow = cashPaymentRow(page, cashOut.payment_number);
		await expect(cashOutRow).toBeVisible({ timeout: 10000 });
		await expect(cashOutRow).toContainText(cashOutReference);
		await expect(cashOutRow).toContainText(/made/i);
		await expect(cashOutRow).toContainText(/15[,.]10/);

		await selectTypeFilter(page, 'RECEIVED');
		await expect(cashPaymentRow(page, cashIn.payment_number)).toBeVisible({ timeout: 10000 });
		await expect(cashPaymentRow(page, cashOut.payment_number)).toHaveCount(0);

		await selectTypeFilter(page, 'MADE');
		await expect(cashPaymentRow(page, cashOut.payment_number)).toBeVisible({ timeout: 10000 });
		await expect(cashPaymentRow(page, cashIn.payment_number)).toHaveCount(0);

		await selectTypeFilter(page, '');
		await expect(cashPaymentRow(page, cashIn.payment_number)).toBeVisible({ timeout: 10000 });
		await expect(cashPaymentRow(page, cashOut.payment_number)).toBeVisible({ timeout: 10000 });
	});
});
