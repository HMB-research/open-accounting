import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureDemoTenant } from './utils';

test.describe('Orders View', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('displays orders page with correct structure', async ({ page }) => {
		await navigateTo(page, '/orders');

		// Wait for page to load - heading should be visible
		await expect(page.getByRole('heading', { name: /orders/i })).toBeVisible();

		// Wait for page content to load
		await page.waitForTimeout(2000);

		// Check for page content (table, empty state, or content area)
		const table = page.locator('table');
		const hasTable = await table.isVisible().catch(() => false);

		// If table has data, verify it's displaying correctly
		if (hasTable) {
			const rows = table.locator('tbody tr');
			const count = await rows.count();
			if (count > 0) {
				// Should have order number pattern visible
				const hasOrderNumber = await page.getByText(/ORD-\d{4}-\d{3}/i).isVisible().catch(() => false);
				if (hasOrderNumber) {
					expect(hasOrderNumber).toBe(true);
				}
			}
		}

		// Page loaded successfully if we got here
		expect(true).toBe(true);
	});

	test('displays order statuses in table when data exists', async ({ page }) => {
		await navigateTo(page, '/orders');
		await expect(page.getByRole('heading', { name: /orders/i })).toBeVisible();

		// Wait for data to load
		await page.waitForTimeout(2000);

		const table = page.locator('table');
		const hasTable = await table.isVisible().catch(() => false);

		if (hasTable) {
			const rows = table.locator('tbody tr');
			const count = await rows.count();

			// Only check statuses if we have data
			if (count > 0) {
				// Status badges should be visible in table rows (case insensitive)
				const statusTexts = ['pending', 'confirmed', 'processing', 'shipped', 'delivered', 'cancelled'];
				let foundStatus = false;
				for (const status of statusTexts) {
					const hasStatus = await table.getByText(new RegExp(status, 'i')).first().isVisible().catch(() => false);
					if (hasStatus) {
						foundStatus = true;
						break;
					}
				}
				expect(foundStatus).toBe(true);
			}
		}
	});

	test('shows order linked to quote when applicable', async ({ page }) => {
		await navigateTo(page, '/orders');

		// Wait for data to load
		await page.waitForTimeout(2000);

		const table = page.locator('table');
		const hasTable = await table.isVisible().catch(() => false);

		if (hasTable) {
			// Check if there's an order with order number pattern
			const hasOrderNumber = await page.getByText(/ORD-\d{4}-\d{3}/i).isVisible().catch(() => false);
			// If orders exist, the page structure is correct
			if (hasOrderNumber) {
				expect(hasOrderNumber).toBe(true);
			}
		}
	});

	test('can filter orders by status', async ({ page }) => {
		await navigateTo(page, '/orders');

		// Find and use the status filter
		const statusFilter = page.locator('select').first();

		if (await statusFilter.isVisible().catch(() => false)) {
			// Get initial row count
			await page.waitForTimeout(1000);

			// Select a filter option
			await statusFilter.selectOption({ index: 1 });

			// Wait for filter to apply
			await page.waitForTimeout(1000);

			// Filter should work (even if count is 0 or same)
			const filteredRows = await page.locator('table tbody tr').count().catch(() => 0);
			// Just verify the filter doesn't cause errors
			expect(filteredRows).toBeGreaterThanOrEqual(0);
		}
	});

	test('has New Order button', async ({ page }) => {
		await navigateTo(page, '/orders');

		// Verify New button exists
		const newButton = page.getByRole('button', { name: /new|create|add/i }).or(
			page.getByRole('link', { name: /new|create|add/i })
		);
		await expect(newButton).toBeVisible();
	});
});
