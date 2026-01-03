import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Payments - Seed Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/payments');
		await page.waitForLoadState('networkidle');
	});

	test('displays seeded payments', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify payment numbers
		const pageContent = await page.content();
		expect(pageContent).toMatch(/PAY-2024-\d{3}/);
	});

	test('shows payment amounts', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for payment amounts (in EUR format)
		const pageContent = await page.content();
		expect(pageContent).toMatch(/[\d,]+\.\d{2}/);
	});

	test('shows correct payment count', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have at least 4 payments
		const rows = page.locator('table tbody tr');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(4);
	});

	test('shows customer names for payments', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for customer names
		const pageContent = await page.content();
		expect(pageContent.includes('TechStart') || pageContent.includes('Nordic') || pageContent.includes('Baltic')).toBeTruthy();
	});
});
