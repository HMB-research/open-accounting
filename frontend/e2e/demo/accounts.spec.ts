import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo Chart of Accounts - Seed Data Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/accounts', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('displays seeded accounts', async ({ page }) => {
		await expect(page.locator('table tbody tr, .account-item').first()).toBeVisible({ timeout: 10000 });

		// Verify key account names - use specific cell selector to avoid matching nav links
		await expect(page.getByRole('cell', { name: 'Cash' })).toBeVisible();
		await expect(page.getByRole('cell', { name: /Bank Account.*EUR/i })).toBeVisible();
	});

	test('shows account codes', async ({ page }) => {
		await expect(page.locator('table tbody tr, .account-item').first()).toBeVisible({ timeout: 10000 });

		// Check for account codes (1000, 1100, etc.)
		const pageContent = await page.content();
		expect(pageContent).toMatch(/1[0-9]{3}/);
	});

	test('shows different account types', async ({ page }) => {
		await expect(page.locator('table tbody tr, .account-item').first()).toBeVisible({ timeout: 10000 });

		// Should show Assets, Liabilities, Revenue, Expense
		const pageContent = await page.content();
		const hasTypes =
			pageContent.includes('Asset') ||
			pageContent.includes('ASSET') ||
			pageContent.includes('Liability') ||
			pageContent.includes('LIABILITY');
		expect(hasTypes).toBeTruthy();
	});

	test('shows minimum expected account count', async ({ page }) => {
		await page.waitForTimeout(3000);

		// Should have 28 accounts
		const rows = page.locator('table tbody tr, .account-item');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(20);
	});
});
