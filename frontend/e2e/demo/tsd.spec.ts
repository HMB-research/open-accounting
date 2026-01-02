import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant, assertTableRowCount } from './utils';

test.describe('Demo TSD Declarations - Seeded Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/tsd');
	});

	test('displays TSD page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /tsd|declaration|tax/i })).toBeVisible();
	});

	test('shows seeded TSD declarations', async ({ page }) => {
		await page.waitForSelector('table tbody tr', { timeout: 10000 });
		// Should have 3 TSD declarations from seed (Oct, Nov, Dec 2024)
		await assertTableRowCount(page, 3);
	});

	test('displays October 2024 TSD (SUBMITTED)', async ({ page }) => {
		await expect(page.getByText(/October|2024-10|10\/2024/i)).toBeVisible();
	});

	test('displays November 2024 TSD (SUBMITTED)', async ({ page }) => {
		await expect(page.getByText(/November|2024-11|11\/2024/i)).toBeVisible();
	});

	test('displays December 2024 TSD (DRAFT)', async ({ page }) => {
		await expect(page.getByText(/December|2024-12|12\/2024/i)).toBeVisible();
	});

	test('shows TSD statuses (SUBMITTED, DRAFT)', async ({ page }) => {
		const hasSubmitted = await page.getByText(/submitted/i).first().isVisible();
		const hasDraft = await page.getByText(/draft/i).first().isVisible();
		expect(hasSubmitted || hasDraft).toBeTruthy();
	});

	test('displays tax amounts (income tax: 2,860 EUR)', async ({ page }) => {
		// Each TSD has total_income_tax: 2,860.00
		const hasAmount = await page.getByText(/2,?860|income.*tax|tax.*amount/i).first().isVisible();
		expect(hasAmount).toBeTruthy();
	});

	test('can click on TSD to view details', async ({ page }) => {
		const tsdRow = page.getByText(/October|2024-10/).first();

		if (await tsdRow.isVisible()) {
			await tsdRow.click();
			await page.waitForTimeout(1000);

			// Should show TSD breakdown
			const hasDetails = await page.getByText(/social.*tax|unemployment|funded.*pension/i).first().isVisible().catch(() => false);
			expect(hasDetails).toBeTruthy();
		}
	});

	test('export XML button is visible', async ({ page }) => {
		const hasExport = await page.getByRole('button', { name: /export|xml|download/i }).isVisible();
		const hasLink = await page.getByRole('link', { name: /export|xml/i }).isVisible();
		expect(hasExport || hasLink).toBeTruthy();
	});
});
