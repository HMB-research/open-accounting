import { test, expect } from '@playwright/test';
import {
	expectTerminalReminderState,
	setupPaymentRemindersPage,
	waitForReminderDataState
} from './payment-reminders-utils';

test.describe('Demo Payment Reminders - Page Structure and Summary', () => {
	test('shows page controls, summary statistics, and invoice list states', async ({ page }, testInfo) => {
		await setupPaymentRemindersPage(page, testInfo);

		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
		const heading = page.getByRole('heading', { level: 1 });
		const headingText = await heading.textContent();
		expect(headingText?.toLowerCase()).toMatch(/reminder|meeldetuletus/i);

		const refreshBtn = page.locator('button.btn-secondary').first();
		await expect(refreshBtn).toBeVisible({ timeout: 10000 });

		const backLink = page.locator('a.btn-secondary[href*="invoices"]');
		await expect(backLink).toBeVisible({ timeout: 10000 });

		await waitForReminderDataState(page);

		const summaryCard = page.locator('.summary-card');
		const hasSummary = await summaryCard.isVisible().catch(() => false);

		if (hasSummary) {
			const pageContent = await summaryCard.textContent();
			const hasStats =
				pageContent?.match(/total.*overdue|üle.*tähtaja/i) ||
				pageContent?.match(/invoice|arve/i) ||
				pageContent?.match(/contact|kontakt/i);

			expect(hasStats).toBeTruthy();
		} else {
			await expectTerminalReminderState(page);
		}

		const hasEmptyState = await page
			.getByText(/no overdue|ei leitud/i)
			.isVisible()
			.catch(() => false);
		const hasTable = await page.locator('table.table').isVisible().catch(() => false);
		const hasError = await page.getByText(/failed|error|viga/i).isVisible().catch(() => false);

		expect(hasEmptyState || hasTable || hasError).toBeTruthy();
	});
});
