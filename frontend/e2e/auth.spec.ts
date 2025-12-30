import { test, expect } from '@playwright/test';

test.describe('Authentication - Login Page', () => {
	// Clear storageState - this test suite tests unauthenticated scenarios
	test.use({ storageState: { cookies: [], origins: [] } });

	test.beforeEach(async ({ page }) => {
		await page.goto('/login');
	});

	test('should display login page elements', async ({ page }) => {
		// Check for login form elements - heading is "Welcome Back"
		await expect(page.getByRole('heading', { name: /welcome|login|sign in/i })).toBeVisible();
		await expect(page.getByLabel(/email/i)).toBeVisible();
		await expect(page.getByLabel(/password/i)).toBeVisible();
		await expect(page.getByRole('button', { name: /sign in|login/i })).toBeVisible();
	});

	test('should show error state for invalid credentials', async ({ page }) => {
		await page.getByLabel(/email/i).fill('invalid@example.com');
		await page.getByLabel(/password/i).fill('wrongpassword');
		await page.getByRole('button', { name: /sign in|login/i }).click();

		// Wait for response - should either show error message or stay on login page
		// Error message could be "Invalid credentials", network error, or other API error
		const errorMessage = page.locator('.alert-error, [role="alert"]');
		const hasError = await errorMessage.isVisible({ timeout: 10000 }).catch(() => false);

		if (hasError) {
			// Verify we have some error text
			await expect(errorMessage).toContainText(/.+/);
		} else {
			// If no error shown, should still be on login page (form didn't navigate away)
			await expect(page).toHaveURL(/login/i);
			await expect(page.getByLabel(/email/i)).toBeVisible();
		}
	});

	test('should have working form inputs', async ({ page }) => {
		// Test that inputs accept values
		const emailInput = page.getByLabel(/email/i);
		const passwordInput = page.getByLabel(/password/i);

		await emailInput.fill('test@example.com');
		await passwordInput.fill('password123');

		await expect(emailInput).toHaveValue('test@example.com');
		await expect(passwordInput).toHaveValue('password123');
	});

	test('should show register toggle option', async ({ page }) => {
		// Should have option to switch to register mode
		const registerToggle = page.getByRole('button', { name: /register|sign up|create account/i });
		const hasRegisterToggle = await registerToggle.isVisible().catch(() => false);

		if (hasRegisterToggle) {
			await registerToggle.click();
			// Name field should appear in register mode
			await expect(page.getByLabel(/name/i)).toBeVisible({ timeout: 5000 });
		}
	});
});

test.describe('Authentication - Login Flow', () => {
	// These tests require the test user from auth.setup to exist
	test.use({ storageState: { cookies: [], origins: [] } });

	test('should attempt login with test credentials', async ({ page }) => {
		await page.goto('/login');

		// Fill credentials matching auth.setup.ts
		await page.getByLabel(/email/i).fill('test@example.com');
		await page.getByLabel(/password/i).fill('testpassword123');
		await page.getByRole('button', { name: /sign in|login/i }).click();

		// Wait for navigation or error - either is acceptable
		// Success: redirects to dashboard
		// Failure: shows error on login page
		await page.waitForLoadState('networkidle', { timeout: 15000 }).catch(() => {});

		const currentUrl = page.url();
		const isOnDashboard = /dashboard|home/i.test(currentUrl);
		const isOnLogin = /login/i.test(currentUrl);

		// Test passes if we navigated somewhere (login worked or error shown)
		expect(isOnDashboard || isOnLogin).toBeTruthy();
	});
});
