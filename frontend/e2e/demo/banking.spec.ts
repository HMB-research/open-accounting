import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant, waitForRouteReady } from './utils';

test.describe('Demo Banking - Seed Data Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/banking', testInfo);
		await waitForRouteReady(page, '#bank-account-selector, table, .empty-state', 15000);
	});

	test('displays bank accounts', async ({ page }) => {
		const accountSelector = page.locator('#bank-account-selector');
		await expect(accountSelector).toBeVisible({ timeout: 10000 });

		// Verify bank account names
		const accountNames = await accountSelector.locator('option').allTextContents();
		expect(accountNames.join(' ')).toMatch(/Main EUR|Savings|Swedbank|SEB/);
	});

	test('shows bank transactions', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have bank transactions
		const rows = page.locator('table tbody tr');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(1);
	});

	test('shows transaction amounts', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for amounts
		const pageContent = await page.content();
		expect(pageContent).toMatch(/[\d,]+\.\d{2}/);
	});

	test('shows transaction statuses', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for status indicators
		const pageContent = await page.content().then(c => c.toLowerCase());
		const hasStatuses =
			pageContent.includes('matched') ||
			pageContent.includes('unmatched') ||
			pageContent.includes('reconciled');
		expect(hasStatuses).toBeTruthy();
	});
});
