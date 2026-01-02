import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Reports - Seeded Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/reports');
	});

	test('displays reports page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /report/i })).toBeVisible();
	});

	test('shows report type selector', async ({ page }) => {
		const hasSelector = await page.locator('select').first().isVisible();
		const hasButtons = await page.getByRole('button', { name: /trial|balance|income/i }).first().isVisible();
		expect(hasSelector || hasButtons).toBeTruthy();
	});

	test('can generate Trial Balance report', async ({ page }) => {
		// Select Trial Balance if dropdown
		const selector = page.locator('select').first();
		if (await selector.isVisible()) {
			const options = await selector.locator('option').all();
			for (const option of options) {
				const text = await option.textContent();
				if (text && /trial balance/i.test(text)) {
					const value = await option.getAttribute('value');
					if (value) {
						await selector.selectOption(value);
						break;
					}
				}
			}
		} else {
			await page.getByRole('button', { name: /trial balance/i }).click();
		}

		await page.waitForTimeout(1000);

		// Generate report
		const generateBtn = page.getByRole('button', { name: /generate|view|run/i });
		if (await generateBtn.isVisible()) {
			await generateBtn.click();
			await page.waitForTimeout(2000);
		}

		// Should show accounts from seeded data
		const hasAccounts = await page.getByText(/Cash|Bank|Receivable|Revenue|Expense/i).first().isVisible();
		expect(hasAccounts).toBeTruthy();
	});

	test('can generate Balance Sheet report', async ({ page }) => {
		const selector = page.locator('select').first();
		if (await selector.isVisible()) {
			const options = await selector.locator('option').all();
			for (const option of options) {
				const text = await option.textContent();
				if (text && /balance sheet/i.test(text)) {
					const value = await option.getAttribute('value');
					if (value) {
						await selector.selectOption(value);
						break;
					}
				}
			}
		}

		await page.waitForTimeout(1000);

		const generateBtn = page.getByRole('button', { name: /generate|view|run/i });
		if (await generateBtn.isVisible()) {
			await generateBtn.click();
			await page.waitForTimeout(2000);
		}

		// Should show Asset, Liability, Equity sections
		const hasAssets = await page.getByText(/asset/i).first().isVisible();
		expect(hasAssets).toBeTruthy();
	});

	test('can generate Income Statement report', async ({ page }) => {
		const selector = page.locator('select').first();
		if (await selector.isVisible()) {
			const options = await selector.locator('option').all();
			for (const option of options) {
				const text = await option.textContent();
				if (text && /income statement|profit.*loss/i.test(text)) {
					const value = await option.getAttribute('value');
					if (value) {
						await selector.selectOption(value);
						break;
					}
				}
			}
		}

		await page.waitForTimeout(1000);

		const generateBtn = page.getByRole('button', { name: /generate|view|run/i });
		if (await generateBtn.isVisible()) {
			await generateBtn.click();
			await page.waitForTimeout(2000);
		}

		// Should show Revenue, Expense sections
		const hasRevenue = await page.getByText(/revenue|income/i).first().isVisible();
		expect(hasRevenue).toBeTruthy();
	});

	test('displays date range selector', async ({ page }) => {
		const hasDateInputs = await page.locator('input[type="date"]').first().isVisible();
		const hasDatePicker = await page.getByText(/from|to|period|date/i).first().isVisible();
		expect(hasDateInputs || hasDatePicker).toBeTruthy();
	});

	test('export button is visible', async ({ page }) => {
		const hasExport = await page.getByRole('button', { name: /export|download|pdf|excel/i }).isVisible();
		expect(hasExport).toBeTruthy();
	});
});
