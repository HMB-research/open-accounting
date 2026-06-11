import { test, expect } from '@playwright/test';
import {
	balanceModal,
	openFirstContactDetailModalIfAvailable,
	setupBalanceConfirmationsPage
} from './balance-confirmations-utils';

test.describe('Demo Balance Confirmations - Contact Details', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupBalanceConfirmationsPage(page, testInfo);
	});

	test('opens contact detail modal with invoices and closes it', async ({ page }) => {
		if (!(await openFirstContactDetailModalIfAvailable(page))) {
			return;
		}

		const modal = balanceModal(page);
		const modalContent = await modal.textContent();
		expect(modalContent).toMatch(/invoice|arve/i);

		const closeButton = modal.locator('.btn-close');
		await expect(closeButton).toBeVisible();
		await closeButton.click();
		await expect(modal).not.toBeVisible({ timeout: 5000 });
	});
});
