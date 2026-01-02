import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Recurring Invoices - Page Structure Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/recurring');
		await page.waitForTimeout(2000);
	});

	test('displays recurring invoices page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /recurring/i })).toBeVisible();
	});

	test('shows create recurring invoice button', async ({ page }) => {
		await expect(page.getByRole('button', { name: /new|create|add/i })).toBeVisible();
	});

	test('loads recurring invoice data or shows empty state', async ({ page }) => {
		await page.waitForTimeout(5000);
		const hasData = await page.locator('table tbody tr').count() > 0;
		const hasEmptyState = await page.getByText(/no recurring|no data|empty|loading/i).isVisible().catch(() => false);
		expect(hasData || hasEmptyState).toBeTruthy();
	});
});
