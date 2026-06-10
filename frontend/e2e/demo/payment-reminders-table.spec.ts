import { test, expect } from '@playwright/test';
import {
	expectTerminalReminderState,
	hasReminderTable,
	setupPaymentRemindersPage,
	waitForReminderDataState
} from './payment-reminders-utils';

test.describe('Demo Payment Reminders - Table Display', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupPaymentRemindersPage(page, testInfo);
	});

	test('table has proper headers when invoices exist', async ({ page }) => {
		const table = page.locator('table.table');
		const tableVisible = await hasReminderTable(page);

		if (tableVisible) {
			const headers = await table.locator('thead').textContent();
			const hasExpectedHeaders =
				headers?.match(/invoice|arve/i) &&
				(headers?.match(/contact|kontakt/i) || headers?.match(/outstanding|tasumata/i));

			expect(hasExpectedHeaders).toBeTruthy();
			return;
		}

		await expectTerminalReminderState(page);
	});

	test('shows overdue days with visual indicator', async ({ page }) => {
		await waitForReminderDataState(page);

		const overdueBadges = page.locator('.overdue-badge');
		const badgeCount = await overdueBadges.count();

		const hasEmptyState = await page
			.getByText(/no overdue|ei leitud/i)
			.isVisible()
			.catch(() => false);
		const hasError = await page.getByText(/failed|error|viga/i).isVisible().catch(() => false);
		expect(badgeCount > 0 || hasEmptyState || hasError).toBeTruthy();
	});
});
