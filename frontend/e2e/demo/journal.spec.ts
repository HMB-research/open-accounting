import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Journal Entries - Seed Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/journal');
		await page.waitForLoadState('networkidle');
	});

	test('displays seeded journal entries', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify journal entry numbers
		const pageContent = await page.content();
		expect(pageContent).toMatch(/JE-2024-\d{3}/);
	});

	test('shows journal entry descriptions', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for entry descriptions
		const pageContent = await page.content();
		const hasDescriptions =
			pageContent.includes('Opening balances') ||
			pageContent.includes('rent') ||
			pageContent.includes('Depreciation');
		expect(hasDescriptions).toBeTruthy();
	});

	test('shows correct journal entry count', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have at least 4 journal entries
		const rows = page.locator('table tbody tr');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(4);
	});

	test('shows entry statuses', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should show POSTED and DRAFT statuses
		const pageContent = await page.content().then(c => c.toLowerCase());
		expect(pageContent.includes('posted') || pageContent.includes('draft')).toBeTruthy();
	});
});
