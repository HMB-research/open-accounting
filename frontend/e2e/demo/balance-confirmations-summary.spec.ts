import { test, expect } from '@playwright/test';
import {
	balanceTable,
	expectTerminalBalanceState,
	hasBalanceSummary,
	hasBalanceTable,
	setupBalanceConfirmationsPage,
	summaryCard
} from './balance-confirmations-utils';

test.describe('Demo Balance Confirmations - Receivables Summary', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupBalanceConfirmationsPage(page, testInfo);
	});

	test('shows receivable summary statistics, table headers, and totals when data exists', async ({
		page
	}) => {
		if (!(await hasBalanceSummary(page))) {
			await expectTerminalBalanceState(page);
			return;
		}

		await expect(summaryCard(page).locator('h2')).toHaveText(
			/accounts receivable|nõuete kokkuvõte/i
		);

		const summaryContent = await summaryCard(page).textContent();
		expect(summaryContent).toMatch(/total balance|kokku saldo/i);
		expect(summaryContent).toMatch(/number of contacts|kontaktide arv/i);
		expect(summaryContent).toMatch(/number of invoices|arvete arv/i);

		if (await hasBalanceTable(page)) {
			const tableHeader = await balanceTable(page).locator('thead').textContent();
			expect(tableHeader).toMatch(/contact|kontakt/i);
			expect(tableHeader).toMatch(/balance|saldo/i);
			await expect(page.locator('table.table tfoot .total-row').first()).toBeVisible();
			return;
		}

		await expectTerminalBalanceState(page);
	});
});
