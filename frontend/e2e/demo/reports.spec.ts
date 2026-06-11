import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant, waitForRouteReady } from './utils';

test.describe('Demo Reports - Page Structure Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/reports', testInfo);
		await waitForRouteReady(page, '.report-controls');
	});

	test('displays report heading, selector, action, and date controls', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /report/i })).toBeVisible();
		await expect(page.locator('select#reportType')).toBeVisible();
		await expect(page.getByRole('button', { name: /generate|genereeri/i })).toBeVisible();
		await expect(page.locator('input#asOfDate')).toBeVisible();
	});
});
