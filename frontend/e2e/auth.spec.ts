import { test, expect } from '@playwright/test';

test.describe('Authentication', () => {
	// Clear storageState - this test suite tests unauthenticated scenarios
	test.use({ storageState: { cookies: [], origins: [] } });

	test.beforeEach(async ({ page }) => {
		await page.goto('/');
	});

	test('should display login page', async ({ page }) => {
		// Check for login form elements
		await expect(page.getByRole('heading', { name: /login|sign in/i })).toBeVisible();
		await expect(page.getByLabel(/email/i)).toBeVisible();
		await expect(page.getByLabel(/password/i)).toBeVisible();
		await expect(page.getByRole('button', { name: /login|sign in/i })).toBeVisible();
	});

	test('should show error for invalid credentials', async ({ page }) => {
		await page.getByLabel(/email/i).fill('invalid@example.com');
		await page.getByLabel(/password/i).fill('wrongpassword');
		await page.getByRole('button', { name: /login|sign in/i }).click();

		// Should show error message
		await expect(page.getByText(/invalid|error|failed/i)).toBeVisible();
	});

	test('should login with valid credentials', async ({ page }) => {
		// Use test credentials (these should be set up in test environment)
		await page.getByLabel(/email/i).fill('test@example.com');
		await page.getByLabel(/password/i).fill('testpassword123');
		await page.getByRole('button', { name: /login|sign in/i }).click();

		// Should redirect to dashboard or show authenticated content
		await expect(page).toHaveURL(/dashboard|home/i, { timeout: 10000 });
	});

	test('should persist session after page reload', async ({ page }) => {
		// Login first
		await page.getByLabel(/email/i).fill('test@example.com');
		await page.getByLabel(/password/i).fill('testpassword123');
		await page.getByRole('button', { name: /login|sign in/i }).click();
		await expect(page).toHaveURL(/dashboard|home/i, { timeout: 10000 });

		// Reload page
		await page.reload();

		// Should still be on dashboard (session persisted)
		await expect(page).toHaveURL(/dashboard|home/i);
	});

	test('should logout successfully', async ({ page }) => {
		// Login first
		await page.getByLabel(/email/i).fill('test@example.com');
		await page.getByLabel(/password/i).fill('testpassword123');
		await page.getByRole('button', { name: /login|sign in/i }).click();
		await expect(page).toHaveURL(/dashboard|home/i, { timeout: 10000 });

		// Click logout
		await page.getByRole('button', { name: /logout|sign out/i }).click();

		// Should redirect to login page
		await expect(page).toHaveURL(/login|\/$/i);
	});
});
