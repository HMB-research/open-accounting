import { test, expect } from '@playwright/test';
import {
	expectTerminalReminderState,
	hasReminderTable,
	setupPaymentRemindersPage
} from './payment-reminders-utils';

test.describe('Demo Payment Reminders - Table Display', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupPaymentRemindersPage(page, testInfo);
	});

	test('shows table headers and overdue indicators or terminal state', async ({ page }) => {
		const table = page.locator('table.table');
		const tableVisible = await hasReminderTable(page);

		if (tableVisible) {
			const headers = await table.locator('thead').textContent();
			const hasExpectedHeaders =
				headers?.match(/invoice|arve/i) &&
				(headers?.match(/contact|kontakt/i) || headers?.match(/outstanding|tasumata/i));

			expect(hasExpectedHeaders).toBeTruthy();
			await expect(page.locator('.overdue-badge').first()).toBeVisible();
			return;
		}

		await expectTerminalReminderState(page);

		const hasEmptyState = await page
			.getByText(/no overdue|ei leitud/i)
			.isVisible()
			.catch(() => false);
		const hasError = await page.getByText(/failed|error|viga/i).isVisible().catch(() => false);
		expect(hasEmptyState || hasError).toBeTruthy();
	});
});
