import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant, waitForRouteReady } from './utils';

async function openTaxPage(page: Page, testInfo: TestInfo): Promise<void> {
	await navigateTo(page, '/tax', testInfo);
	await waitForRouteReady(page, 'select#year, select#month, .declarations-list, .empty-state');
}

test.describe('Tax Overview View', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('displays tax page with correct structure', async ({ page }, testInfo) => {
		await openTaxPage(page, testInfo);

		await expect(page.getByRole('heading', { name: /vat|tax|declaration|käibemaks|deklaratsioon/i }).first()).toBeVisible();
	});

	test('has period selector', async ({ page }, testInfo) => {
		await openTaxPage(page, testInfo);

		await expect(page.locator('select#year')).toBeVisible();
		await expect(page.locator('select#month')).toBeVisible();
		await expect(page.locator('select#month option')).toHaveCount(12);
	});

	test('has generate declaration button', async ({ page }, testInfo) => {
		await openTaxPage(page, testInfo);

		const generateButton = page.getByRole('button', { name: /generate|create|new|genereeri|loo/i }).first();
		await expect(generateButton).toBeVisible();
		await expect(generateButton).toBeEnabled();
	});

	test('displays declarations table or empty state', async ({ page }, testInfo) => {
		await openTaxPage(page, testInfo);

		await expect(page.locator('.declarations-list, .empty-state').first()).toBeVisible();
	});

	test('shows VAT amounts when declarations exist', async ({ page }, testInfo) => {
		await openTaxPage(page, testInfo);

		const declarationsList = page.locator('.declarations-list');
		if (await declarationsList.isVisible().catch(() => false)) {
			const firstDeclaration = declarationsList.locator('.declaration-item').first();
			await expect(firstDeclaration).toBeVisible();
			await expect(firstDeclaration).toContainText(/\d{4}-\d{2}/);
			await expect(firstDeclaration).toContainText(/€|EUR|\d+[,.]\d{2}/);
			return;
		}

		await expect(page.locator('.empty-state')).toBeVisible();
	});
});
