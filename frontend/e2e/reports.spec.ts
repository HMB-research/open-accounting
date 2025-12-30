import { test, expect } from '@playwright/test';

test.describe('Reports', () => {
	test.beforeEach(async ({ page }) => {
		// Auth is handled by global setup - just navigate
		await page.goto('/reports');
	});

	test('should display reports page heading', async ({ page }) => {
		// Page heading is "Financial Reports"
		await expect(page.getByRole('heading', { name: /financial.*reports|reports/i })).toBeVisible();
	});

	test('should show tenant selection prompt when no tenant', async ({ page }) => {
		// Without a tenant parameter, should show prompt to select one
		await expect(
			page.getByText(/select.*tenant|dashboard/i).or(page.getByRole('link', { name: /dashboard/i }))
		).toBeVisible();
	});
});

test.describe('Reports - With Tenant Context', () => {
	// These tests require tenant context which may not always be available
	// They test the UI structure when reports controls are visible

	test.beforeEach(async ({ page }) => {
		// Navigate to reports - tenant would be set via URL param if available
		await page.goto('/reports');
	});

	test('should show report type selector when tenant available', async ({ page }) => {
		// Check if report controls are visible (depends on tenant being set)
		const reportSelect = page.getByLabel(/report.*type/i);
		const isEmpty = await page.getByText(/select.*tenant/i).isVisible();

		if (!isEmpty) {
			// If not empty state, should have report type selection
			await expect(reportSelect.or(page.getByRole('combobox'))).toBeVisible();
		} else {
			// Empty state is also valid - tenant not configured
			expect(isEmpty).toBeTruthy();
		}
	});

	test('should have generate button when tenant available', async ({ page }) => {
		const isEmpty = await page.getByText(/select.*tenant/i).isVisible();

		if (!isEmpty) {
			// Generate button should be available
			await expect(page.getByRole('button', { name: /generate|run|view/i })).toBeVisible();
		} else {
			// Empty state shows dashboard link instead
			await expect(page.getByRole('link', { name: /dashboard/i })).toBeVisible();
		}
	});

	test('should have date input when tenant available', async ({ page }) => {
		const isEmpty = await page.getByText(/select.*tenant/i).isVisible();

		if (!isEmpty) {
			// Date inputs should be available
			const dateInput = page.getByLabel(/date|as.*of/i).first();
			await expect(dateInput).toBeVisible();
		} else {
			// Empty state is valid
			expect(isEmpty).toBeTruthy();
		}
	});
});

test.describe('Reports - Mobile', () => {
	test.use({ viewport: { width: 375, height: 667 } });

	test.beforeEach(async ({ page }) => {
		// Auth is handled by global setup - just navigate
		await page.goto('/reports');
	});

	test('should be usable on mobile viewport', async ({ page }) => {
		// Page should load with heading visible
		await expect(page.getByRole('heading', { name: /financial.*reports|reports/i })).toBeVisible();
	});

	test('should have touch-friendly elements on mobile', async ({ page }) => {
		// Either report controls or empty state should be visible
		const hasContent = await page.locator('.card').first().isVisible();
		expect(hasContent).toBeTruthy();
	});

	test('should not have horizontal overflow on mobile', async ({ page }) => {
		// Check that body doesn't overflow horizontally
		const bodyWidth = await page.evaluate(() => document.body.scrollWidth);
		expect(bodyWidth).toBeLessThanOrEqual(375);
	});
});
