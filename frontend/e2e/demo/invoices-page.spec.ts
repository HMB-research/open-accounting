import { test, expect } from '@playwright/test';
import { newInvoiceButton, setupInvoicesPage } from './invoices-utils';

test.describe('Demo Invoices - Page Structure', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupInvoicesPage(page, testInfo);
	});

	test('renders invoice list controls and terminal data state', async ({ page }) => {
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
		await expect(newInvoiceButton(page)).toBeVisible({ timeout: 10000 });
		await expect(page.locator('table tbody tr, .empty-state').first()).toBeVisible({
			timeout: 15000
		});

		if (await page.locator('table thead').first().isVisible().catch(() => false)) {
			const headers = page.locator('table thead th');
			const count = await headers.count();
			expect(count).toBeGreaterThan(0);
			return;
		}

		await expect(page.locator('.empty-state')).toBeVisible();
	});
});
