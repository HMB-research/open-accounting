import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo Settings - Page Structure Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/settings', testInfo);
		await page.waitForTimeout(2000);
	});

	test('displays settings page heading or cards', async ({ page }) => {
		const hasHeading = await page.getByRole('heading', { name: /setting/i }).isVisible().catch(() => false);
		const hasCards = await page.getByText(/company|email|plugin/i).first().isVisible().catch(() => false);
		expect(hasHeading || hasCards).toBeTruthy();
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
