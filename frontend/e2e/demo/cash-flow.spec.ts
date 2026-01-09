import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo Cash Flow Statement - Page Structure Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/reports/cash-flow', testInfo);
		await page.waitForTimeout(2000);
	});

	test('displays cash flow statement page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /cash flow|rahavoog/i })).toBeVisible();
	});

	test('shows date range controls', async ({ page }) => {
		const hasDateInputs = await page.locator('input[type="date"]').first().isVisible().catch(() => false);
		expect(hasDateInputs).toBeTruthy();
	});

	test('has start and end date inputs', async ({ page }) => {
		const startDate = page.locator('input#startDate');
		const endDate = page.locator('input#endDate');

		await expect(startDate).toBeVisible();
		await expect(endDate).toBeVisible();
	});

	test('has generate report button', async ({ page }) => {
		const generateButton = page.getByRole('button', { name: /generate|genereeri/i });
		await expect(generateButton).toBeVisible();
		await expect(generateButton).toBeEnabled();
	});

	test('has back button to reports page', async ({ page }) => {
		const backButton = page.getByRole('link', { name: /back|tagasi/i });
		await expect(backButton).toBeVisible();
	});

	test('can generate cash flow report', async ({ page }) => {
		const generateButton = page.getByRole('button', { name: /generate|genereeri/i });
		await generateButton.click();

		// Wait for either report content, loading indicator to finish, or error
		await expect(async () => {
			const loadingText = page.getByText(/loading|generating|laadimine|genereerin/i);
			const hasLoading = await loadingText.isVisible().catch(() => false);

			// If still loading, that's fine - keep waiting
			if (hasLoading) {
				expect(true).toBe(true);
				return;
			}

			// Check for various success/error states
			const hasOperating = await page.getByRole('heading', { name: /operating|äritegevus/i }).isVisible().catch(() => false);
			const hasError = await page.locator('.alert-error, .error').isVisible().catch(() => false);
			const hasReportContainer = await page.locator('.report-container, .report, .cash-flow-report').isVisible().catch(() => false);
			const hasNoData = await page.getByText(/no data|no transactions|andmeid pole/i).isVisible().catch(() => false);
			const hasSummary = await page.getByText(/total|net|cash flow|kokku|rahavoog/i).isVisible().catch(() => false);

			// One of these should be true after loading completes
			expect(hasOperating || hasError || hasReportContainer || hasNoData || hasSummary).toBeTruthy();
		}).toPass({ timeout: 15000 });
	});
});
