import { test, expect } from '@playwright/test';
import { openNewInvoiceModal, setupInvoicesPage } from './invoices-utils';

test.describe('Demo Invoices - Create Invoice Modal', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupInvoicesPage(page, testInfo);
	});

	test('opens create invoice modal with required fields and closes it', async ({ page }) => {
		const modal = await openNewInvoiceModal(page);

		await expect(modal.locator('#type')).toBeVisible({ timeout: 5000 });
		await expect(modal.locator('#contact')).toBeVisible();
		await expect(modal.locator('#issue-date')).toBeVisible();
		await expect(modal.locator('#due-date')).toBeVisible();
		await expect(modal.locator('.btn-new-contact')).toBeVisible();
		await expect(modal.locator('input, select').first()).toBeVisible();

		await modal.getByRole('button', { name: /cancel|tühista/i }).click();
		await expect(modal).not.toBeVisible({ timeout: 5000 });
	});
});
