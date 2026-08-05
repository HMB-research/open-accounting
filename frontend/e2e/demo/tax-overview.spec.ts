import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant, waitForRouteReady } from './utils';

const routeLoadTimeout = 30000;

async function openLegacyTaxPage(page: Page, testInfo: TestInfo): Promise<void> {
	await navigateTo(page, '/tax', testInfo, { waitForNetworkIdle: false });
	await expect(page).toHaveURL(/\/vat-returns(?:\?|$)/, { timeout: routeLoadTimeout });
	await waitForRouteReady(page, '.generate-section, .declarations-list, .empty-state', routeLoadTimeout);
}

test.describe('Legacy Tax Route Compatibility', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('redirects the historical tax route to the canonical VAT workflow', async ({ page }, testInfo) => {
		await openLegacyTaxPage(page, testInfo);
		await expect(page.getByRole('heading', { name: /vat|declaration|käibemaks|deklaratsioon/i }).first()).toBeVisible();
		await expect(page.locator('.generate-section')).toBeVisible();
	});
});
