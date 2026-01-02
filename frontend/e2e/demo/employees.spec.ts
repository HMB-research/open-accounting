import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant, assertTableRowCount } from './utils';

test.describe('Demo Employees - Seeded Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/employees');
	});

	test('displays employees page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /employee/i })).toBeVisible();
	});

	test('shows seeded employees in table', async ({ page }) => {
		await page.waitForSelector('table tbody tr', { timeout: 10000 });
		// Should have at least 4 active employees (Liisa is inactive)
		await assertTableRowCount(page, 4);
	});

	test('displays Maria Tamm - Software Developer', async ({ page }) => {
		await expect(page.getByText('Maria Tamm')).toBeVisible();
		await expect(page.getByText(/Software Developer/i)).toBeVisible();
	});

	test('displays Jaan Kask - Project Manager', async ({ page }) => {
		await expect(page.getByText('Jaan Kask')).toBeVisible();
		await expect(page.getByText(/Project Manager/i)).toBeVisible();
	});

	test('displays Anna Mets - UX Designer', async ({ page }) => {
		await expect(page.getByText('Anna Mets')).toBeVisible();
		await expect(page.getByText(/UX Designer/i)).toBeVisible();
	});

	test('displays Peeter Saar - Senior Developer', async ({ page }) => {
		await expect(page.getByText('Peeter Saar')).toBeVisible();
		await expect(page.getByText(/Senior Developer/i)).toBeVisible();
	});

	test('shows department information', async ({ page }) => {
		// Employees are in Engineering, Management, Design departments
		const hasDepartment = await page.getByText(/Engineering|Management|Design/).first().isVisible();
		expect(hasDepartment).toBeTruthy();
	});

	test('can click on employee to view details', async ({ page }) => {
		const mariaRow = page.getByText('Maria Tamm');
		await mariaRow.click();
		await page.waitForTimeout(1000);

		// Should show employee details (salary: 3500.00 EUR)
		const hasDetails = await page.getByText(/3,?500|maria.tamm@acme.ee|EMP001/).first().isVisible().catch(() => false);
		const hasModal = await page.locator('.modal, [role="dialog"]').isVisible().catch(() => false);

		expect(hasDetails || hasModal).toBeTruthy();
	});

	test('add employee button is visible', async ({ page }) => {
		await expect(page.getByRole('button', { name: /new|create|add/i })).toBeVisible();
	});
});
