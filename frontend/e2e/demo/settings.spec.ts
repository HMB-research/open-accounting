import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant, waitForRouteReady } from './utils';

test.describe('Demo Settings - Page Structure Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/settings', testInfo);
		await waitForRouteReady(page, '.settings-grid');
	});

	test('displays settings page heading or cards', async ({ page }) => {
		// Wait for heading (level 1) to be visible
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
	});

	test('shows settings navigation options', async ({ page }) => {
		const hasCompany = await page.getByText(/company/i).first().isVisible().catch(() => false);
		const hasEmail = await page.getByText(/email/i).first().isVisible().catch(() => false);
		const hasLinks = await page.getByRole('link').count() > 0;
		expect(hasCompany || hasEmail || hasLinks).toBeTruthy();
	});

	test('can navigate to company settings', async ({ page }, testInfo) => {
		await navigateTo(page, '/settings/company', testInfo);
		await waitForRouteReady(page, 'input#companyName, .empty-state', 15000);
		await expect(page.locator('input#companyName')).toBeVisible();
	});
});
