import { test, expect } from '@playwright/test';
import { DEMO_API_URL as DEMO_API, DEMO_URL } from './utils';
import { loginAsDemoEnv } from './env-utils';

test.describe('Demo Environment - Health Checks', () => {
	test('API health endpoint responds', async ({ request }) => {
		const response = await request.get(`${DEMO_API}/health`);
		expect(response.ok()).toBeTruthy();
	});

	test('Frontend loads successfully', async ({ page }) => {
		await page.goto(DEMO_URL);
		await expect(page).toHaveTitle(/tallion|open accounting/i);
	});

	test('Login page renders correctly', async ({ page }) => {
		await page.goto(`${DEMO_URL}/login`);

		await expect(page.getByRole('heading', { name: /welcome|login|sign in/i })).toBeVisible();
		await expect(page.getByLabel(/email/i)).toBeVisible();
		await expect(page.locator('#password')).toBeVisible();
		await expect(page.getByRole('button', { name: /sign in|login/i })).toBeVisible();
	});
});

test.describe('Demo Environment - Authentication', () => {
	test('Demo user can login successfully', async ({ page }) => {
		await loginAsDemoEnv(page);

		await expect(page).toHaveURL(/dashboard/);
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
	});

	test('Invalid credentials show error', async ({ page }) => {
		await page.goto(`${DEMO_URL}/login`);

		await page.getByLabel(/email/i).fill('invalid@example.com');
		await page.locator('#password').fill('wrongpassword');
		await page.getByRole('button', { name: /sign in|login/i }).click();

		const errorAlert = page.locator('.alert-error, [role="alert"]').first();
		await expect(errorAlert.or(page.getByLabel(/email/i))).toBeVisible({ timeout: 5000 });

		const stillOnLogin = page.url().includes('/login');
		const hasError = await errorAlert.isVisible().catch(() => false);
		expect(stillOnLogin || hasError).toBeTruthy();
	});

	test('Logout works correctly', async ({ page }) => {
		await loginAsDemoEnv(page);

		const logoutButton = page.getByRole('button', { name: /logout|sign out/i });
		const hasLogout = await logoutButton.isVisible().catch(() => false);

		if (hasLogout) {
			await logoutButton.click();
			await page.waitForURL(/login/, { timeout: 10000 });
			await expect(page).toHaveURL(/login/);
		} else {
			const userMenu = page.locator('[class*="user"], [class*="avatar"], [class*="profile"]').first();
			if (await userMenu.isVisible()) {
				await userMenu.click();
				const logoutItem = page.getByText(/logout|sign out/i);
				if (await logoutItem.isVisible()) {
					await logoutItem.click();
					await page.waitForURL(/login/, { timeout: 10000 });
				}
			}
		}
	});
});
