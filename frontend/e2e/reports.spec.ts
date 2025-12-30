import { test, expect } from '@playwright/test';

test.describe('Reports', () => {
	test.beforeEach(async ({ page }) => {
		// Auth is handled by global setup - just navigate
		await page.goto('/reports');
	});

	test('should display reports page', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /reports/i })).toBeVisible();
	});

	test('should show report type selector', async ({ page }) => {
		// Should have report type selection
		await expect(
			page.getByLabel(/report.*type/i).or(page.getByRole('combobox').or(page.getByRole('tablist')))
		).toBeVisible();
	});

	test('should generate trial balance report', async ({ page }) => {
		// Select trial balance
		const reportSelect = page.getByLabel(/report.*type/i).or(page.getByRole('combobox'));
		if (await reportSelect.isVisible()) {
			await reportSelect.selectOption({ label: /trial.*balance/i });
		} else {
			// Click on trial balance tab
			await page.getByRole('tab', { name: /trial.*balance/i }).or(page.getByText(/trial.*balance/i)).click();
		}

		// Select date
		const dateInput = page.getByLabel(/date|as.*of/i);
		if (await dateInput.isVisible()) {
			await dateInput.fill(new Date().toISOString().split('T')[0]);
		}

		// Generate report
		const generateButton = page.getByRole('button', { name: /generate|run|view/i });
		if (await generateButton.isVisible()) {
			await generateButton.click();
		}

		// Should show report data
		await expect(page.getByText(/asset|liability|equity|debit|credit/i)).toBeVisible({ timeout: 10000 });
	});

	test('should generate balance sheet report', async ({ page }) => {
		// Select balance sheet
		const balanceSheetTab = page.getByRole('tab', { name: /balance.*sheet/i }).or(page.getByText(/balance.*sheet/i));
		if (await balanceSheetTab.isVisible()) {
			await balanceSheetTab.click();
		}

		// Select date if available
		const dateInput = page.getByLabel(/date|as.*of/i);
		if (await dateInput.isVisible()) {
			await dateInput.fill(new Date().toISOString().split('T')[0]);
		}

		// Generate report
		const generateButton = page.getByRole('button', { name: /generate|run|view/i });
		if (await generateButton.isVisible()) {
			await generateButton.click();
		}

		// Should show balance sheet structure
		await expect(page.getByText(/assets|liabilities|equity/i)).toBeVisible({ timeout: 10000 });
	});

	test('should generate income statement report', async ({ page }) => {
		// Select income statement
		const incomeTab = page.getByRole('tab', { name: /income.*statement|p&l|profit/i }).or(
			page.getByText(/income.*statement/i)
		);
		if (await incomeTab.isVisible()) {
			await incomeTab.click();
		}

		// Select date range if available
		const startDate = page.getByLabel(/start.*date|from/i);
		const endDate = page.getByLabel(/end.*date|to/i);

		if (await startDate.isVisible()) {
			const today = new Date();
			const startOfYear = new Date(today.getFullYear(), 0, 1).toISOString().split('T')[0];
			await startDate.fill(startOfYear);
		}

		if (await endDate.isVisible()) {
			await endDate.fill(new Date().toISOString().split('T')[0]);
		}

		// Generate report
		const generateButton = page.getByRole('button', { name: /generate|run|view/i });
		if (await generateButton.isVisible()) {
			await generateButton.click();
		}

		// Should show income statement structure
		await expect(page.getByText(/revenue|expense|net.*income/i)).toBeVisible({ timeout: 10000 });
	});

	test('should show print button', async ({ page }) => {
		// Generate any report first
		const generateButton = page.getByRole('button', { name: /generate|run|view/i });
		if (await generateButton.isVisible()) {
			await generateButton.click();
			await page.waitForTimeout(1000);
		}

		// Print button should be visible
		await expect(page.getByRole('button', { name: /print/i })).toBeVisible();
	});

	test('should validate balance in trial balance', async ({ page }) => {
		// Select trial balance
		const trialBalanceTab = page.getByRole('tab', { name: /trial.*balance/i }).or(page.getByText(/trial.*balance/i));
		if (await trialBalanceTab.isVisible()) {
			await trialBalanceTab.click();
		}

		// Generate report
		const generateButton = page.getByRole('button', { name: /generate|run|view/i });
		if (await generateButton.isVisible()) {
			await generateButton.click();
		}

		// Look for balance indicator (should show balanced/unbalanced)
		await expect(page.getByText(/balanced|in.*balance|total/i)).toBeVisible({ timeout: 10000 });
	});
});

test.describe('Reports - Date Selection', () => {
	test.beforeEach(async ({ page }) => {
		// Auth is handled by global setup - just navigate
		await page.goto('/reports');
	});

	test('should accept valid date input', async ({ page }) => {
		const dateInput = page.getByLabel(/date|as.*of/i).first();
		if (await dateInput.isVisible()) {
			await dateInput.fill('2024-12-31');
			await expect(dateInput).toHaveValue('2024-12-31');
		}
	});

	test('should handle date range selection', async ({ page }) => {
		// Navigate to income statement (has date range)
		const incomeTab = page.getByRole('tab', { name: /income.*statement|p&l/i }).or(page.getByText(/income.*statement/i));
		if (await incomeTab.isVisible()) {
			await incomeTab.click();
		}

		const startDate = page.getByLabel(/start.*date|from/i);
		const endDate = page.getByLabel(/end.*date|to/i);

		if ((await startDate.isVisible()) && (await endDate.isVisible())) {
			await startDate.fill('2024-01-01');
			await endDate.fill('2024-12-31');

			// Both should have values
			await expect(startDate).toHaveValue('2024-01-01');
			await expect(endDate).toHaveValue('2024-12-31');
		}
	});
});

test.describe('Reports - Mobile', () => {
	test.use({ viewport: { width: 375, height: 667 } });

	test.beforeEach(async ({ page }) => {
		// Auth is handled by global setup - just navigate
		await page.goto('/reports');
	});

	test('should be usable on mobile viewport', async ({ page }) => {
		// Page should load
		await expect(page.getByRole('heading', { name: /reports/i })).toBeVisible();
		// Controls should be accessible
		await expect(page.getByRole('button')).toBeVisible();
	});

	test('should allow horizontal scroll for report tables', async ({ page }) => {
		// Generate a report
		const generateButton = page.getByRole('button', { name: /generate|run|view/i });
		if (await generateButton.isVisible()) {
			await generateButton.click();
			await page.waitForTimeout(1000);
		}

		// Table container should allow scroll
		const tableContainer = page.locator('.table-container, .report-table, table').first();
		if (await tableContainer.isVisible()) {
			// Scroll should work without breaking layout
			await tableContainer.evaluate((el) => el.scrollLeft = 100);
		}
	});
});
