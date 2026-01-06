import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo Balance Confirmations - Page Structure', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/reports/balance-confirmations', testInfo);
		await page.waitForLoadState('networkidle');
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

		// Check options exist
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
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/reports/balance-confirmations', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('generates receivables summary by default', async ({ page }) => {
		// Click generate button
		const generateBtn = page.locator('button.btn-primary');
		await generateBtn.click();
		await page.waitForTimeout(1000);

		// Should show summary section or empty state
		const hasSummary = await page.locator('.summary-card').isVisible().catch(() => false);
		const hasEmptyState = await page
			.getByText(/no outstanding|ei leitud/i)
			.isVisible()
			.catch(() => false);

		expect(hasSummary || hasEmptyState).toBeTruthy();
	});

	test('shows summary statistics when data exists', async ({ page }) => {
		// Generate report
		const generateBtn = page.locator('button.btn-primary');
		await generateBtn.click();
		await page.waitForTimeout(1000);

		// Check for summary statistics
		const pageContent = await page.content();
		const hasStatistics =
			pageContent.includes('Total Balance') ||
			pageContent.includes('Kokku saldo') ||
			pageContent.includes('Number of Contacts') ||
			pageContent.includes('Kontaktide arv');

		expect(hasStatistics || pageContent.includes('No outstanding')).toBeTruthy();
	});
});

test.describe('Demo Balance Confirmations - Payables', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/reports/balance-confirmations', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('can switch to payables view', async ({ page }) => {
		// Select payables
		const typeSelect = page.locator('select#balanceType');
		await typeSelect.selectOption('PAYABLE');

		// Generate report
		const generateBtn = page.locator('button.btn-primary');
		await generateBtn.click();
		await page.waitForTimeout(1000);

		// Should show payables summary or empty state
		const pageContent = await page.content();
		const hasPayablesContent =
			pageContent.includes('Accounts Payable') ||
			pageContent.includes('Kohustuste') ||
			pageContent.includes('No outstanding') ||
			pageContent.includes('ei leitud');

		expect(hasPayablesContent).toBeTruthy();
	});
});

test.describe('Demo Balance Confirmations - Date Filtering', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/reports/balance-confirmations', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('can change as of date and regenerate', async ({ page }) => {
		// Set a past date
		const dateInput = page.locator('input#asOfDate');
		await dateInput.fill('2024-12-31');

		// Generate report
		const generateBtn = page.locator('button.btn-primary');
		await generateBtn.click();
		await page.waitForTimeout(1000);

		// Should show the selected date in summary
		const summaryCard = page.locator('.summary-card');
		const hasDate = await summaryCard.isVisible().catch(() => false);
		if (hasDate) {
			const content = await summaryCard.textContent();
			expect(content).toContain('2024-12-31');
		}
	});
});

test.describe('Demo Balance Confirmations - Contact Details', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/reports/balance-confirmations', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('can view contact detail modal when contacts exist', async ({ page }) => {
		// Generate report
		const generateBtn = page.locator('button.btn-primary');
		await generateBtn.click();
		await page.waitForTimeout(1000);

		// Check if any view details buttons exist
		const viewButtons = page.locator('button:has-text("View Details"), button:has-text("Vaata")');
		const buttonCount = await viewButtons.count();

		if (buttonCount > 0) {
			// Click first view details button
			await viewButtons.first().click();
			await page.waitForTimeout(500);

			// Modal should appear
			const modal = page.locator('.modal');
			await expect(modal).toBeVisible({ timeout: 5000 });

			// Modal should have invoice details
			const modalContent = await modal.textContent();
			expect(modalContent).toMatch(/invoice|arve/i);
		}
	});

	test('modal can be closed', async ({ page }) => {
		// Generate report
		const generateBtn = page.locator('button.btn-primary');
		await generateBtn.click();
		await page.waitForTimeout(1000);

		// Check if any view details buttons exist
		const viewButtons = page.locator('button:has-text("View Details"), button:has-text("Vaata")');
		const buttonCount = await viewButtons.count();

		if (buttonCount > 0) {
			// Click first view details button
			await viewButtons.first().click();
			await page.waitForTimeout(500);

			// Close modal
			const closeBtn = page.locator('.btn-close');
			await closeBtn.click();
			await page.waitForTimeout(500);

			// Modal should be hidden
			const modal = page.locator('.modal');
			await expect(modal).not.toBeVisible();
		}
	});
});

test.describe('Demo Balance Confirmations - Table Display', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/reports/balance-confirmations', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('displays table with proper headers when contacts exist', async ({ page }) => {
		// Generate report
		const generateBtn = page.locator('button.btn-primary');
		await generateBtn.click();
		await page.waitForTimeout(1000);

		// Check if table exists
		const table = page.locator('table.table');
		const tableVisible = await table.isVisible().catch(() => false);

		if (tableVisible) {
			// Check for expected column headers
			const tableContent = await table.textContent();
			const hasContactColumn = tableContent?.match(/contact|kontakt/i);
			const hasBalanceColumn = tableContent?.match(/balance|saldo/i);

			expect(hasContactColumn || hasBalanceColumn).toBeTruthy();
		}
	});

	test('shows total row in table footer when data exists', async ({ page }) => {
		// Generate report
		const generateBtn = page.locator('button.btn-primary');
		await generateBtn.click();
		await page.waitForTimeout(1000);

		// Check for total row
		const totalRow = page.locator('.total-row');
		const totalRowVisible = await totalRow.isVisible().catch(() => false);

		// Either total row exists or there's no data
		const emptyState = page.locator('.empty-state');
		const emptyVisible = await emptyState.isVisible().catch(() => false);

		expect(totalRowVisible || emptyVisible).toBeTruthy();
	});
});
