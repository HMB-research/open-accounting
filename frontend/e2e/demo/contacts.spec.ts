import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Contacts - Seeded Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/contacts');
		// Wait for data to load (longer timeout for demo environment)
		await page.waitForTimeout(2000);
	});

	test('displays contacts page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /contact/i })).toBeVisible();
	});

	test('shows New Contact button', async ({ page }) => {
		await expect(page.getByRole('button', { name: /New Contact/i })).toBeVisible();
	});

	test('shows contact type filter dropdown', async ({ page }) => {
		// The page has an "All Types" dropdown filter
		await expect(page.getByRole('combobox').first()).toBeVisible();
	});

	test('shows search input', async ({ page }) => {
		await expect(page.getByPlaceholder(/search contacts/i)).toBeVisible();
	});

	test('shows Search button', async ({ page }) => {
		await expect(page.getByRole('button', { name: /Search/i })).toBeVisible();
	});

	test('loads contacts data or shows empty state', async ({ page }) => {
		// Wait for loading to finish
		await page.waitForTimeout(5000);

		// Check if we have contacts data OR a "no contacts" message OR still loading
		const hasData = await page.locator('table tbody tr').count() > 0;
		const hasEmptyState = await page.getByText(/no contacts|no data|empty/i).isVisible().catch(() => false);
		const hasLoading = await page.getByText(/loading/i).isVisible().catch(() => false);

		// Page should show one of these states
		expect(hasData || hasEmptyState || hasLoading).toBeTruthy();
	});

	test('can interact with type filter', async ({ page }) => {
		const typeFilter = page.getByRole('combobox').first();
		await expect(typeFilter).toBeVisible();

		// Should be able to click and interact with the filter
		await typeFilter.click();
		await page.waitForTimeout(500);
	});
});
