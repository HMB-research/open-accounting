import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, getDemoCredentials, ensureDemoTenant, DEMO_API_URL } from './utils';

/**
 * Demo Data Verification Tests
 *
 * These tests verify that demo pages load correctly and show either data or empty state.
 * Tests check for proper page structure and absence of error states.
 *
 * Run with: bun run test:e2e:demo:verify
 */

/**
 * Wait for page to finish loading (no loading indicator visible)
 */
async function waitForPageLoaded(page: import('@playwright/test').Page) {
	await expect(async () => {
		const isLoading = await page.getByText(/^Loading\.\.\.$/i).first().isVisible().catch(() => false);
		expect(isLoading).toBe(false);
	}).toPass({ timeout: 15000 });
}

test.describe('Demo Data Verification - Page Load Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('Dashboard loads and shows content', async ({ page }, testInfo) => {
		const creds = getDemoCredentials(testInfo);

		// Navigate to dashboard with tenant
		await page.goto(`/dashboard?tenant=${creds.tenantId}`);
		await waitForPageLoaded(page);

		// Check for dashboard content (cards, charts, or heading)
		await expect(async () => {
			const hasCards = await page.locator('.summary-card, .card, .chart-card').first().isVisible().catch(() => false);
			const hasHeading = await page.getByRole('heading', { level: 1 }).isVisible().catch(() => false);
			expect(hasCards || hasHeading).toBeTruthy();
		}).toPass({ timeout: 15000 });

		// Check that we're not seeing error states
		const errorAlert = page.locator('.alert-error').first();
		const hasError = await errorAlert.isVisible().catch(() => false);
		expect(hasError, 'Dashboard should not show error alerts').toBeFalsy();
	});

	test('Accounts page loads correctly', async ({ page }, testInfo) => {
		await navigateTo(page, '/accounts', testInfo);
		await waitForPageLoaded(page);

		// Check for accounts page content
		await expect(async () => {
			const hasTable = await page.locator('table').first().isVisible().catch(() => false);
			const hasEmpty = await page.locator('.empty-state').isVisible().catch(() => false);
			const hasHeading = await page.getByRole('heading', { level: 1 }).isVisible().catch(() => false);
			expect(hasTable || hasEmpty || hasHeading).toBeTruthy();
		}).toPass({ timeout: 15000 });
	});

	test('Journal page loads correctly', async ({ page }, testInfo) => {
		await navigateTo(page, '/journal', testInfo);
		await waitForPageLoaded(page);

		// Check for journal page content
		await expect(async () => {
			const hasTable = await page.locator('table').first().isVisible().catch(() => false);
			const hasEmpty = await page.locator('.empty-state').isVisible().catch(() => false);
			const hasHeading = await page.getByRole('heading', { level: 1 }).isVisible().catch(() => false);
			expect(hasTable || hasEmpty || hasHeading).toBeTruthy();
		}).toPass({ timeout: 15000 });
	});

	test('Contacts page loads correctly', async ({ page }, testInfo) => {
		await navigateTo(page, '/contacts', testInfo);
		await waitForPageLoaded(page);

		// Check for contacts page content
		await expect(async () => {
			const hasTable = await page.locator('table').first().isVisible().catch(() => false);
			const hasEmpty = await page.locator('.empty-state').isVisible().catch(() => false);
			const hasHeading = await page.getByRole('heading', { level: 1 }).isVisible().catch(() => false);
			expect(hasTable || hasEmpty || hasHeading).toBeTruthy();
		}).toPass({ timeout: 15000 });
	});

	test('Invoices page loads correctly', async ({ page }, testInfo) => {
		await navigateTo(page, '/invoices', testInfo);
		await waitForPageLoaded(page);

		// Check for invoices page content
		await expect(async () => {
			const hasTable = await page.locator('table').first().isVisible().catch(() => false);
			const hasEmpty = await page.locator('.empty-state').isVisible().catch(() => false);
			const hasHeading = await page.getByRole('heading', { level: 1 }).isVisible().catch(() => false);
			expect(hasTable || hasEmpty || hasHeading).toBeTruthy();
		}).toPass({ timeout: 15000 });
	});

	test('Payments page loads correctly', async ({ page }, testInfo) => {
		await navigateTo(page, '/payments', testInfo);
		await waitForPageLoaded(page);

		// Check for payments page content
		await expect(async () => {
			const hasTable = await page.locator('table').first().isVisible().catch(() => false);
			const hasEmpty = await page.locator('.empty-state').isVisible().catch(() => false);
			const hasHeading = await page.getByRole('heading', { level: 1 }).isVisible().catch(() => false);
			expect(hasTable || hasEmpty || hasHeading).toBeTruthy();
		}).toPass({ timeout: 15000 });
	});

	test('Employees page loads correctly', async ({ page }, testInfo) => {
		await navigateTo(page, '/employees', testInfo);
		await waitForPageLoaded(page);

		// Check for employees page content
		await expect(async () => {
			const hasTable = await page.locator('table').first().isVisible().catch(() => false);
			const hasEmpty = await page.locator('.empty-state').isVisible().catch(() => false);
			const hasHeading = await page.getByRole('heading', { level: 1 }).isVisible().catch(() => false);
			expect(hasTable || hasEmpty || hasHeading).toBeTruthy();
		}).toPass({ timeout: 15000 });
	});

	test('Payroll page loads correctly', async ({ page }, testInfo) => {
		await navigateTo(page, '/payroll', testInfo);
		await waitForPageLoaded(page);

		// Check for payroll page content
		await expect(async () => {
			const hasTable = await page.locator('table').first().isVisible().catch(() => false);
			const hasEmpty = await page.locator('.empty-state').isVisible().catch(() => false);
			const hasHeading = await page.getByRole('heading', { level: 1 }).isVisible().catch(() => false);
			const hasCard = await page.locator('.card').first().isVisible().catch(() => false);
			expect(hasTable || hasEmpty || hasHeading || hasCard).toBeTruthy();
		}).toPass({ timeout: 15000 });
	});

	test('Recurring invoices page loads correctly', async ({ page }, testInfo) => {
		await navigateTo(page, '/recurring', testInfo);
		await waitForPageLoaded(page);

		// Check for recurring page content
		await expect(async () => {
			const hasTable = await page.locator('table').first().isVisible().catch(() => false);
			const hasEmpty = await page.locator('.empty-state').isVisible().catch(() => false);
			const hasHeading = await page.getByRole('heading', { level: 1 }).isVisible().catch(() => false);
			expect(hasTable || hasEmpty || hasHeading).toBeTruthy();
		}).toPass({ timeout: 15000 });
	});

	test('Banking page loads correctly', async ({ page }, testInfo) => {
		await navigateTo(page, '/banking', testInfo);
		await waitForPageLoaded(page);

		// Check for banking page content
		await expect(async () => {
			const hasTable = await page.locator('table').first().isVisible().catch(() => false);
			const hasEmpty = await page.locator('.empty-state').isVisible().catch(() => false);
			const hasHeading = await page.getByRole('heading', { level: 1 }).isVisible().catch(() => false);
			const hasCard = await page.locator('.card').first().isVisible().catch(() => false);
			expect(hasTable || hasEmpty || hasHeading || hasCard).toBeTruthy();
		}).toPass({ timeout: 15000 });
	});

	test('Reports page loads correctly', async ({ page }, testInfo) => {
		await navigateTo(page, '/reports', testInfo);
		await waitForPageLoaded(page);

		// Check for reports page content
		await expect(async () => {
			const hasSelector = await page.locator('select').first().isVisible().catch(() => false);
			const hasButton = await page.getByRole('button').first().isVisible().catch(() => false);
			const hasHeading = await page.getByRole('heading', { level: 1 }).isVisible().catch(() => false);
			expect(hasSelector || hasButton || hasHeading).toBeTruthy();
		}).toPass({ timeout: 15000 });
	});

	test('TSD page loads correctly', async ({ page }, testInfo) => {
		await navigateTo(page, '/tsd', testInfo);
		await waitForPageLoaded(page);

		// Check for TSD page content
		await expect(async () => {
			const hasTable = await page.locator('table').first().isVisible().catch(() => false);
			const hasEmpty = await page.locator('.empty-state').isVisible().catch(() => false);
			const hasHeading = await page.getByRole('heading', { level: 1 }).isVisible().catch(() => false);
			expect(hasTable || hasEmpty || hasHeading).toBeTruthy();
		}).toPass({ timeout: 15000 });
	});
});

test.describe('Demo API Data Verification', () => {
	const DEMO_SECRET = process.env.DEMO_RESET_SECRET;

	test.skip(!DEMO_SECRET, 'DEMO_RESET_SECRET required for API verification');

	test('API reports correct data counts for all entities', async ({}, testInfo) => {
		const userNum = (testInfo.parallelIndex % 3) + 2;
		const response = await fetch(`${DEMO_API_URL}/api/demo/status?user=${userNum}`, {
			headers: { 'X-Demo-Secret': DEMO_SECRET! }
		});

		expect(response.ok, `API status check should succeed for user ${userNum}`).toBeTruthy();

		const status = await response.json();

		// Verify all counts are greater than 0
		expect(status.accounts.count, 'Must have accounts').toBeGreaterThan(0);
		expect(status.contacts.count, 'Must have contacts').toBeGreaterThan(0);
		expect(status.invoices.count, 'Must have invoices').toBeGreaterThan(0);
		expect(status.employees.count, 'Must have employees').toBeGreaterThan(0);
		expect(status.payments.count, 'Must have payments').toBeGreaterThan(0);
		expect(status.journalEntries.count, 'Must have journal entries').toBeGreaterThan(0);
		expect(status.bankAccounts.count, 'Must have bank accounts').toBeGreaterThan(0);
		expect(status.recurringInvoices.count, 'Must have recurring invoices').toBeGreaterThan(0);
		expect(status.payrollRuns.count, 'Must have payroll runs').toBeGreaterThan(0);
		expect(status.tsdDeclarations.count, 'Must have TSD declarations').toBeGreaterThan(0);

		// Log counts for visibility
		console.log(`User ${userNum} data counts:`, {
			accounts: status.accounts.count,
			contacts: status.contacts.count,
			invoices: status.invoices.count,
			employees: status.employees.count,
			payments: status.payments.count,
			journalEntries: status.journalEntries.count,
			bankAccounts: status.bankAccounts.count,
			recurringInvoices: status.recurringInvoices.count,
			payrollRuns: status.payrollRuns.count,
			tsdDeclarations: status.tsdDeclarations.count
		});
	});
});
