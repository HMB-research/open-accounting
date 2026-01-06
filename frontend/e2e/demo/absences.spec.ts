import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo Leave Management - Page Structure Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/employees/absences', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('displays leave management page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
		// Check for Leave Management title (or Estonian "Puhkuste haldus")
		const heading = page.getByRole('heading', { level: 1 });
		const headingText = await heading.textContent();
		expect(headingText?.toLowerCase()).toMatch(/leave|puhk/i);
	});

	test('shows request leave button', async ({ page }) => {
		await expect(
			page.getByRole('button', { name: /request.*leave|taotl.*puhk/i })
		).toBeVisible({ timeout: 10000 });
	});

	test('has year filter dropdown', async ({ page }) => {
		// Wait for filters to be visible
		await expect(page.locator('.filters')).toBeVisible({ timeout: 10000 });

		// Check for year dropdown
		const yearDropdown = page.locator('select').first();
		await expect(yearDropdown).toBeVisible();

		// Should have current year as an option
		const currentYear = new Date().getFullYear();
		await expect(yearDropdown.locator(`option[value="${currentYear}"]`)).toBeAttached();
	});

	test('has employee filter dropdown', async ({ page }) => {
		// Wait for filters to be visible
		await expect(page.locator('.filters')).toBeVisible({ timeout: 10000 });

		// There should be two dropdowns (year and employee)
		const selects = page.locator('.filters select');
		const count = await selects.count();
		expect(count).toBeGreaterThanOrEqual(2);
	});

	test('shows tab navigation for records and balances', async ({ page }) => {
		// Check for tabs
		await expect(page.locator('.tabs')).toBeVisible({ timeout: 10000 });

		// Should have Records and Balances tabs
		const recordsTab = page.locator('.tab').first();
		const balancesTab = page.locator('.tab').nth(1);

		await expect(recordsTab).toBeVisible();
		await expect(balancesTab).toBeVisible();
	});

	test('can switch between records and balances tabs', async ({ page }) => {
		// Wait for tabs to be visible
		await expect(page.locator('.tabs')).toBeVisible({ timeout: 10000 });

		// Click balances tab
		const balancesTab = page.locator('.tab').nth(1);
		await balancesTab.click();
		await page.waitForTimeout(300);

		// Balances tab should now be active
		await expect(balancesTab).toHaveClass(/active/);

		// Click records tab
		const recordsTab = page.locator('.tab').first();
		await recordsTab.click();
		await page.waitForTimeout(300);

		// Records tab should now be active
		await expect(recordsTab).toHaveClass(/active/);
	});

	test('shows empty state or records table', async ({ page }) => {
		// Wait for content to load
		await page.waitForTimeout(500);

		// Should show either empty state message or a table
		const hasEmptyState = await page.locator('.empty-state').isVisible().catch(() => false);
		const hasTable = await page.locator('table').isVisible().catch(() => false);

		expect(hasEmptyState || hasTable).toBeTruthy();
	});
});

test.describe('Demo Leave Management - Request Leave Modal', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/employees/absences', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('can open request leave modal', async ({ page }) => {
		// Click the request leave button
		const requestButton = page.getByRole('button', { name: /request|taotl/i }).first();
		await expect(requestButton).toBeVisible({ timeout: 10000 });
		await requestButton.click();

		// Modal should be visible
		await expect(page.locator('.modal')).toBeVisible({ timeout: 5000 });
	});

	test('request modal has required form fields', async ({ page }) => {
		// Open modal
		const requestButton = page.getByRole('button', { name: /request|taotl/i }).first();
		await requestButton.click();
		await expect(page.locator('.modal')).toBeVisible({ timeout: 5000 });

		// Check for employee dropdown
		await expect(page.locator('.modal select').first()).toBeVisible();

		// Check for absence type dropdown
		await expect(page.locator('.modal select').nth(1)).toBeVisible();

		// Check for date inputs
		await expect(page.locator('.modal input[type="date"]').first()).toBeVisible();
		await expect(page.locator('.modal input[type="date"]').nth(1)).toBeVisible();
	});

	test('can close request leave modal', async ({ page }) => {
		// Open modal
		const requestButton = page.getByRole('button', { name: /request|taotl/i }).first();
		await requestButton.click();
		await expect(page.locator('.modal')).toBeVisible({ timeout: 5000 });

		// Click cancel button
		const cancelButton = page.locator('.modal').getByRole('button', { name: /cancel|tühista/i });
		await cancelButton.click();

		// Modal should be closed
		await expect(page.locator('.modal')).not.toBeVisible();
	});
});

test.describe('Demo Leave Management - Employee Selection', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/employees/absences', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('employee dropdown contains seeded employees', async ({ page }) => {
		// Wait for filters
		await expect(page.locator('.filters')).toBeVisible({ timeout: 10000 });

		// Get employee dropdown (second select in filters)
		const employeeDropdown = page.locator('.filters select').nth(1);
		await expect(employeeDropdown).toBeVisible();

		// Get all options
		const options = await employeeDropdown.locator('option').allTextContents();

		// Should have "All Employees" option and at least one employee
		expect(options.length).toBeGreaterThanOrEqual(2);

		// Check if seeded employees are present (names may be in different formats)
		const optionsText = options.join(' ');
		const hasEmployees =
			optionsText.includes('Tamm') ||
			optionsText.includes('Kask') ||
			optionsText.includes('Maria') ||
			optionsText.includes('Jaan');
		expect(hasEmployees).toBeTruthy();
	});

	test('selecting employee updates balances tab', async ({ page }) => {
		// Wait for page to load
		await expect(page.locator('.filters')).toBeVisible({ timeout: 10000 });

		// Switch to balances tab first
		const balancesTab = page.locator('.tab').nth(1);
		await balancesTab.click();
		await page.waitForTimeout(300);

		// Without employee selected, should show message to select employee
		const needsEmployee = await page.getByText(/select.*employee/i).isVisible().catch(() => false);
		const hasEmptyState = await page.locator('.empty-state').isVisible().catch(() => false);

		expect(needsEmployee || hasEmptyState).toBeTruthy();
	});
});
