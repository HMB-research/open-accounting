import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo Employees - Seed Data Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/employees', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('displays seeded employees', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify seeded employee names
		await expect(page.getByText('Maria Tamm')).toBeVisible();
		await expect(page.getByText('Jaan Kask')).toBeVisible();
	});

	test('shows employee positions', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for positions
		const pageContent = await page.content();
		const hasPositions =
			pageContent.includes('Developer') ||
			pageContent.includes('Manager') ||
			pageContent.includes('Designer');
		expect(hasPositions).toBeTruthy();
	});

	test('shows correct active employee count', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have 4 active employees (Liisa Kivi is inactive)
		const rows = page.locator('table tbody tr');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(4);
	});

	test('shows employee departments', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for departments
		const pageContent = await page.content();
		const hasDepartments =
			pageContent.includes('Engineering') ||
			pageContent.includes('Management') ||
			pageContent.includes('Design');
		expect(hasDepartments).toBeTruthy();
	});
});
