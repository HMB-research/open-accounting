import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo Invoices - Seed Data Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/invoices', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('displays seeded invoices', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify invoice numbers are visible (format: INV{N}-YYYY-NNN)
		const pageContent = await page.content();
		expect(pageContent).toMatch(/INV\d?-?202[45]-\d{3}/);
	});

	test('shows invoices with various statuses', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for status indicators (PAID, SENT, DRAFT)
		const pageContent = await page.content();
		const hasStatuses =
			pageContent.toLowerCase().includes('paid') ||
			pageContent.toLowerCase().includes('sent') ||
			pageContent.toLowerCase().includes('draft');
		expect(hasStatuses).toBeTruthy();
	});

	test('shows correct invoice count', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have at least 9 invoices
		const rows = page.locator('table tbody tr');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(9);
	});

	test('invoices show customer names', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for seeded customer names
		const hasTechStart = await page.getByText('TechStart').isVisible().catch(() => false);
		const hasNordic = await page.getByText('Nordic').isVisible().catch(() => false);
		expect(hasTechStart || hasNordic).toBeTruthy();
	});
});
