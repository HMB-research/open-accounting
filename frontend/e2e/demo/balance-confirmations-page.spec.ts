import { test, expect } from '@playwright/test';
import {
	asOfDateInput,
	balanceTypeSelect,
	generateButton,
	setupBalanceConfirmationsPage
} from './balance-confirmations-utils';

test.describe('Demo Balance Confirmations - Page Structure', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupBalanceConfirmationsPage(page, testInfo);
	});

	test('renders balance confirmations controls and result state', async ({ page }) => {
		const heading = page.getByRole('heading', { level: 1 });
		await expect(heading).toBeVisible({ timeout: 10000 });
		await expect(heading).toHaveText(/balance.*confirmation|saldokinnitus/i);

		const typeSelect = balanceTypeSelect(page);
		await expect(typeSelect).toBeVisible({ timeout: 10000 });
		await expect(typeSelect.locator('option[value="RECEIVABLE"]')).toBeAttached();
		await expect(typeSelect.locator('option[value="PAYABLE"]')).toBeAttached();

		const dateInput = asOfDateInput(page);
		await expect(dateInput).toBeVisible({ timeout: 10000 });
		await expect(dateInput).toHaveAttribute('type', 'date');

		await expect(generateButton(page)).toBeVisible({ timeout: 10000 });
		await expect(page.locator('.summary-card, .empty-state, .alert-error').first()).toBeVisible();
	});
});
