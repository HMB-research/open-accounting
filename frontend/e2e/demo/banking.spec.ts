import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Banking - Seeded Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/banking');
	});

	test('displays banking page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /bank/i })).toBeVisible();
	});

	test('shows Main EUR Account (Swedbank)', async ({ page }) => {
		await expect(page.getByText(/Main EUR Account|Swedbank/i)).toBeVisible();
	});

	test('shows Savings Account (SEB)', async ({ page }) => {
		await expect(page.getByText(/Savings Account|SEB/i)).toBeVisible();
	});

	test('displays account balances in EUR', async ({ page }) => {
		// Main account: 72,189.00 EUR, Savings: 10,000.00 EUR
		await expect(page.getByText(/EUR|€/)).toBeVisible();
	});

	test('shows bank account selector or list', async ({ page }) => {
		const hasSelector = await page.locator('select').first().isVisible();
		const hasList = await page.locator('table, .account-list').first().isVisible();
		expect(hasSelector || hasList).toBeTruthy();
	});

	test('displays bank transactions when account selected', async ({ page }) => {
		// Select Main EUR Account if dropdown exists
		const selector = page.locator('select').first();
		if (await selector.isVisible()) {
			const options = await selector.locator('option').all();
			for (const option of options) {
				const text = await option.textContent();
				if (text && /Main EUR|Swedbank/i.test(text)) {
					const value = await option.getAttribute('value');
					if (value) {
						await selector.selectOption(value);
						break;
					}
				}
			}
			await page.waitForTimeout(1000);
		}

		// Should show transactions (8 seeded)
		const hasTransactions = await page.getByText(/INV-2024|RENT|payment/i).first().isVisible();
		expect(hasTransactions).toBeTruthy();
	});

	test('shows transaction statuses (MATCHED, RECONCILED, UNMATCHED)', async ({ page }) => {
		const hasStatus = await page.getByText(/matched|reconciled|unmatched/i).first().isVisible();
		expect(hasStatus).toBeTruthy();
	});

	test('import transactions button is visible', async ({ page }) => {
		const hasImport = await page.getByRole('button', { name: /import/i }).isVisible();
		const hasLink = await page.getByRole('link', { name: /import/i }).isVisible();
		expect(hasImport || hasLink).toBeTruthy();
	});
});
