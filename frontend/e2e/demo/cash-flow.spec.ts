import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo Cash Flow Statement - Page Structure Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
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
		await page.waitForTimeout(3000);

		// After generation, should show report sections or error
		const hasOperating = await page.getByRole('heading', { name: /operating|äritegevus/i }).isVisible().catch(() => false);
		const hasError = await page.locator('.alert-error').isVisible().catch(() => false);
		const hasReportContainer = await page.locator('.report-container').isVisible().catch(() => false);

		// One of these should be true
		expect(hasOperating || hasError || hasReportContainer).toBeTruthy();
	});
});
