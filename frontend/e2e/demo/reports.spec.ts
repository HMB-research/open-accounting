import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant, waitForRouteReady } from './utils';

test.describe('Demo Reports - Page Structure Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/reports', testInfo);
		await waitForRouteReady(page, '.report-controls');
	});

	test('displays reports page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /report/i })).toBeVisible();
	});

	test('shows report type selector or buttons', async ({ page }) => {
		await expect(page.locator('select#reportType')).toBeVisible();
		await expect(page.getByRole('button', { name: /generate|genereeri/i })).toBeVisible();
	});

	test('shows date range controls', async ({ page }) => {
		await expect(page.locator('input#asOfDate')).toBeVisible();
	});
});
