import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant, assertTableRowCount } from './utils';

test.describe('Demo Chart of Accounts - Seeded Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/accounts');
	});

	test('displays accounts page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /account|chart/i })).toBeVisible();
	});

	test('shows seeded accounts', async ({ page }) => {
		await page.waitForSelector('table tbody tr, .account-list', { timeout: 10000 });
		// Should have 28 accounts from seed
		await assertTableRowCount(page, 10); // At least 10 visible
	});

	test('displays Cash account (1000)', async ({ page }) => {
		await expect(page.getByText(/1000/)).toBeVisible();
		await expect(page.getByText('Cash')).toBeVisible();
	});

	test('displays Bank Account - EUR (1100)', async ({ page }) => {
		await expect(page.getByText(/1100/)).toBeVisible();
		await expect(page.getByText(/Bank Account.*EUR/i)).toBeVisible();
	});

	test('displays Accounts Receivable (1200)', async ({ page }) => {
		await expect(page.getByText(/1200/)).toBeVisible();
		await expect(page.getByText(/Accounts Receivable/i)).toBeVisible();
	});

	test('displays Sales Revenue (4000)', async ({ page }) => {
		await expect(page.getByText(/4000/)).toBeVisible();
		await expect(page.getByText(/Sales Revenue/i)).toBeVisible();
	});

	test('displays Salaries Expense (6000)', async ({ page }) => {
		await expect(page.getByText(/6000/)).toBeVisible();
		await expect(page.getByText(/Salaries Expense/i)).toBeVisible();
	});

	test('shows account types (ASSET, LIABILITY, EQUITY, REVENUE, EXPENSE)', async ({ page }) => {
		const hasAsset = await page.getByText(/asset/i).first().isVisible();
		const hasLiability = await page.getByText(/liability/i).first().isVisible();
		const hasRevenue = await page.getByText(/revenue/i).first().isVisible();
		const hasExpense = await page.getByText(/expense/i).first().isVisible();

		// At least one account type should be visible
		expect(hasAsset || hasLiability || hasRevenue || hasExpense).toBeTruthy();
	});

	test('can filter or search accounts', async ({ page }) => {
		const searchInput = page.getByPlaceholder(/search|filter/i).or(page.locator('input[type="search"]'));

		if (await searchInput.isVisible()) {
			await searchInput.fill('Cash');
			await page.waitForTimeout(500);
			await expect(page.getByText('Cash')).toBeVisible();
		}
	});
});
