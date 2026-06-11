import { test, expect } from '@playwright/test';
import {
	expectTerminalReminderState,
	hasReminderTable,
	setupPaymentRemindersPage
} from './payment-reminders-utils';

test.describe('Demo Payment Reminders - Invoice Selection', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupPaymentRemindersPage(page, testInfo);
	});

	test('supports invoice selection and send actions when invoices are available', async ({ page }) => {
		const tableVisible = await hasReminderTable(page);

		if (!tableVisible) {
			await expectTerminalReminderState(page);
			return;
		}

		const checkboxes = page.locator('tbody input[type="checkbox"]:not([disabled])');
		const checkboxCount = await checkboxes.count();

		if (checkboxCount > 0) {
			await checkboxes.first().click();
			const isChecked = await checkboxes.first().isChecked();
			expect(isChecked).toBeTruthy();
		} else {
			await expect(page.locator('tbody input[type="checkbox"]').first()).toBeVisible();
		}

		const headerCheckbox = page.locator('thead input[type="checkbox"]');
		await expect(headerCheckbox).toBeVisible();

		const sendBtns = page.locator('tbody button.btn-sm');
		await expect(sendBtns.first()).toBeVisible();

		const btnCount = await sendBtns.count();
		const rowCount = await page.locator('tbody tr').count();
		if (rowCount > 0) {
			expect(btnCount).toBe(rowCount);
		}
	});
});
