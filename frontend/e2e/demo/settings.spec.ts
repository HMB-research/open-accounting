import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo Settings - Page Structure Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/settings', testInfo);
		await page.waitForLoadState('networkidle');
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
		await page.waitForTimeout(2000);
		const hasContent = await page.locator('input').first().isVisible().catch(() => false);
		const hasForm = await page.getByRole('form').isVisible().catch(() => false);
		const hasText = await page.getByText(/name|company|registration/i).first().isVisible().catch(() => false);
		expect(hasContent || hasForm || hasText).toBeTruthy();
	});
});
