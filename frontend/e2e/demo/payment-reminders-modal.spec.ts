import { test, expect } from '@playwright/test';
import { openFirstSendModalIfAvailable, setupPaymentRemindersPage } from './payment-reminders-utils';

test.describe('Demo Payment Reminders - Modal', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupPaymentRemindersPage(page, testInfo);
	});

	test('opens send modal when clicking send button', async ({ page }) => {
		if (await openFirstSendModalIfAvailable(page)) {
			const modalContent = await page.locator('.modal').textContent();
			expect(modalContent).toMatch(/send|saada|reminder|meeldetuletus/i);
		}
	});

	test('modal can be closed', async ({ page }) => {
		if (await openFirstSendModalIfAvailable(page)) {
			const closeBtn = page.locator('.btn-close');
			await expect(closeBtn).toBeVisible();
			await closeBtn.click();
			await expect(page.locator('.modal')).not.toBeVisible();
		}
	});

	test('modal has custom message field', async ({ page }) => {
		if (await openFirstSendModalIfAvailable(page)) {
			const textarea = page.locator('.modal textarea');
			await expect(textarea).toBeVisible();
			await textarea.fill('Test message');
			await expect(textarea).toHaveValue('Test message');
		}
	});
});
