import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo Payments - Seed Data Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/payments', testInfo);
		// Wait for loading to complete
		await page.waitForLoadState('networkidle');
		// Wait for table to appear OR empty state
		await page.waitForSelector('table tbody tr, .empty-state, [class*="empty"]', { timeout: 15000 }).catch(() => {});
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
});
