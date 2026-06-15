import { test, expect } from '@playwright/test';
import { openFirstSendModalIfAvailable, setupPaymentRemindersPage } from './payment-reminders-utils';

test.describe('Demo Payment Reminders - Modal', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupPaymentRemindersPage(page, testInfo);
	});

	test('opens send modal, edits the message, and closes it', async ({ page }) => {
		if (!(await openFirstSendModalIfAvailable(page))) {
			return;
		}

		const modal = page.locator('.modal');
		const modalContent = await modal.textContent();
		expect(modalContent).toMatch(/send|saada|reminder|meeldetuletus/i);

		const textarea = modal.locator('textarea');
		await expect(textarea).toBeVisible();
		await textarea.fill('Test message');
		await expect(textarea).toHaveValue('Test message');

		const closeBtn = page.locator('.btn-close');
		await expect(closeBtn).toBeVisible();
		await closeBtn.click();
		await expect(modal).not.toBeVisible();
	});
});
