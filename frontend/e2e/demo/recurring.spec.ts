import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo Recurring Invoices - Seed Data Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/recurring', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('displays seeded recurring invoices', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify recurring invoice names
		const pageContent = await page.content();
		expect(pageContent.includes('Support') || pageContent.includes('Retainer') || pageContent.includes('License')).toBeTruthy();
	});

	test('shows frequency types', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for frequency types
		const pageContent = await page.content().then(c => c.toLowerCase());
		const hasFrequencies =
			pageContent.includes('monthly') ||
			pageContent.includes('quarterly') ||
			pageContent.includes('yearly');
		expect(hasFrequencies).toBeTruthy();
	});

	test('shows correct recurring invoice count', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have at least 3 recurring invoices
		const rows = page.locator('table tbody tr');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(3);
	});

	test('shows customer names', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for customer names
		const pageContent = await page.content();
		expect(pageContent.includes('TechStart') || pageContent.includes('Nordic') || pageContent.includes('GreenTech')).toBeTruthy();
	});
});
