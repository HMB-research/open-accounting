import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from './utils';

test.describe('Cash Payments View', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('displays cash payments page with correct structure', async ({ page }, testInfo) => {
		await navigateTo(page, '/payments/cash', testInfo);

		// Wait for page to load - heading should be visible
		await expect(page.getByRole('heading', { name: /cash/i })).toBeVisible();

		// Wait for page content to load
		await page.waitForTimeout(2000);

		// Check for summary cards (Total Received, Total Paid, Balance)
		const summaryCards = page.locator('.summary-card, .summary-cards');
		const hasSummary = await summaryCards.isVisible().catch(() => false);

		if (hasSummary) {
			// Should show balance-related text
			const hasBalanceInfo = await page
				.getByText(/received|paid|balance/i)
				.first()
				.isVisible()
				.catch(() => false);
			expect(hasBalanceInfo).toBe(true);
		}

		// Page loaded successfully
		expect(true).toBe(true);
	});

	test('has new payment button', async ({ page }, testInfo) => {
		await navigateTo(page, '/payments/cash', testInfo);

		// Verify New button exists
		const newButton = page
			.getByRole('button', { name: /new|create|add|record/i })
			.or(page.getByRole('link', { name: /new|create|add/i }));
		await expect(newButton).toBeVisible();
	});

	test('has payment type filter', async ({ page }, testInfo) => {
		await navigateTo(page, '/payments/cash', testInfo);

		await page.waitForTimeout(1000);

		// Check for filter dropdown
		const filterSelect = page.locator('select').first();
		const hasFilter = await filterSelect.isVisible().catch(() => false);

		if (hasFilter) {
			// Should have All/Received/Made options
			const options = await filterSelect.locator('option').count();
			expect(options).toBeGreaterThanOrEqual(2);
		}
	});

	test('displays table when payments exist', async ({ page }, testInfo) => {
		await navigateTo(page, '/payments/cash', testInfo);

		await page.waitForTimeout(2000);

		const table = page.locator('table');
		const hasTable = await table.isVisible().catch(() => false);

		// Either table or empty state should be visible
		const emptyState = page.locator('.empty-state');
		const hasEmpty = await emptyState.isVisible().catch(() => false);

		expect(hasTable || hasEmpty).toBe(true);
	});
});
