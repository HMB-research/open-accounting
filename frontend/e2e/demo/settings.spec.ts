import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Settings - Seeded Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
	});

	test('displays settings page with navigation cards', async ({ page }) => {
		await navigateTo(page, '/settings');

		const hasHeading = await page.getByRole('heading', { name: /setting/i }).isVisible();
		const hasCards = await page.getByText(/company|email|plugin/i).first().isVisible();
		expect(hasHeading || hasCards).toBeTruthy();
	});

	test('company settings shows Acme Corporation data', async ({ page }) => {
		await navigateTo(page, '/settings/company');

		// Should display seeded company info
		await expect(page.getByText(/Acme Corporation/i)).toBeVisible();
	});

	test('company settings shows registration code 12345678', async ({ page }) => {
		await navigateTo(page, '/settings/company');

		await expect(page.getByText(/12345678/)).toBeVisible();
	});

	test('company settings shows VAT number EE123456789', async ({ page }) => {
		await navigateTo(page, '/settings/company');

		await expect(page.getByText(/EE123456789/)).toBeVisible();
	});

	test('company settings shows address', async ({ page }) => {
		await navigateTo(page, '/settings/company');

		await expect(page.getByText(/Viru väljak|Tallinn/i)).toBeVisible();
	});

	test('company settings shows bank details', async ({ page }) => {
		await navigateTo(page, '/settings/company');

		const hasBankDetails = await page.getByText(/Swedbank|EE123456789012345678/i).first().isVisible();
		expect(hasBankDetails).toBeTruthy();
	});

	test('email settings page loads', async ({ page }) => {
		await navigateTo(page, '/settings/email');

		const hasHeading = await page.getByRole('heading', { name: /email/i }).isVisible();
		const hasContent = await page.getByText(/smtp|template|notification/i).first().isVisible();
		expect(hasHeading || hasContent).toBeTruthy();
	});

	test('plugins settings page loads', async ({ page }) => {
		await navigateTo(page, '/settings/plugins');

		const hasHeading = await page.getByRole('heading', { name: /plugin/i }).isVisible();
		const hasContent = await page.getByText(/enable|installed|configure/i).first().isVisible();
		expect(hasHeading || hasContent).toBeTruthy();
	});

	test('can edit company settings', async ({ page }) => {
		await navigateTo(page, '/settings/company');

		// Find an input field and verify it's editable
		const nameInput = page.locator('input').first();
		await expect(nameInput).toBeVisible();

		// Should be able to focus the input
		await nameInput.focus();
		expect(await nameInput.isEditable()).toBeTruthy();
	});

	test('save button is visible on company settings', async ({ page }) => {
		await navigateTo(page, '/settings/company');

		await expect(page.getByRole('button', { name: /save|update/i })).toBeVisible();
	});
});
