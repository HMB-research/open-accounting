import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Payroll - Page Structure Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/payroll');
		await page.waitForTimeout(2000);
	});

	test('displays payroll page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /payroll/i })).toBeVisible();
	});

	test('shows create payroll run button', async ({ page }) => {
		await expect(page.getByRole('button', { name: /new|create|add/i })).toBeVisible();
	});

	test('loads payroll data or shows empty state', async ({ page }) => {
		await page.waitForTimeout(5000);
		const hasData = await page.locator('table tbody tr').count() > 0;
		const hasEmptyState = await page.getByText(/no payroll|no data|empty|loading/i).isVisible().catch(() => false);
		expect(hasData || hasEmptyState).toBeTruthy();
	});
});
