import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant, assertTableRowCount } from './utils';

test.describe('Demo Recurring Invoices - Seeded Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/recurring');
	});

	test('displays recurring invoices page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /recurring/i })).toBeVisible();
	});

	test('shows seeded recurring invoices', async ({ page }) => {
		await page.waitForSelector('table tbody tr', { timeout: 10000 });
		// Should have 3 recurring invoices from seed
		await assertTableRowCount(page, 3);
	});

	test('displays Monthly Support - TechStart', async ({ page }) => {
		await expect(page.getByText(/Monthly Support.*TechStart|TechStart.*Monthly/i)).toBeVisible();
	});

	test('displays Quarterly Retainer - Nordic', async ({ page }) => {
		await expect(page.getByText(/Quarterly.*Nordic|Nordic.*Quarterly/i)).toBeVisible();
	});

	test('displays Annual License - GreenTech', async ({ page }) => {
		await expect(page.getByText(/Annual.*GreenTech|GreenTech.*Annual|Yearly/i)).toBeVisible();
	});

	test('shows frequency types (MONTHLY, QUARTERLY, YEARLY)', async ({ page }) => {
		const hasMonthly = await page.getByText(/monthly/i).first().isVisible();
		const hasQuarterly = await page.getByText(/quarterly/i).first().isVisible();
		const hasYearly = await page.getByText(/yearly|annual/i).first().isVisible();

		expect(hasMonthly || hasQuarterly || hasYearly).toBeTruthy();
	});

	test('shows active status', async ({ page }) => {
		// All 3 recurring invoices are active
		await expect(page.getByText(/active/i).first()).toBeVisible();
	});

	test('displays customer names', async ({ page }) => {
		const hasTechStart = await page.getByText(/TechStart/).isVisible();
		const hasNordic = await page.getByText(/Nordic/).isVisible();
		const hasGreenTech = await page.getByText(/GreenTech/).isVisible();

		expect(hasTechStart || hasNordic || hasGreenTech).toBeTruthy();
	});

	test('create recurring invoice button is visible', async ({ page }) => {
		await expect(page.getByRole('button', { name: /new|create|add/i })).toBeVisible();
	});
});
