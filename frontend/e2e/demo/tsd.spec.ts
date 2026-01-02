import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo TSD Declarations - Page Structure Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/tsd');
		await page.waitForTimeout(2000);
	});

	test('displays TSD page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /tsd|declaration|tax/i })).toBeVisible();
	});

	test('shows export or create button', async ({ page }) => {
		const hasExport = await page.getByRole('button', { name: /export|xml|download/i }).isVisible().catch(() => false);
		const hasCreate = await page.getByRole('button', { name: /new|create|add/i }).isVisible().catch(() => false);
		expect(hasExport || hasCreate).toBeTruthy();
	});

	test('loads TSD data or shows empty state', async ({ page }) => {
		await page.waitForTimeout(5000);
		const hasData = await page.locator('table tbody tr').count() > 0;
		const hasEmptyState = await page.getByText(/no tsd|no declaration|no data|empty|loading/i).isVisible().catch(() => false);
		expect(hasData || hasEmptyState).toBeTruthy();
	});
});
