import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Invoices - Page Structure Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/invoices');
		await page.waitForTimeout(2000);
	});

	test('displays invoices page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /invoice/i })).toBeVisible();
	});

	test('shows create invoice button', async ({ page }) => {
		await expect(page.getByRole('button', { name: /new|create|add/i })).toBeVisible();
	});

	test('shows status filter or tabs', async ({ page }) => {
		// Invoice page should have status filtering
		const hasFilter = await page.getByRole('combobox').first().isVisible().catch(() => false);
		const hasTabs = await page.getByRole('tab').first().isVisible().catch(() => false);
		const hasStatusText = await page.getByText(/all|draft|sent|paid/i).first().isVisible().catch(() => false);
		expect(hasFilter || hasTabs || hasStatusText).toBeTruthy();
	});

	test('loads invoice data or shows empty state', async ({ page }) => {
		await page.waitForTimeout(5000);
		const hasData = await page.locator('table tbody tr').count() > 0;
		const hasEmptyState = await page.getByText(/no invoice|no data|empty|loading/i).isVisible().catch(() => false);
		expect(hasData || hasEmptyState).toBeTruthy();
	});
});
