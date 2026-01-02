import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Banking - Page Structure Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/banking');
		await page.waitForTimeout(2000);
	});

	test('displays banking page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /bank/i })).toBeVisible();
	});

	test('shows import button or bank selector', async ({ page }) => {
		const hasImport = await page.getByRole('button', { name: /import/i }).isVisible().catch(() => false);
		const hasSelector = await page.getByRole('combobox').first().isVisible().catch(() => false);
		expect(hasImport || hasSelector).toBeTruthy();
	});

	test('loads banking data or shows empty state', async ({ page }) => {
		await page.waitForTimeout(5000);
		const hasData = await page.locator('table tbody tr').count() > 0;
		const hasEmptyState = await page.getByText(/no transaction|no data|empty|loading|no bank/i).isVisible().catch(() => false);
		expect(hasData || hasEmptyState).toBeTruthy();
	});
});
