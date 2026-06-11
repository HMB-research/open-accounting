import { test, expect } from '@playwright/test';
import {
	asOfDateInput,
	balanceTypeSelect,
	expectTerminalBalanceState,
	generateBalanceSummary,
	hasBalanceSummary,
	setupBalanceConfirmationsPage
} from './balance-confirmations-utils';

test.describe('Demo Balance Confirmations - Filters', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupBalanceConfirmationsPage(page, testInfo);
	});

	test('regenerates payables and changed as-of date summaries', async ({ page }) => {
		await balanceTypeSelect(page).selectOption('PAYABLE');
		await generateBalanceSummary(page);

		if (await hasBalanceSummary(page)) {
			await expect(page.locator('.summary-card h2')).toHaveText(
				/accounts payable|kohustuste kokkuvõte/i
			);
		} else {
			await expectTerminalBalanceState(page);
		}

		await asOfDateInput(page).fill('2024-12-31');
		await generateBalanceSummary(page);

		if (await hasBalanceSummary(page)) {
			await expect(page.locator('.summary-card .as-of-date')).toContainText('2024-12-31');
			return;
		}

		await expectTerminalBalanceState(page);
	});
});
