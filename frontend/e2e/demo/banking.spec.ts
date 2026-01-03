import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Banking - Seed Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/banking');
		await page.waitForLoadState('networkidle');
	});

	test('displays bank accounts', async ({ page }) => {
		await page.waitForTimeout(3000);

		// Verify bank account names
		const pageContent = await page.content();
		expect(pageContent.includes('Main EUR') || pageContent.includes('Savings') || pageContent.includes('Swedbank') || pageContent.includes('SEB')).toBeTruthy();
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
