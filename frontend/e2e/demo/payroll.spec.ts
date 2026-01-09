import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo Payroll - Page Structure Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/payroll', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('displays payroll page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
	});

	test('shows new payroll run button', async ({ page }) => {
		await expect(page.getByRole('button', { name: /new payroll run/i })).toBeVisible({ timeout: 10000 });
	});

	test('has year filter dropdown', async ({ page }) => {
		const yearDropdown = page.getByRole('combobox', { name: /year/i });
		await expect(yearDropdown).toBeVisible({ timeout: 10000 });
		// Should contain 2024 option (where demo data exists)
		await expect(yearDropdown.locator('option[value="2024"]')).toBeAttached();
	});

	test('shows tax rates information', async ({ page }) => {
		// Check for tax rates display
		const hasTaxInfo = await page.getByText(/tax.*rate|income.*tax|social.*tax/i).first().isVisible().catch(() => false);
		expect(hasTaxInfo).toBeTruthy();
	});

	test('can filter by 2024 year', async ({ page }) => {
		// Wait for year dropdown (inside main content, not the language selector in nav)
		const yearDropdown = page.locator('main select').first();
		await expect(yearDropdown).toBeVisible({ timeout: 10000 });
		await yearDropdown.selectOption('2024');
		await page.waitForLoadState('networkidle');

		// Page should respond to year change (heading should remain visible)
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
	});
});
