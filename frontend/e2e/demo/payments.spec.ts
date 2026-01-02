import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Payments - Page Structure Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/payments');
		await page.waitForTimeout(2000);
	});

	test('displays payments page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /payment/i })).toBeVisible();
	});

	test('shows record payment button', async ({ page }) => {
		await expect(page.getByRole('button', { name: /new|create|add|record/i })).toBeVisible();
	});

	test('loads payment data or shows empty state', async ({ page }) => {
		await page.waitForTimeout(5000);
		const hasData = await page.locator('table tbody tr').count() > 0;
		const hasEmptyState = await page.getByText(/no payment|no data|empty|loading/i).isVisible().catch(() => false);
		expect(hasData || hasEmptyState).toBeTruthy();
	});
});
