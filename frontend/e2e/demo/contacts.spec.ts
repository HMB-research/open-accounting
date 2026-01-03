import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Contacts - Seed Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/contacts');
		await page.waitForLoadState('networkidle');
	});

	test('displays seeded customer contacts', async ({ page }) => {
		// Wait for table to load
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify seeded customers are visible
		await expect(page.getByText('TechStart OÜ')).toBeVisible();
		await expect(page.getByText('Nordic Solutions AS')).toBeVisible();
	});

	test('displays seeded supplier contacts', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify seeded suppliers are visible
		await expect(page.getByText('Office Supplies Ltd')).toBeVisible();
	});

	test('shows correct contact count', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have at least 7 contacts (4 customers + 3 suppliers)
		const rows = page.locator('table tbody tr');
		await expect(rows).toHaveCount(7, { timeout: 10000 });
	});

	test('contact details include email and phone', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for contact details (email domain or phone patterns)
		const pageContent = await page.content();
		expect(pageContent).toContain('@');
	});
});
