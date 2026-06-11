import { test, expect } from '@playwright/test';
import {
	balanceTable,
	asOfDateInput,
	detailExportButton,
	expectTerminalBalanceState,
	expectExportResponse,
	generateBalanceSummary,
	hasBalanceSummary,
	hasBalanceTable,
	openContactDetailModal,
	setupBalanceConfirmationsPage,
	summaryExportButton,
	summaryCard
} from './balance-confirmations-utils';

test.describe('Demo Balance Confirmations - Receivables Summary', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupBalanceConfirmationsPage(page, testInfo);
	});

	test('shows receivable summary statistics, table headers, and totals when data exists', async ({
		page
	}) => {
		await asOfDateInput(page).fill('2024-12-31');
		const response = await generateBalanceSummary(page);
		expect(response?.status()).toBe(200);

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
		expect(summaryContent).toContain('18254.00');
		expect(summaryContent).toMatch(/\b3\b/);

		if (await hasBalanceTable(page)) {
			const table = balanceTable(page);
			const tableHeader = await table.locator('thead').textContent();
			expect(tableHeader).toMatch(/contact|kontakt/i);
			expect(tableHeader).toMatch(/balance|saldo/i);
			await expect(table.locator('tbody tr')).toHaveCount(3);
			await expect(table.locator('tbody tr').filter({ hasText: 'Nordic Solutions AS' })).toContainText(
				'7640.00'
			);
			await expect(table).not.toContainText('Baltic Commerce');
			await expect(page.locator('table.table tfoot .total-row').first()).toBeVisible();
			await expect(page.locator('table.table tfoot .total-row').first()).toContainText('18254.00');
			return;
		}

		await expectTerminalBalanceState(page);
	});

	test('downloads receivable summary and contact confirmation exports', async ({ page }) => {
		await asOfDateInput(page).fill('2024-12-31');
		const response = await generateBalanceSummary(page);
		expect(response?.status()).toBe(200);

		await expect(summaryExportButton(page, 'csv')).toBeVisible({ timeout: 10000 });
		await expectExportResponse(page, () => summaryExportButton(page, 'csv').click(), 'csv');

		await openContactDetailModal(page, 'Nordic Solutions AS');
		const modal = page.locator('[role="dialog"], .modal').first();
		await expect(modal).toContainText(/INV[1-4]-2024-006/);
		await expect(modal).toContainText('7640.00');
		await expect(modal).not.toContainText(/INV[1-4]-2024-007/);

		await expectExportResponse(page, () => detailExportButton(page, 'pdf').click(), 'pdf', {
			detail: true
		});
	});
});
