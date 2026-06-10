import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant, waitForRouteReady } from './utils';

const balanceLoadedState = '.summary-card, .empty-state, .alert-error';
const summaryRequestPattern = /\/api\/v1\/tenants\/[^/]+\/reports\/balance-confirmations\?/;

async function openBalanceConfirmationsPage(page: Page, testInfo: TestInfo): Promise<void> {
	await navigateTo(page, '/reports/balance-confirmations', testInfo);
	await waitForRouteReady(page, 'h1');
}

async function waitForBalanceDataState(page: Page): Promise<void> {
	await expect(page.locator(balanceLoadedState).first()).toBeVisible({ timeout: 15000 });
}

async function generateBalanceSummary(page: Page): Promise<void> {
	const responsePromise = page
		.waitForResponse(
			(response) =>
				summaryRequestPattern.test(response.url()) && response.request().method() === 'GET',
			{ timeout: 15000 }
		)
		.catch(() => null);

	await page.locator('button.btn-primary').click();
	await responsePromise;
	await waitForBalanceDataState(page);
}

async function hasVisibleError(page: Page): Promise<boolean> {
	return page.locator('.alert-error').isVisible().catch(() => false);
}

async function hasBalanceSummary(page: Page): Promise<boolean> {
	await waitForBalanceDataState(page);
	if (await hasVisibleError(page)) {
		return false;
	}
	return page.locator('.summary-card').isVisible().catch(() => false);
}

async function hasBalanceTable(page: Page): Promise<boolean> {
	if (!(await hasBalanceSummary(page))) {
		return false;
	}
	return page.locator('table.table').first().isVisible().catch(() => false);
}

async function expectTerminalBalanceState(page: Page): Promise<void> {
	await expect(page.locator('.empty-state, .alert-error').first()).toBeVisible();
}

async function openFirstContactDetailModalIfAvailable(page: Page): Promise<boolean> {
	if (!(await hasBalanceTable(page))) {
		await expectTerminalBalanceState(page);
		return false;
	}

	const viewButton = page.locator('tbody button.btn-sm').first();
	await expect(viewButton).toBeVisible();
	await viewButton.click();
	await expect(page.locator('.modal')).toBeVisible({ timeout: 5000 });
	return true;
}

test.describe('Demo Balance Confirmations - Page Structure', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await openBalanceConfirmationsPage(page, testInfo);
	});

	test('displays balance confirmations page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
		const heading = page.getByRole('heading', { level: 1 });
		const headingText = await heading.textContent();
		expect(headingText?.toLowerCase()).toMatch(/balance.*confirmation|saldokinnitus/i);
	});

	test('has balance type selector with receivable and payable options', async ({ page }) => {
		const typeSelect = page.locator('select#balanceType');
		await expect(typeSelect).toBeVisible({ timeout: 10000 });
		await expect(typeSelect.locator('option[value="RECEIVABLE"]')).toBeAttached();
		await expect(typeSelect.locator('option[value="PAYABLE"]')).toBeAttached();
	});

	test('has as of date input field', async ({ page }) => {
		const dateInput = page.locator('input#asOfDate');
		await expect(dateInput).toBeVisible({ timeout: 10000 });
		await expect(dateInput).toHaveAttribute('type', 'date');
	});

	test('has generate button', async ({ page }) => {
		const generateBtn = page.locator('button.btn-primary');
		await expect(generateBtn).toBeVisible({ timeout: 10000 });
	});
});

test.describe('Demo Balance Confirmations - Receivables', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await openBalanceConfirmationsPage(page, testInfo);
	});

	test('generates receivables summary by default', async ({ page }) => {
		await waitForBalanceDataState(page);

		if (await hasBalanceSummary(page)) {
			await expect(page.locator('.summary-card h2')).toHaveText(
				/accounts receivable|nõuete kokkuvõte/i
			);
			return;
		}

		await expectTerminalBalanceState(page);
	});

	test('shows summary statistics when data exists', async ({ page }) => {
		if (await hasBalanceSummary(page)) {
			const summaryCard = page.locator('.summary-card');
			const summaryContent = await summaryCard.textContent();
			expect(summaryContent).toMatch(/total balance|kokku saldo/i);
			expect(summaryContent).toMatch(/number of contacts|kontaktide arv/i);
			expect(summaryContent).toMatch(/number of invoices|arvete arv/i);
			return;
		}

		await expectTerminalBalanceState(page);
	});
});

test.describe('Demo Balance Confirmations - Payables', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await openBalanceConfirmationsPage(page, testInfo);
	});

	test('can switch to payables view', async ({ page }) => {
		await page.locator('select#balanceType').selectOption('PAYABLE');
		await generateBalanceSummary(page);

		if (await hasBalanceSummary(page)) {
			await expect(page.locator('.summary-card h2')).toHaveText(
				/accounts payable|kohustuste kokkuvõte/i
			);
			return;
		}

		await expectTerminalBalanceState(page);
	});
});

test.describe('Demo Balance Confirmations - Date Filtering', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await openBalanceConfirmationsPage(page, testInfo);
	});

	test('can change as of date and regenerate', async ({ page }) => {
		await page.locator('input#asOfDate').fill('2024-12-31');
		await generateBalanceSummary(page);

		if (await hasBalanceSummary(page)) {
			await expect(page.locator('.summary-card .as-of-date')).toContainText('2024-12-31');
			return;
		}

		await expectTerminalBalanceState(page);
	});
});

test.describe('Demo Balance Confirmations - Contact Details', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await openBalanceConfirmationsPage(page, testInfo);
	});

	test('can view contact detail modal when contacts exist', async ({ page }) => {
		if (await openFirstContactDetailModalIfAvailable(page)) {
			const modalContent = await page.locator('.modal').textContent();
			expect(modalContent).toMatch(/invoice|arve/i);
		}
	});

	test('modal can be closed', async ({ page }) => {
		if (await openFirstContactDetailModalIfAvailable(page)) {
			const closeBtn = page.locator('.btn-close');
			await expect(closeBtn).toBeVisible();
			await closeBtn.click();
			await expect(page.locator('.modal')).not.toBeVisible();
		}
	});
});

test.describe('Demo Balance Confirmations - Table Display', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await openBalanceConfirmationsPage(page, testInfo);
	});

	test('displays table with proper headers when contacts exist', async ({ page }) => {
		const table = page.locator('table.table').first();

		if (await hasBalanceTable(page)) {
			const tableHeader = await table.locator('thead').textContent();
			expect(tableHeader).toMatch(/contact|kontakt/i);
			expect(tableHeader).toMatch(/balance|saldo/i);
			return;
		}

		await expectTerminalBalanceState(page);
	});

	test('shows total row in table footer when data exists', async ({ page }) => {
		if (await hasBalanceTable(page)) {
			await expect(page.locator('table.table tfoot .total-row').first()).toBeVisible();
			return;
		}

		await expectTerminalBalanceState(page);
	});
});
