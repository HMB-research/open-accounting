import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Chart of Accounts - Page Structure Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/accounts');
		await page.waitForTimeout(2000);
	});

	test('displays accounts page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /account|chart/i })).toBeVisible();
	});

	test('loads account data or shows empty state', async ({ page }) => {
		await page.waitForTimeout(5000);
		const hasData = await page.locator('table tbody tr').count() > 0;
		const hasList = await page.locator('.account-list, [class*="account"]').first().isVisible().catch(() => false);
		const hasEmptyState = await page.getByText(/no account|no data|empty|loading/i).isVisible().catch(() => false);
		expect(hasData || hasList || hasEmptyState).toBeTruthy();
	});
});
