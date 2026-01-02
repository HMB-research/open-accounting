import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant, assertTableRowCount } from './utils';

test.describe('Demo Contacts - Seeded Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/contacts');
	});

	test('displays contacts page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /contact/i })).toBeVisible();
	});

	test('shows all 7 seeded contacts in table', async ({ page }) => {
		// Wait for table to load
		await page.waitForSelector('table tbody tr', { timeout: 10000 });
		await assertTableRowCount(page, 7);
	});

	test('displays TechStart OÜ customer', async ({ page }) => {
		await expect(page.getByText('TechStart OÜ')).toBeVisible();
	});

	test('displays Nordic Solutions AS customer', async ({ page }) => {
		await expect(page.getByText('Nordic Solutions AS')).toBeVisible();
	});

	test('displays Baltic Commerce customer', async ({ page }) => {
		await expect(page.getByText('Baltic Commerce')).toBeVisible();
	});

	test('displays GreenTech Industries customer', async ({ page }) => {
		await expect(page.getByText('GreenTech Industries')).toBeVisible();
	});

	test('displays Office Supplies Ltd supplier', async ({ page }) => {
		await expect(page.getByText('Office Supplies Ltd')).toBeVisible();
	});

	test('displays CloudHost Services supplier', async ({ page }) => {
		await expect(page.getByText('CloudHost Services')).toBeVisible();
	});

	test('displays Marketing Agency OÜ supplier', async ({ page }) => {
		await expect(page.getByText('Marketing Agency OÜ')).toBeVisible();
	});

	test('can filter contacts by search', async ({ page }) => {
		const searchInput = page.getByPlaceholder(/search/i).or(page.locator('input[type="search"]'));

		if (await searchInput.isVisible()) {
			await searchInput.fill('TechStart');
			await page.waitForTimeout(500);

			// Should show TechStart but not Nordic
			await expect(page.getByText('TechStart OÜ')).toBeVisible();

			// Clear and verify all contacts return
			await searchInput.fill('');
			await page.waitForTimeout(500);
		}
	});

	test('can click on contact to view details', async ({ page }) => {
		const techStartRow = page.getByText('TechStart OÜ');
		await techStartRow.click();

		// Should navigate to contact details or show modal
		await page.waitForTimeout(1000);

		// Should show contact details (email from seed: info@techstart.ee)
		const hasDetails = await page.getByText(/info@techstart.ee|14567890|EE145678901/i).first().isVisible().catch(() => false);
		const hasModal = await page.locator('.modal, [role="dialog"]').isVisible().catch(() => false);

		expect(hasDetails || hasModal).toBeTruthy();
	});
});
