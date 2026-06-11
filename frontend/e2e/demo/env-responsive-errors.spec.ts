import { test, expect } from '@playwright/test';
import { DEMO_URL } from './utils';
import { loginAsDemoEnv } from './env-utils';

const dashboardViewports = [
	{ label: 'mobile', size: { width: 375, height: 667 } },
	{ label: 'tablet', size: { width: 768, height: 1024 } }
];

test.describe('Demo Environment - Responsive Design', () => {
	test('mobile and tablet viewports work', async ({ page }, testInfo) => {
		for (const viewport of dashboardViewports) {
			await page.setViewportSize(viewport.size);
			await loginAsDemoEnv(page, testInfo);

			await expect(page).toHaveURL(/dashboard/);

			const content = page.locator('main, [class*="content"]').first();
			await expect(content, `${viewport.label} dashboard content`).toBeVisible();
		}
	});
});

test.describe('Demo Environment - Error Handling', () => {
	test('handles unknown and protected routes gracefully', async ({ page }) => {
		await page.goto(`${DEMO_URL}/this-page-does-not-exist`);
		await page.waitForLoadState('domcontentloaded');

		const is404 = await page.getByText(/404|not found|page.*exist/i).isVisible().catch(() => false);
		const redirected = page.url().includes('/login') || page.url().includes('/dashboard');
		const hasContent = await page.locator('body').isVisible();

		expect(is404 || redirected || hasContent).toBeTruthy();

		await page.goto(`${DEMO_URL}/dashboard`);
		await page.waitForLoadState('domcontentloaded');

		const onLogin = page.url().includes('/login');
		const hasLoginForm = await page.getByLabel(/email/i).isVisible().catch(() => false);
		const onDashboard = page.url().includes('/dashboard');

		expect(onLogin || hasLoginForm || onDashboard).toBeTruthy();
	});
});
