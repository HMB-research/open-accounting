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

	test('can select individual invoices when available', async ({ page }) => {
		const tableVisible = await hasReminderTable(page);

		if (tableVisible) {
			const checkboxes = page.locator('tbody input[type="checkbox"]:not([disabled])');
			const checkboxCount = await checkboxes.count();

			if (checkboxCount > 0) {
				await checkboxes.first().click();
				const isChecked = await checkboxes.first().isChecked();
				expect(isChecked).toBeTruthy();
				return;
			}

			await expect(page.locator('tbody input[type="checkbox"]').first()).toBeVisible();
			return;
		}

		await expectTerminalReminderState(page);
	});

	test('has select all functionality', async ({ page }) => {
		const tableVisible = await hasReminderTable(page);

		if (tableVisible) {
			const headerCheckbox = page.locator('thead input[type="checkbox"]');
			await expect(headerCheckbox).toBeVisible();
			return;
		}

		await expectTerminalReminderState(page);
	});
});

test.describe('Demo Payment Reminders - Send Functionality', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupPaymentRemindersPage(page, testInfo);
	});

	test('has send reminders button', async ({ page }) => {
		const tableVisible = await hasReminderTable(page);

		if (tableVisible) {
			await expect(page.locator('tbody button.btn-sm').first()).toBeVisible();
			return;
		}

		await expectTerminalReminderState(page);
	});

	test('individual invoice has send button when email available', async ({ page }) => {
		const tableVisible = await hasReminderTable(page);

		if (tableVisible) {
			const sendBtns = page.locator('tbody button.btn-sm');
			const btnCount = await sendBtns.count();

			const rowCount = await page.locator('tbody tr').count();
			if (rowCount > 0) {
				expect(btnCount).toBe(rowCount);
				await expect(sendBtns.first()).toBeVisible();
				return;
			}
		}

		await expectTerminalReminderState(page);
	});
});
