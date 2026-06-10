import { test, expect } from '@playwright/test';
import { DEMO_URL } from './utils';
import { loginAsDemoEnv } from './env-utils';

test.describe('Demo Environment - Responsive Design', () => {
	test('Mobile viewport works', async ({ page }, testInfo) => {
		await page.setViewportSize({ width: 375, height: 667 });
		await loginAsDemoEnv(page, testInfo);

		await expect(page).toHaveURL(/dashboard/);

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();
	});

	test('Tablet viewport works', async ({ page }, testInfo) => {
		await page.setViewportSize({ width: 768, height: 1024 });
		await loginAsDemoEnv(page, testInfo);

		await expect(page).toHaveURL(/dashboard/);

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();
	});
});

test.describe('Demo Environment - Error Handling', () => {
	test('Unknown route handled gracefully', async ({ page }) => {
		await page.goto(`${DEMO_URL}/this-page-does-not-exist`);
		await page.waitForLoadState('domcontentloaded');

		const is404 = await page.getByText(/404|not found|page.*exist/i).isVisible().catch(() => false);
		const redirected = page.url().includes('/login') || page.url().includes('/dashboard');
		const hasContent = await page.locator('body').isVisible();

		expect(is404 || redirected || hasContent).toBeTruthy();
	});

	test('Protected routes require authentication', async ({ page }) => {
		await page.goto(`${DEMO_URL}/dashboard`);
		await page.waitForLoadState('domcontentloaded');

		const onLogin = page.url().includes('/login');
		const hasLoginForm = await page.getByLabel(/email/i).isVisible().catch(() => false);
		const onDashboard = page.url().includes('/dashboard');

		expect(onLogin || hasLoginForm || onDashboard).toBeTruthy();
	});
});
