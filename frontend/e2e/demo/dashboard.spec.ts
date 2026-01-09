import { test, expect } from '@playwright/test';
import { ensureAuthenticated, ensureDemoTenant, getDemoCredentials } from './utils';

/**
 * Wait for dashboard to finish loading data
 */
async function waitForDashboardLoaded(page: import('@playwright/test').Page) {
	// Wait for loading state to finish
	await expect(async () => {
		// Either no loading text, or dashboard content is visible
		const isLoading = await page.getByText(/^Loading\.\.\.$/i).first().isVisible().catch(() => false);
		const hasContent = await page.locator('.summary-grid, .summary-card, .chart-card').first().isVisible().catch(() => false);
		expect(isLoading === false || hasContent === true).toBeTruthy();
	}).toPass({ timeout: 15000 });
}

test.describe('Demo Dashboard - Seeded Data Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await waitForDashboardLoaded(page);
	});

	test('displays organization selector or dashboard content', async ({ page }, testInfo) => {
		// Find the org selector - tenant selector is a select element
		const tenantSelector = page.locator('.tenant-selector select, select').first();

		// Check if tenant selector exists and has an option selected
		const selectorVisible = await tenantSelector.isVisible().catch(() => false);

		if (selectorVisible) {
			// Verify something is selected (tenant name can vary by test worker)
			const selectedText = await tenantSelector.locator('option:checked').textContent().catch(() => '');
			expect(selectedText?.length, 'Tenant selector should have text').toBeGreaterThan(0);
		} else {
			// Fallback: just verify dashboard content loaded for demo tenant
			const hasContent = await page.locator('.summary-grid, .summary-card, h1').first().isVisible().catch(() => false);
			expect(hasContent, 'Dashboard should show content').toBeTruthy();
		}
	});

	test('shows Cash Flow card on dashboard', async ({ page }) => {
		// Wait for dashboard analytics to load
		await expect(async () => {
			const hasCashFlow = await page.getByText(/Cash Flow|rahavoog/i).first().isVisible().catch(() => false);
			expect(hasCashFlow).toBeTruthy();
		}).toPass({ timeout: 15000 });
	});

	test('shows Recent Activity section', async ({ page }) => {
		// Wait for activity section to load
		await expect(async () => {
			const hasActivity = await page.getByText(/Recent Activity|viimased tegevused/i).first().isVisible().catch(() => false);
			expect(hasActivity).toBeTruthy();
		}).toPass({ timeout: 15000 });
	});

	test('shows Revenue vs Expenses chart', async ({ page }) => {
		// Wait for chart section to load
		await expect(async () => {
			const hasChart = await page.getByText(/Revenue vs Expenses|Tulud vs Kulud/i).first().isVisible().catch(() => false);
			expect(hasChart).toBeTruthy();
		}).toPass({ timeout: 15000 });
	});

	test('shows New Organization button', async ({ page }) => {
		// Dashboard has + New Organization button (handles i18n)
		await expect(
			page.getByRole('button', { name: /New Organization|uus organisatsioon|\+/i }).first()
		).toBeVisible({ timeout: 10000 });
	});

	test('navigation header is visible with main menu items', async ({ page, browserName }) => {
		// Skip on mobile - navigation is hidden behind hamburger menu
		const viewportWidth = page.viewportSize()?.width || 1280;
		test.skip(viewportWidth < 768, 'Mobile navigation is collapsed');

		// Wait for navigation to be fully rendered
		const nav = page.getByRole('navigation');
		await expect(nav).toBeVisible({ timeout: 15000 });

		// Check for navigation links with increased timeout for slower CI environments
		// Use polling to handle slow rendering
		await expect(async () => {
			const links = nav.getByRole('link');
			const count = await links.count();
			expect(count).toBeGreaterThan(0);
		}).toPass({ timeout: 15000 });
	});
});
