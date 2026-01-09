import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from './utils';

/**
 * Wait for the absences page to finish loading
 */
async function waitForPageLoaded(page: import('@playwright/test').Page) {
	// Wait for "Loading..." text to disappear
	await expect(async () => {
		const isLoading = await page.getByText('Loading...').isVisible().catch(() => false);
		expect(isLoading).toBe(false);
	}).toPass({ timeout: 15000 });
}

test.describe('Demo Leave Management - Page Structure Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/employees/absences', testInfo);
		await waitForPageLoaded(page);
	});

	test('displays leave management page heading', async ({ page }) => {
		// Wait for heading to be visible
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
	});

	test('shows request leave button', async ({ page }) => {
		// Check for the "+" button with request/leave text (handles i18n)
		await expect(
			page.getByRole('button', { name: /request|taotl|\+/i }).first()
		).toBeVisible({ timeout: 10000 });
	});

	test('has year filter dropdown', async ({ page }) => {
		// Use the specific ID for the year filter
		const yearDropdown = page.locator('#yearFilter');
		await expect(yearDropdown).toBeVisible({ timeout: 10000 });

		// Should have current year as an option
		const currentYear = new Date().getFullYear();
		await expect(yearDropdown.locator(`option[value="${currentYear}"]`)).toBeAttached();
	});

	test('has employee filter dropdown', async ({ page }) => {
		// Use the specific ID for the employee filter
		const employeeDropdown = page.locator('#employeeFilter');
		await expect(employeeDropdown).toBeVisible({ timeout: 10000 });

		// Should have "All Employees" option
		await expect(employeeDropdown.locator('option').first()).toBeAttached();
	});

	test('shows tab navigation for records and balances', async ({ page }) => {
		// Check for tabs container
		await expect(page.locator('.tabs')).toBeVisible({ timeout: 10000 });

		// Should have at least 2 tab buttons
		const tabs = page.locator('.tabs .tab, .tabs button');
		await expect(async () => {
			const count = await tabs.count();
			expect(count).toBeGreaterThanOrEqual(2);
		}).toPass({ timeout: 5000 });
	});

	test('can switch between records and balances tabs', async ({ page }) => {
		// Wait for tabs to be visible
		await expect(page.locator('.tabs')).toBeVisible({ timeout: 10000 });

		const tabs = page.locator('.tabs .tab, .tabs button');

		// Click second tab (balances)
		await tabs.nth(1).click();
		await page.waitForTimeout(300);

		// Second tab should now be active
		await expect(tabs.nth(1)).toHaveClass(/active/);

		// Click first tab (records)
		await tabs.first().click();
		await page.waitForTimeout(300);

		// First tab should now be active
		await expect(tabs.first()).toHaveClass(/active/);
	});

	test('shows empty state or records table', async ({ page }) => {
		// Wait for content to load after page ready
		await expect(async () => {
			// Should show either empty state message or a table
			const hasEmptyState = await page.locator('.empty-state').isVisible().catch(() => false);
			const hasTable = await page.locator('table').isVisible().catch(() => false);
			const hasContent = await page.locator('.card').isVisible().catch(() => false);

			expect(hasEmptyState || hasTable || hasContent).toBeTruthy();
		}).toPass({ timeout: 10000 });
	});
});

test.describe('Demo Leave Management - Request Leave Modal', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/employees/absences', testInfo);
		await waitForPageLoaded(page);
	});

	test('can open request leave modal', async ({ page }) => {
		// Click the request leave button (handles i18n)
		const requestButton = page.getByRole('button', { name: /request|taotl|\+/i }).first();
		await expect(requestButton).toBeVisible({ timeout: 10000 });
		await requestButton.click();

		// Modal should be visible - use role="dialog" for accessibility
		const modal = page.locator('[role="dialog"], .modal');
		await expect(modal).toBeVisible({ timeout: 5000 });
	});

	test('request modal has required form fields', async ({ page }) => {
		// Open modal
		const requestButton = page.getByRole('button', { name: /request|taotl|\+/i }).first();
		await requestButton.click();

		const modal = page.locator('[role="dialog"], .modal');
		await expect(modal).toBeVisible({ timeout: 5000 });

		// Check for employee dropdown (using ID)
		await expect(modal.locator('#employee, select').first()).toBeVisible({ timeout: 5000 });

		// Check for absence type dropdown
		await expect(modal.locator('#absenceType, select').nth(1)).toBeVisible({ timeout: 5000 });

		// Check for date inputs
		await expect(modal.locator('#startDate, input[type="date"]').first()).toBeVisible();
	});

	test('can close request leave modal', async ({ page }) => {
		// Open modal
		const requestButton = page.getByRole('button', { name: /request|taotl|\+/i }).first();
		await requestButton.click();

		const modal = page.locator('[role="dialog"], .modal');
		await expect(modal).toBeVisible({ timeout: 5000 });

		// Click cancel button (handles i18n)
		const cancelButton = modal.getByRole('button', { name: /cancel|tühista/i });
		await cancelButton.click();

		// Modal should be closed
		await expect(modal).not.toBeVisible({ timeout: 5000 });
	});
});

test.describe('Demo Leave Management - Employee Selection', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/employees/absences', testInfo);
		await waitForPageLoaded(page);
	});

	test('employee dropdown contains options', async ({ page }) => {
		// Use the specific ID for the employee filter
		const employeeDropdown = page.locator('#employeeFilter');
		await expect(employeeDropdown).toBeVisible({ timeout: 10000 });

		// Get all options
		const options = await employeeDropdown.locator('option').allTextContents();

		// Should have at least "All Employees" option
		expect(options.length).toBeGreaterThanOrEqual(1);
	});

	test('selecting balances tab shows empty state when no employee selected', async ({ page }) => {
		// Wait for tabs to load
		const tabs = page.locator('.tabs .tab, .tabs button');
		await expect(tabs.first()).toBeVisible({ timeout: 10000 });

		// Switch to balances tab
		await tabs.nth(1).click();
		await page.waitForTimeout(500);

		// Without employee selected, should show message to select employee or empty state
		await expect(async () => {
			const needsEmployee = await page.getByText(/select.*employee|please select/i).isVisible().catch(() => false);
			const hasEmptyState = await page.locator('.empty-state').isVisible().catch(() => false);

			expect(needsEmployee || hasEmptyState).toBeTruthy();
		}).toPass({ timeout: 5000 });
	});
});
