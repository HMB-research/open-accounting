import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from './utils';

/**
 * Wait for salary calculator page to be ready
 */
async function waitForCalculatorReady(page: import('@playwright/test').Page) {
	await expect(async () => {
		const isLoading = await page.getByText(/^Loading\.\.\.$/i).first().isVisible().catch(() => false);
		expect(isLoading).toBe(false);
	}).toPass({ timeout: 15000 });
}

test.describe('Demo Salary Calculator - Page Structure', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/payroll/calculator', testInfo);
		await waitForCalculatorReady(page);
	});

	test('displays salary calculator page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
	});

	test('has gross salary input field', async ({ page }) => {
		await expect(async () => {
			const hasInput = await page.locator('input[type="number"], input#grossSalary').first().isVisible().catch(() => false);
			expect(hasInput).toBeTruthy();
		}).toPass({ timeout: 10000 });
	});

	test('has pension rate dropdown', async ({ page }) => {
		// Check for pension rate dropdown
		await expect(async () => {
			const hasSelect = await page.locator('select').first().isVisible().catch(() => false);
			expect(hasSelect).toBeTruthy();
		}).toPass({ timeout: 10000 });
	});
});

test.describe('Demo Salary Calculator - Calculations', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/payroll/calculator', testInfo);
		await waitForCalculatorReady(page);
	});

	test('shows calculation sections', async ({ page }) => {
		// Check for calculation result sections
		await expect(async () => {
			const hasContent = await page.locator('.card, .result-breakdown, table').first().isVisible().catch(() => false);
			const hasHeading = await page.getByRole('heading').first().isVisible().catch(() => false);
			expect(hasContent || hasHeading).toBeTruthy();
		}).toPass({ timeout: 10000 });
	});

	test('can enter salary value', async ({ page }) => {
		// Wait for input to be visible
		const grossSalaryInput = page.locator('input[type="number"], input#grossSalary').first();
		await expect(grossSalaryInput).toBeVisible({ timeout: 10000 });

		// Clear and enter a salary
		await grossSalaryInput.fill('3000');

		// Value should be entered
		const value = await grossSalaryInput.inputValue();
		expect(value).toBe('3000');
	});
});
