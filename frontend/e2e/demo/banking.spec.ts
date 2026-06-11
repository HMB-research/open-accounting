import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant, waitForRouteReady } from './utils';

test.describe('Demo Banking - Seed Data Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/banking', testInfo);
		await waitForRouteReady(page, '#bank-account-selector, table, .empty-state', 15000);
	});

	test('displays bank accounts, transactions, amounts, and statuses', async ({ page }) => {
		const accountSelector = page.locator('#bank-account-selector');
		await expect(accountSelector).toBeVisible({ timeout: 10000 });

		const accountNames = await accountSelector.locator('option').allTextContents();
		expect(accountNames.join(' ')).toMatch(/Main EUR|Savings|Swedbank|SEB/);

		const rows = page.locator('table tbody tr');
		await expect(rows.first()).toBeVisible({ timeout: 10000 });
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(1);

		const pageContent = await page.content();
		expect(pageContent).toMatch(/[\d,]+\.\d{2}/);

		const lowerPageContent = pageContent.toLowerCase();
		const hasStatuses =
			lowerPageContent.includes('matched') ||
			lowerPageContent.includes('unmatched') ||
			lowerPageContent.includes('reconciled');
		expect(hasStatuses).toBeTruthy();
	});
});
