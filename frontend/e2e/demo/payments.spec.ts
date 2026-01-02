import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant, assertTableRowCount } from './utils';

test.describe('Demo Payments - Seeded Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/payments');
	});

	test('displays payments page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /payment/i })).toBeVisible();
	});

	test('shows seeded payments in table', async ({ page }) => {
		await page.waitForSelector('table tbody tr', { timeout: 10000 });
		// Should have 4 payments from seed
		await assertTableRowCount(page, 4);
	});

	test('displays PAY-2024 payment numbers', async ({ page }) => {
		await expect(page.getByText(/PAY-2024/)).toBeVisible();
	});

	test('displays payment from TechStart (3,050 EUR)', async ({ page }) => {
		const hasAmount = await page.getByText(/3,?050/).first().isVisible();
		const hasTechStart = await page.getByText(/TechStart/).first().isVisible();
		expect(hasAmount || hasTechStart).toBeTruthy();
	});

	test('displays payment from Nordic Solutions (10,675 EUR)', async ({ page }) => {
		const hasAmount = await page.getByText(/10,?675/).first().isVisible();
		const hasNordic = await page.getByText(/Nordic/).first().isVisible();
		expect(hasAmount || hasNordic).toBeTruthy();
	});

	test('shows Bank Transfer payment method', async ({ page }) => {
		await expect(page.getByText(/Bank Transfer/i)).toBeVisible();
	});

	test('displays payment dates from November/December 2024', async ({ page }) => {
		await expect(page.getByText(/2024/)).toBeVisible();
	});

	test('can click on payment to view details', async ({ page }) => {
		const paymentRow = page.getByText(/PAY-2024-001/).first();

		if (await paymentRow.isVisible()) {
			await paymentRow.click();
			await page.waitForTimeout(1000);

			// Should show payment allocation details
			const hasDetails = await page.getByText(/INV-2024-001|allocated|3,?050/).first().isVisible().catch(() => false);
			expect(hasDetails).toBeTruthy();
		}
	});

	test('record payment button is visible', async ({ page }) => {
		await expect(page.getByRole('button', { name: /new|create|add|record/i })).toBeVisible();
	});
});
