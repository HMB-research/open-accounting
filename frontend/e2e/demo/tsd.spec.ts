import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo TSD Declarations - Seed Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/tsd');
		await page.waitForLoadState('networkidle');
	});

	test('displays seeded TSD declarations', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify TSD periods (2024)
		const pageContent = await page.content();
		expect(pageContent.includes('2024')).toBeTruthy();
	});

	test('shows declaration statuses', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should show SUBMITTED and DRAFT statuses
		const pageContent = await page.content().then(c => c.toLowerCase());
		expect(pageContent.includes('submitted') || pageContent.includes('draft')).toBeTruthy();
	});

	test('shows correct declaration count', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have at least 3 TSD declarations
		const rows = page.locator('table tbody tr');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(3);
	});

	test('shows tax amounts', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for tax amounts
		const pageContent = await page.content();
		expect(pageContent).toMatch(/[\d,]+\.\d{2}/);
	});
});
