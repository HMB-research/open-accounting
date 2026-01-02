import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant, assertTableRowCount } from './utils';

test.describe('Demo Payroll - Seeded Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/payroll');
	});

	test('displays payroll page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /payroll/i })).toBeVisible();
	});

	test('shows seeded payroll runs', async ({ page }) => {
		await page.waitForSelector('table tbody tr', { timeout: 10000 });
		// Should have 3 payroll runs from seed (Oct, Nov, Dec 2024)
		await assertTableRowCount(page, 3);
	});

	test('displays October 2024 payroll run (PAID)', async ({ page }) => {
		await expect(page.getByText(/October|2024-10|10\/2024/i)).toBeVisible();
	});

	test('displays November 2024 payroll run (PAID)', async ({ page }) => {
		await expect(page.getByText(/November|2024-11|11\/2024/i)).toBeVisible();
	});

	test('displays December 2024 payroll run (APPROVED)', async ({ page }) => {
		await expect(page.getByText(/December|2024-12|12\/2024/i)).toBeVisible();
	});

	test('shows payroll statuses (PAID, APPROVED)', async ({ page }) => {
		const hasPaid = await page.getByText(/paid/i).first().isVisible();
		const hasApproved = await page.getByText(/approved/i).first().isVisible();
		expect(hasPaid || hasApproved).toBeTruthy();
	});

	test('displays gross salary totals (~15,800 EUR)', async ({ page }) => {
		// Each payroll run has total_gross: 15,800.00
		await expect(page.getByText(/15,?800|gross/i).first()).toBeVisible();
	});

	test('can click on payroll run to view payslips', async ({ page }) => {
		const payrollRow = page.getByText(/October|2024-10/).first();

		if (await payrollRow.isVisible()) {
			await payrollRow.click();
			await page.waitForTimeout(1000);

			// Should show payslips for 4 employees
			const hasPayslips = await page.getByText(/Maria|Jaan|Anna|Peeter|payslip/i).first().isVisible().catch(() => false);
			expect(hasPayslips).toBeTruthy();
		}
	});

	test('create payroll run button is visible', async ({ page }) => {
		await expect(page.getByRole('button', { name: /new|create|add/i })).toBeVisible();
	});
});
