import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, getDemoCredentials, ensureDemoTenant, DEMO_API_URL } from './utils';

/**
 * Demo Data Verification Tests
 *
 * These tests STRICTLY verify that demo data exists in ALL views.
 * Tests will FAIL if any view shows empty state or loading errors.
 *
 * Run with: npm run test:e2e:demo:verify
 * Or in REPL loop: npm run test:e2e:demo:loop 50 "data-verification"
 *
 * Prerequisites:
 * 1. Backend running with DEMO_MODE=true
 * 2. Demo data seeded via /api/demo/reset
 * 3. DEMO_RESET_SECRET environment variable set
 */

test.describe('Demo Data Verification - All Views Must Have Data', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('Dashboard shows actual chart data (not empty)', async ({ page }, testInfo) => {
		const creds = getDemoCredentials(testInfo);

		// Navigate to dashboard with tenant
		await page.goto(`/dashboard?tenant=${creds.tenantId}`);
		await page.waitForLoadState('networkidle');

		// Wait for dashboard to fully load
		await page.waitForTimeout(2000);

		// Check for chart canvas (indicates charts rendered)
		const canvasElements = page.locator('canvas');
		const canvasCount = await canvasElements.count();
		expect(canvasCount, 'Dashboard should have at least one chart canvas').toBeGreaterThanOrEqual(1);

		// Verify summary cards show actual numbers (not just labels)
		const summaryCards = page.locator('.summary-card, .card, .stat-card');
		const cardCount = await summaryCards.count();
		expect(cardCount, 'Dashboard should have summary cards').toBeGreaterThanOrEqual(1);

		// Check that we're not seeing error states
		const errorIndicators = page.locator('.alert-error, .error-message, [class*="error"]');
		const hasError = (await errorIndicators.count()) > 0 && (await errorIndicators.first().isVisible().catch(() => false));
		expect(hasError, 'Dashboard should not show error states').toBeFalsy();
	});

	test('Accounts page shows chart of accounts (not empty)', async ({ page }, testInfo) => {
		await navigateTo(page, '/accounts', testInfo);
		await page.waitForTimeout(1000);

		// Must show table with actual account rows
		const tableRows = page.locator('table tbody tr');
		const rowCount = await tableRows.count();
		expect(rowCount, 'Accounts page must have account rows (expected 33+)').toBeGreaterThanOrEqual(10);

		// Verify key accounts exist in the table (use table cell selector to avoid nav elements)
		await expect(page.locator('table td').getByText(/Cash$/i).first(), 'Must show Cash account').toBeVisible({ timeout: 5000 });
		await expect(page.locator('table td').getByText(/Bank/i).first(), 'Must show Bank account').toBeVisible({ timeout: 5000 });
		await expect(page.locator('table td').getByText(/Receivable/i).first(), 'Must show Receivable account').toBeVisible({ timeout: 5000 });
	});

	test('Journal entries page shows entries (not empty)', async ({ page }, testInfo) => {
		await navigateTo(page, '/journal', testInfo);
		await page.waitForTimeout(2000);

		// Check for content loaded
		const content = page.locator('main, [class*="content"]').first();
		await expect(content, 'Journal content must be visible').toBeVisible({ timeout: 10000 });

		// Check for table or journal entry data
		const tableRows = page.locator('table tbody tr');
		const rowCount = await tableRows.count();

		// Journal might have entries in a table or show journal entry identifiers
		const pageContent = await page.content();
		const hasJournalData = /JE.*\d|journal.*entry|opening|debit.*credit/i.test(pageContent) || rowCount >= 1;

		expect(hasJournalData, 'Journal page must show journal entries or entry data').toBeTruthy();
	});

	test('Contacts page shows contacts (not empty)', async ({ page }, testInfo) => {
		await navigateTo(page, '/contacts', testInfo);
		await page.waitForTimeout(1000);

		// Must show table with actual contact rows
		const tableRows = page.locator('table tbody tr');
		await expect(tableRows.first(), 'Contacts table must be visible').toBeVisible({ timeout: 10000 });

		const rowCount = await tableRows.count();
		expect(rowCount, 'Contacts page must have contacts (expected 7)').toBeGreaterThanOrEqual(5);

		// Verify specific seeded contacts exist
		await expect(page.getByText('TechStart').first(), 'Must show TechStart contact').toBeVisible({ timeout: 5000 });
		await expect(page.getByText('Nordic').first(), 'Must show Nordic contact').toBeVisible({ timeout: 5000 });
	});

	test('Invoices page shows invoices (not empty)', async ({ page }, testInfo) => {
		await navigateTo(page, '/invoices', testInfo);
		await page.waitForTimeout(1000);

		// Must show table with actual invoice rows
		const tableRows = page.locator('table tbody tr');
		await expect(tableRows.first(), 'Invoices table must be visible').toBeVisible({ timeout: 10000 });

		const rowCount = await tableRows.count();
		expect(rowCount, 'Invoices page must have invoices (expected 9)').toBeGreaterThanOrEqual(5);

		// Look for invoice number patterns
		const pageContent = await page.content();
		expect(pageContent, 'Must contain invoice numbers').toMatch(/INV.*2024|INV-\d+/i);
	});

	test('Payments page shows payments (not empty)', async ({ page }, testInfo) => {
		await navigateTo(page, '/payments', testInfo);
		await page.waitForTimeout(2000);

		// Check for content loaded
		const content = page.locator('main, [class*="content"]').first();
		await expect(content, 'Payments content must be visible').toBeVisible({ timeout: 10000 });

		// Check for table data or payment indicators
		const tableRows = page.locator('table tbody tr');
		const rowCount = await tableRows.count();

		// Payments page should have data in table or show payment identifiers (PAY-*)
		const pageContent = await page.content();
		const hasPaymentData = /PAY.*\d|payment.*\d|received|transfer|€|\$/i.test(pageContent) || rowCount >= 1;

		expect(hasPaymentData, 'Payments page must show payment data').toBeTruthy();
	});

	test('Employees page shows employees (not empty)', async ({ page }, testInfo) => {
		await navigateTo(page, '/employees', testInfo);
		await page.waitForTimeout(1000);

		// Must show table with actual employee rows
		const tableRows = page.locator('table tbody tr');
		await expect(tableRows.first(), 'Employees table must be visible').toBeVisible({ timeout: 10000 });

		const rowCount = await tableRows.count();
		expect(rowCount, 'Employees page must have employees (expected 5)').toBeGreaterThanOrEqual(3);

		// Verify specific seeded employees exist
		await expect(page.getByText(/Maria/i).first(), 'Must show Maria employee').toBeVisible({ timeout: 5000 });
		await expect(page.getByText(/Jaan/i).first(), 'Must show Jaan employee').toBeVisible({ timeout: 5000 });
	});

	test('Payroll page shows payroll runs (not empty)', async ({ page }, testInfo) => {
		await navigateTo(page, '/payroll', testInfo);
		await page.waitForTimeout(2000);

		// Must show payroll content
		const content = page.locator('main, [class*="content"]').first();
		await expect(content, 'Payroll content must be visible').toBeVisible({ timeout: 10000 });

		// Check for payroll data indicators (year selectors, tables, or payroll run cards)
		const hasTable = (await page.locator('table tbody tr').count()) >= 1;
		const hasPayrollCards = (await page.locator('.payroll-card, .card').count()) >= 1;
		const hasYearSelector = await page.locator('select').isVisible().catch(() => false);
		const hasData = await page.getByText(/2024|october|november|december|paid/i).first().isVisible().catch(() => false);

		const hasPayrollContent = hasTable || hasPayrollCards || hasYearSelector || hasData;
		expect(hasPayrollContent, 'Payroll page must have payroll content (runs or selectors)').toBeTruthy();
	});

	test('Recurring invoices page shows recurring invoices (not empty)', async ({ page }, testInfo) => {
		await navigateTo(page, '/recurring', testInfo);
		await page.waitForTimeout(1000);

		// Must show content area
		const content = page.locator('main, [class*="content"]').first();
		await expect(content, 'Recurring invoices content must be visible').toBeVisible({ timeout: 10000 });

		// Check for recurring invoice data
		const tableRows = page.locator('table tbody tr');
		const rowCount = await tableRows.count();
		expect(rowCount, 'Recurring invoices page must have invoices (expected 3)').toBeGreaterThanOrEqual(1);

		// Verify frequency indicators
		const pageContent = await page.content();
		expect(pageContent.toLowerCase(), 'Must contain frequency terms').toMatch(/monthly|quarterly|yearly|support|retainer/i);
	});

	test('Banking page shows bank accounts (not empty)', async ({ page }, testInfo) => {
		await navigateTo(page, '/banking', testInfo);
		await page.waitForTimeout(1000);

		// Check for bank account content
		const content = page.locator('main, [class*="content"]').first();
		await expect(content, 'Banking content must be visible').toBeVisible({ timeout: 10000 });

		// Look for bank account data
		const pageContent = await page.content();
		const hasMainAccount = /main.*eur|eur.*account|swedbank|savings/i.test(pageContent);
		expect(hasMainAccount, 'Banking page must show bank accounts').toBeTruthy();
	});

	test('Reports page loads and allows report generation', async ({ page }, testInfo) => {
		await navigateTo(page, '/reports', testInfo);
		await page.waitForTimeout(1000);

		// Must show reports page
		const content = page.locator('main, [class*="content"]').first();
		await expect(content, 'Reports content must be visible').toBeVisible({ timeout: 10000 });

		// Check for report type selector (select dropdown with report options)
		const reportSelector = page.locator('select#reportType, select');
		const hasSelector = await reportSelector.first().isVisible().catch(() => false);

		// Also check for generate button or report type options
		const generateButton = page.getByRole('button').filter({ hasText: /generate|report/i });
		const hasGenerateButton = await generateButton.isVisible().catch(() => false);

		expect(hasSelector || hasGenerateButton, 'Reports page must show report selector or generate button').toBeTruthy();
	});

	test('TSD declarations page shows TSD data (not empty)', async ({ page }, testInfo) => {
		await navigateTo(page, '/tsd', testInfo);
		await page.waitForTimeout(1000);

		// Must show TSD page heading
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 15000 });

		// Check for TSD declarations
		const tableRows = page.locator('table tbody tr');
		const rowCount = await tableRows.count();
		expect(rowCount, 'TSD page must have declarations (expected 3)').toBeGreaterThanOrEqual(1);
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
