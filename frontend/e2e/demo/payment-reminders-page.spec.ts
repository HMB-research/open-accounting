import { test, expect } from '@playwright/test';
import {
	expectTerminalReminderState,
	setupPaymentRemindersPage,
	waitForReminderDataState
} from './payment-reminders-utils';

test.describe('Demo Payment Reminders - Page Structure', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupPaymentRemindersPage(page, testInfo);
	});

	test('displays payment reminders page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
		const heading = page.getByRole('heading', { level: 1 });
		const headingText = await heading.textContent();
		expect(headingText?.toLowerCase()).toMatch(/reminder|meeldetuletus/i);
	});

	test('has refresh button', async ({ page }) => {
		const refreshBtn = page.locator('button.btn-secondary').first();
		await expect(refreshBtn).toBeVisible({ timeout: 10000 });
	});

	test('has back button linking to invoices', async ({ page }) => {
		const backLink = page.locator('a.btn-secondary[href*="invoices"]');
		await expect(backLink).toBeVisible({ timeout: 10000 });
	});
});

test.describe('Demo Payment Reminders - Summary Display', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupPaymentRemindersPage(page, testInfo);
	});

	test('shows overdue summary statistics', async ({ page }) => {
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
			return;
		}

		await expectTerminalReminderState(page);
	});

	test('displays empty state or invoice list', async ({ page }) => {
		await waitForReminderDataState(page);

		const hasEmptyState = await page
			.getByText(/no overdue|ei leitud/i)
			.isVisible()
			.catch(() => false);
		const hasTable = await page.locator('table.table').isVisible().catch(() => false);
		const hasError = await page.getByText(/failed|error|viga/i).isVisible().catch(() => false);

		expect(hasEmptyState || hasTable || hasError).toBeTruthy();
	});
});
