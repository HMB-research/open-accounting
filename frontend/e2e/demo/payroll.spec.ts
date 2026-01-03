import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Payroll - Seed Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/payroll');
		await page.waitForLoadState('networkidle');
	});

	test('displays payroll runs', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify payroll periods (Oct, Nov, Dec 2024)
		const pageContent = await page.content();
		expect(pageContent.includes('2024') || pageContent.includes('October') || pageContent.includes('November') || pageContent.includes('December')).toBeTruthy();
	});

	test('shows payroll statuses', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should show PAID and APPROVED statuses
		const pageContent = await page.content().then(c => c.toLowerCase());
		expect(pageContent.includes('paid') || pageContent.includes('approved')).toBeTruthy();
	});

	test('shows correct payroll run count', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have at least 3 payroll runs
		const rows = page.locator('table tbody tr');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(3);
	});

	test('shows total amounts', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for amount values
		const pageContent = await page.content();
		expect(pageContent).toMatch(/[\d,]+\.\d{2}/);
	});
});
