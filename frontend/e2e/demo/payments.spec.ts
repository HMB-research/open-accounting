import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from './utils';

interface PaymentResponse {
	id: string;
	payment_number: string;
	payment_type: string;
	amount: number;
	reference?: string;
	allocations?: Array<{
		invoice_id: string;
		amount: number;
	}>;
}

async function waitForPaymentsLoaded(page: Page) {
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

async function openPayments(page: Page, testInfo: TestInfo) {
	const paymentsLoaded = page.waitForResponse(
		(response) =>
			response.url().includes('/payments') &&
			response.request().method() === 'GET' &&
			response.status() === 200
	);
	const invoicesLoaded = page.waitForResponse(
		(response) =>
			response.url().includes('/invoices?status=SENT') &&
			response.request().method() === 'GET' &&
			response.status() === 200
	);
	await navigateTo(page, '/payments', testInfo);
	await Promise.all([paymentsLoaded, invoicesLoaded]);
	await waitForPaymentsLoaded(page);
}

function paymentRow(page: Page, paymentNumber: string) {
	return page.locator('table tbody tr').filter({ hasText: paymentNumber });
}

test.describe('Demo Payments - Seed Data Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await openPayments(page, testInfo);
	});

	test('displays payments page content', async ({ page }) => {
		// Wait for page to load - should show heading
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
	});

	test('shows payment page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /payments/i })).toBeVisible({ timeout: 10000 });
	});

	test('has new payment button', async ({ page }) => {
		await expect(page.getByRole('button', { name: /new payment/i })).toBeVisible({ timeout: 10000 });
	});

	test('shows payment type filter', async ({ page }) => {
		// Check for the filter dropdown
		const hasFilter = await page.locator('select, [role="combobox"]').first().isVisible().catch(() => false);
		expect(hasFilter).toBeTruthy();
	});

	test('creates a received payment with an invoice allocation and filters it', async ({ page }) => {
		const reference = `E2E-PAY-${Date.now()}`;
		const notes = `Payment allocation workflow ${Date.now()}`;

		await page.getByRole('button', { name: /new payment/i }).click();
		const dialog = page.getByRole('dialog', { name: /record payment/i });
		await expect(dialog).toBeVisible();

		await dialog.locator('#type').selectOption('RECEIVED');
		await dialog.locator('#contact').selectOption({ index: 1 });
		await dialog.locator('#date').fill('2026-06-09');
		await dialog.locator('#amount').fill('125.50');
		await dialog.locator('#method').selectOption('BANK_TRANSFER');
		await dialog.locator('#reference').fill(reference);
		await dialog.locator('#notes').fill(notes);

		await dialog.getByRole('button', { name: /\+.*invoice|add invoice/i }).click();
		const allocation = dialog.locator('.allocation-row').first();
		await allocation.locator('select').selectOption({ index: 1 });
		await allocation.locator('input[type="number"]').fill('125.50');

		const createResponsePromise = page.waitForResponse((response) => {
			const path = new URL(response.url()).pathname;
			return (
				response.request().method() === 'POST' &&
				/\/api\/v1\/tenants\/[^/]+\/payments$/.test(path)
			);
		});
		await dialog.getByRole('button', { name: /^record payment$/i }).click();
		const createResponse = await createResponsePromise;
		expect(createResponse.status()).toBe(201);
		const payment = (await createResponse.json()) as PaymentResponse;
		expect(payment.payment_type).toBe('RECEIVED');
		expect(payment.reference).toBe(reference);
		expect(payment.allocations).toHaveLength(1);
		expect(payment.allocations?.[0]?.amount).toBe(125.5);

		const row = paymentRow(page, payment.payment_number);
		await expect(row).toBeVisible({ timeout: 10000 });
		await expect(row).toContainText(reference);
		await expect(row).toContainText(/received/i);
		await expect(row).toContainText(/125[,.]50/);
		await expect(row.locator('td').nth(6)).toContainText(/0[,.]00/);

		await page.locator('.filters select').first().selectOption('RECEIVED');
		await waitForPaymentsLoaded(page);
		await expect(paymentRow(page, payment.payment_number)).toBeVisible({
			timeout: 10000
		});

		await page.locator('.filters select').first().selectOption('MADE');
		await waitForPaymentsLoaded(page);
		await expect(paymentRow(page, payment.payment_number)).toHaveCount(0);
	});
});
