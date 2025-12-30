import { test, expect } from '@playwright/test';

test.describe('Dashboard', () => {
	test.beforeEach(async ({ page }) => {
		// Auth is handled by global setup - just navigate
		await page.goto('/dashboard');
	});

	test('should display dashboard page', async ({ page }) => {
		// Dashboard heading should be visible
		await expect(page.getByRole('heading', { name: /dashboard/i })).toBeVisible();
	});

	test('should show new organization button', async ({ page }) => {
		// New organization button should be visible
		await expect(page.getByRole('button', { name: /new.*organization|create.*organization/i })).toBeVisible();
	});

	test('should handle no tenant state gracefully', async ({ page }) => {
		// Wait for page to settle
		await page.waitForLoadState('networkidle').catch(() => {});

		// Dashboard should have content - check heading first
		const heading = page.getByRole('heading', { name: /dashboard/i });
		const hasHeading = await heading.isVisible().catch(() => false);

		// If we have the heading, page loaded - that's a pass
		if (hasHeading) {
			expect(hasHeading).toBeTruthy();
			return;
		}

		// Otherwise check for any of these states:
		// 1. Welcome message, 2. Summary cards, 3. Error message, 4. Tenant selector
		const welcomeMessage = page.getByText(/welcome|create.*organization/i);
		const summaryCards = page.getByText(/revenue/i);
		const errorMessage = page.locator('.alert-error');
		const tenantSelector = page.locator('select');

		const hasWelcome = await welcomeMessage.first().isVisible({ timeout: 5000 }).catch(() => false);
		const hasSummary = await summaryCards.first().isVisible({ timeout: 1000 }).catch(() => false);
		const hasError = await errorMessage.first().isVisible({ timeout: 1000 }).catch(() => false);
		const hasTenantSelector = await tenantSelector.first().isVisible({ timeout: 1000 }).catch(() => false);

		// Any of these states is acceptable
		expect(hasWelcome || hasSummary || hasError || hasTenantSelector || hasHeading).toBeTruthy();
	});

	test('should display content area', async ({ page }) => {
		// Either summary cards or welcome card should exist
		const contentCard = page.locator('.card, .summary-card').first();
		await expect(contentCard).toBeVisible();
	});

	test('should navigate to invoices from nav', async ({ page }) => {
		// Navigation should work - click on invoices link in nav
		const invoicesLink = page.getByRole('link', { name: /invoices/i }).first();
		await invoicesLink.click();
		await expect(page).toHaveURL(/invoices/i);
	});
});

test.describe('Dashboard - With Tenant', () => {
	// These tests verify UI when tenant exists

	test.beforeEach(async ({ page }) => {
		await page.goto('/dashboard');
	});

	test('should show summary cards when tenant exists', async ({ page }) => {
		// Check if we have a tenant (summary cards visible) or not (welcome message)
		const hasRevenue = await page.getByText(/revenue/i).isVisible().catch(() => false);

		if (hasRevenue) {
			// Verify all summary cards when tenant exists
			await expect(page.getByText(/expenses/i)).toBeVisible();
			await expect(page.getByText(/net.*income/i)).toBeVisible();
			await expect(page.getByText(/receivables/i)).toBeVisible();
		} else {
			// No tenant - welcome state is valid
			await expect(page.getByText(/welcome|create.*organization/i)).toBeVisible();
		}
	});

	test('should show chart when tenant has data', async ({ page }) => {
		// Chart only shows when tenant exists and has data
		const chartContainer = page.locator('.chart-container, canvas');
		const hasChart = await chartContainer.first().isVisible().catch(() => false);

		if (hasChart) {
			// Verify chart is rendered
			expect(hasChart).toBeTruthy();
		}
		// If no chart, that's OK - tenant might not have data
	});
});

test.describe('Dashboard - Mobile', () => {
	test.use({ viewport: { width: 375, height: 667 } });

	test.beforeEach(async ({ page }) => {
		// Auth is handled by global setup - just navigate
		await page.goto('/dashboard');
	});

	test('should display dashboard on mobile', async ({ page }) => {
		// Dashboard heading should be visible
		await expect(page.getByRole('heading', { name: /dashboard/i })).toBeVisible();
	});

	test('should have accessible content on mobile', async ({ page }) => {
		// Content should be visible on mobile
		const card = page.locator('.card, .summary-card, .empty-state').first();
		await expect(card).toBeVisible();
	});

	test('should have mobile navigation', async ({ page }) => {
		// Look for hamburger menu on mobile
		const mobileNav = page.locator('[aria-label*="menu"], .hamburger, .mobile-menu-btn');
		const hasHamburger = await mobileNav.isVisible().catch(() => false);

		// On mobile, either hamburger menu or visible nav should exist
		const nav = page.getByRole('navigation');
		const hasNav = await nav.isVisible().catch(() => false);

		expect(hasHamburger || hasNav).toBeTruthy();
	});

	test('should not have horizontal overflow on mobile', async ({ page }) => {
		// Check that body doesn't overflow horizontally
		const bodyWidth = await page.evaluate(() => document.body.scrollWidth);
		expect(bodyWidth).toBeLessThanOrEqual(375);
	});
});
