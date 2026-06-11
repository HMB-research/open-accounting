import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from './utils';

interface PaymentResponse {
	id: string;
	payment_number: string;
	payment_type: string;
	amount: number;
	reference?: string;
	reversal_of_payment_id?: string;
	reversed_by_payment_id?: string;
	reversal_reason?: string;
	allocations?: Array<{
		invoice_id: string;
		amount: number;
	}>;
}

interface PaymentReversalResponse {
	original_payment: PaymentResponse;
	reversal_payment: PaymentResponse;
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
	const paymentsLoaded = page.waitForResponse((response) => {
		const url = new URL(response.url());
		return (
			response.request().method() === 'GET' &&
			url.pathname.match(/\/api\/v1\/tenants\/[^/]+\/payments$/) !== null &&
			response.status() === 200
		);
	});
	const invoicesLoaded = page.waitForResponse(
		(response) =>
			response.url().includes('/invoices?status=SENT') &&
			response.request().method() === 'GET' &&
			response.status() === 200
	);
	await navigateTo(page, '/payments', testInfo, { waitForNetworkIdle: false });
	await Promise.all([paymentsLoaded, invoicesLoaded]);
	await waitForPaymentsLoaded(page);
}

function paymentRow(page: Page, paymentNumber: string) {
	return page.locator('table tbody tr').filter({ hasText: paymentNumber });
}

async function expectPaymentsShell(page: Page) {
	await expect(page.getByRole('heading', { name: /payments/i })).toBeVisible();
	await expect(page.getByRole('button', { name: /new payment/i })).toBeVisible();

	const filterSelect = page.locator('.filters select').first();
	await expect(filterSelect).toBeVisible();
	await expect(filterSelect.locator('option')).toHaveCount(3);
	await expect(page.locator('table, .empty-state').first()).toBeVisible();
}

async function selectTypeFilter(page: Page, type: '' | 'RECEIVED' | 'MADE') {
	const responsePromise = page.waitForResponse((response) => {
		const url = new URL(response.url());
		return (
			response.request().method() === 'GET' &&
			url.pathname.match(/\/api\/v1\/tenants\/[^/]+\/payments$/) !== null &&
			(type ? url.searchParams.get('type') === type : !url.searchParams.has('type')) &&
			response.status() === 200
		);
	});

	await page.locator('.filters select').first().selectOption(type);
	await responsePromise;
	await waitForPaymentsLoaded(page);
}

async function recordPayment(
	page: Page,
	options: {
		type: 'RECEIVED' | 'MADE';
		amount: string;
		reference: string;
		notes: string;
		allocateToInvoice?: boolean;
	}
) {
	await page.getByRole('button', { name: /new payment/i }).click();
	const dialog = page.getByRole('dialog', { name: /record payment/i });
	await expect(dialog).toBeVisible();

	await dialog.locator('#type').selectOption(options.type);
	await dialog.locator('#contact').selectOption({ index: 1 });
	await dialog.locator('#date').fill('2026-06-09');
	await dialog.locator('#amount').fill(options.amount);
	await dialog.locator('#method').selectOption('BANK_TRANSFER');
	await dialog.locator('#reference').fill(options.reference);
	await dialog.locator('#notes').fill(options.notes);

	if (options.allocateToInvoice) {
		await dialog.getByRole('button', { name: /\+.*invoice|add invoice/i }).click();
		const allocation = dialog.locator('.allocation-row').first();
		await allocation.locator('select').selectOption({ index: 1 });
		await allocation.locator('input[type="number"]').fill(options.amount);
	}

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
	await expect(dialog).toBeHidden();

	return (await createResponse.json()) as PaymentResponse;
}

test.describe('Demo Payments - Seed Data Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await openPayments(page, testInfo);
	});

	test('renders the payment ledger shell and loaded state', async ({ page }) => {
		await expectPaymentsShell(page);
	});

	test('creates a received payment with an invoice allocation and filters it', async ({ page }) => {
		const reference = `E2E-PAY-${Date.now()}`;
		const notes = `Payment allocation workflow ${Date.now()}`;

		const payment = await recordPayment(page, {
			type: 'RECEIVED',
			amount: '125.50',
			reference,
			notes,
			allocateToInvoice: true
		});
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

		await selectTypeFilter(page, 'RECEIVED');
		await expect(paymentRow(page, payment.payment_number)).toBeVisible({
			timeout: 10000
		});

		await selectTypeFilter(page, 'MADE');
		await expect(paymentRow(page, payment.payment_number)).toHaveCount(0);
	});

	test('reverses a payment with an auditable offsetting payment', async ({ page }) => {
		const reference = `E2E-REV-${Date.now()}`;
		const reversalReference = `REV-${reference}`;
		const reversalReason = `Duplicate demo import ${Date.now()}`;

		const payment = await recordPayment(page, {
			type: 'RECEIVED',
			amount: '42.25',
			reference,
			notes: 'Payment created for reversal workflow'
		});
		expect(payment.payment_type).toBe('RECEIVED');
		expect(payment.reference).toBe(reference);

		const originalRow = paymentRow(page, payment.payment_number);
		await expect(originalRow).toBeVisible({ timeout: 10000 });
		await expect(originalRow).toContainText(reference);

		await originalRow.getByRole('button', { name: /^reverse$/i }).click();
		const reversalDialog = page.getByRole('dialog', { name: /reverse payment/i });
		await expect(reversalDialog).toBeVisible();
		await expect(reversalDialog.locator('#reversal-original')).toHaveValue(payment.payment_number);
		await reversalDialog.locator('#reversal-date').fill('2026-06-09');
		await reversalDialog.locator('#reversal-reason').fill(reversalReason);
		await reversalDialog.locator('#reversal-reference').fill(reversalReference);
		await reversalDialog.locator('#reversal-notes').fill('Offsetting payment created by demo E2E');

		const reverseResponsePromise = page.waitForResponse((response) => {
			const path = new URL(response.url()).pathname;
			return (
				response.request().method() === 'POST' &&
				/\/api\/v1\/tenants\/[^/]+\/payments\/[^/]+\/reverse$/.test(path)
			);
		});
		await reversalDialog.getByRole('button', { name: /^reverse$/i }).click();
		const reverseResponse = await reverseResponsePromise;
		expect(reverseResponse.status()).toBe(201);
		const reversal = (await reverseResponse.json()) as PaymentReversalResponse;
		expect(reversal.original_payment.id).toBe(payment.id);
		expect(reversal.original_payment.reversed_by_payment_id).toBe(reversal.reversal_payment.id);
		expect(reversal.original_payment.reversal_reason).toBe(reversalReason);
		expect(reversal.reversal_payment.payment_type).toBe('MADE');
		expect(reversal.reversal_payment.reversal_of_payment_id).toBe(payment.id);
		expect(reversal.reversal_payment.reference).toBe(reversalReference);
		expect(reversal.reversal_payment.reversal_reason).toBe(reversalReason);

		const refreshedOriginalRow = paymentRow(page, reversal.original_payment.payment_number);
		const reversalRow = paymentRow(page, reversal.reversal_payment.payment_number);
		await expect(refreshedOriginalRow).toBeVisible({ timeout: 10000 });
		await expect(refreshedOriginalRow).toContainText(/reversed/i);
		await expect(refreshedOriginalRow.getByRole('button', { name: /^reverse$/i })).toHaveCount(0);
		await expect(reversalRow).toBeVisible({ timeout: 10000 });
		await expect(reversalRow).toContainText(/reversal/i);
		await expect(reversalRow).toContainText(reversalReference);
		await expect(reversalRow).toContainText(/made/i);
		await expect(reversalRow).toContainText(/42[,.]25/);

		await selectTypeFilter(page, 'MADE');
		await expect(paymentRow(page, reversal.reversal_payment.payment_number)).toBeVisible({
			timeout: 10000
		});
		await expect(paymentRow(page, reversal.original_payment.payment_number)).toHaveCount(0);
	});
});
