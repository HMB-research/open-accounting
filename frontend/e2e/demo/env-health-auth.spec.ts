import { test, expect } from '@playwright/test';
import { DEMO_API_URL as DEMO_API, DEMO_URL } from './utils';
import { loginAsDemoEnv } from './env-utils';

test.describe('Demo Environment - Health Checks', () => {
	test('verifies health, login form, authentication, and logout', async ({ page, request }) => {
		const response = await request.get(`${DEMO_API}/health`);
		expect(response.ok()).toBeTruthy();

		await page.goto(DEMO_URL);
		await expect(page).toHaveTitle(/tallion|open accounting/i);

		await page.goto(`${DEMO_URL}/login`);
		await expect(page.getByRole('heading', { name: /welcome|login|sign in/i })).toBeVisible();
		await expect(page.getByLabel(/email/i)).toBeVisible();
		await expect(page.locator('#password')).toBeVisible();
		await expect(page.getByRole('button', { name: /sign in|login/i })).toBeVisible();

		await page.getByLabel(/email/i).fill('invalid@example.com');
		await page.locator('#password').fill('wrongpassword');
		await page.getByRole('button', { name: /sign in|login/i }).click();

		const errorAlert = page.locator('.alert-error, [role="alert"]').first();
		await expect(errorAlert.or(page.getByLabel(/email/i))).toBeVisible({ timeout: 5000 });

		const stillOnLogin = page.url().includes('/login');
		const hasError = await errorAlert.isVisible().catch(() => false);
		expect(stillOnLogin || hasError).toBeTruthy();

		await loginAsDemoEnv(page);
		await expect(page).toHaveURL(/dashboard/);
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible();

		const logoutButton = page.getByRole('button', { name: /logout|sign out/i }).first();
		await expect(logoutButton).toBeVisible();
		await logoutButton.click();
		await page.waitForURL(/login/, { timeout: 10000 });
		await expect(page).toHaveURL(/login/);
	});
});
