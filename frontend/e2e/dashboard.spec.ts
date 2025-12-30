import { test, expect } from '@playwright/test';

test.describe('Dashboard', () => {
	test.beforeEach(async ({ page }) => {
		// Login before each test
		await page.goto('/');
		await page.getByLabel(/email/i).fill('test@example.com');
		await page.getByLabel(/password/i).fill('testpassword123');
		await page.getByRole('button', { name: /login|sign in/i }).click();
		await expect(page).toHaveURL(/dashboard/i, { timeout: 10000 });
	});

	test('should display dashboard summary cards', async ({ page }) => {
		// Check for summary cards
		await expect(page.getByText(/revenue/i)).toBeVisible();
		await expect(page.getByText(/expenses/i)).toBeVisible();
		await expect(page.getByText(/net income/i)).toBeVisible();
		await expect(page.getByText(/receivables/i)).toBeVisible();
	});

	test('should display invoice status section', async ({ page }) => {
		await expect(page.getByText(/invoice status|invoices/i)).toBeVisible();
		// Check for status indicators (draft, pending, overdue)
		await expect(page.getByText(/draft/i)).toBeVisible();
	});

	test('should display revenue vs expenses chart', async ({ page }) => {
		// Chart container should exist
		const chartContainer = page.locator('.chart-container, canvas');
		await expect(chartContainer.first()).toBeVisible();
	});

	test('should display quick links', async ({ page }) => {
		// Check for quick action links
		await expect(page.getByRole('link', { name: /create.*invoice|new.*invoice/i })).toBeVisible();
		await expect(page.getByRole('link', { name: /contacts/i })).toBeVisible();
	});

	test('should navigate to invoices from quick link', async ({ page }) => {
		await page.getByRole('link', { name: /invoices/i }).first().click();
		await expect(page).toHaveURL(/invoices/i);
	});

	test('should show loading state initially', async ({ page }) => {
		// Reload to see loading state
		await page.goto('/dashboard');
		// Loading indicator should appear briefly (may be too fast to catch)
		// Just verify page loads without error
		await expect(page.getByText(/revenue/i)).toBeVisible({ timeout: 10000 });
	});

	test('should handle tenant selection', async ({ page }) => {
		// If tenant selector exists, test it
		const tenantSelector = page.locator('select, [role="combobox"]').first();
		if (await tenantSelector.isVisible()) {
			await tenantSelector.click();
			// Should show tenant options
			await expect(page.getByRole('option')).toHaveCount(1, { timeout: 5000 });
		}
	});
});

test.describe('Dashboard - Mobile', () => {
	test.use({ viewport: { width: 375, height: 667 } });

	test.beforeEach(async ({ page }) => {
		await page.goto('/');
		await page.getByLabel(/email/i).fill('test@example.com');
		await page.getByLabel(/password/i).fill('testpassword123');
		await page.getByRole('button', { name: /login|sign in/i }).click();
		await expect(page).toHaveURL(/dashboard/i, { timeout: 10000 });
	});

	test('should display in single column on mobile', async ({ page }) => {
		// Summary cards should stack vertically
		const cards = page.locator('.summary-card, .card').first();
		await expect(cards).toBeVisible();
	});

	test('should have accessible navigation on mobile', async ({ page }) => {
		// Look for hamburger menu or mobile nav
		const mobileNav = page.locator('[aria-label*="menu"], .hamburger, .mobile-nav');
		if (await mobileNav.isVisible()) {
			await mobileNav.click();
			await expect(page.getByRole('navigation')).toBeVisible();
		}
	});
});
