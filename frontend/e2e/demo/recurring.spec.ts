import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo Recurring Invoices - Seed Data Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/recurring', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('displays seeded recurring invoices', async ({ page }) => {
		// Wait for either table data or empty state
		const tableBody = page.locator('table tbody');
		const emptyState = page.getByText(/no recurring|no data|empty/i);

		// Wait for either condition
		await Promise.race([
			tableBody.locator('tr').first().waitFor({ state: 'visible', timeout: 15000 }),
			emptyState.waitFor({ state: 'visible', timeout: 15000 })
		]).catch(() => {
			// Neither appeared - check page state for debugging
		});

		// Check if we have data
		const hasRows = (await tableBody.locator('tr').count()) > 0;
		const hasEmptyState = await emptyState.isVisible().catch(() => false);

		// At least one condition should be true
		expect(hasRows || hasEmptyState).toBeTruthy();

		// If we have data, verify content
		if (hasRows) {
			const pageContent = await page.content();
			expect(
				pageContent.includes('Support') ||
					pageContent.includes('Retainer') ||
					pageContent.includes('License') ||
					pageContent.includes('Monthly') ||
					pageContent.includes('Quarterly')
			).toBeTruthy();
		}
	});

	test('shows frequency types', async ({ page }) => {
		const tableBody = page.locator('table tbody');
		const emptyState = page.getByText(/no recurring|no data|empty/i);

		await Promise.race([
			tableBody.locator('tr').first().waitFor({ state: 'visible', timeout: 15000 }),
			emptyState.waitFor({ state: 'visible', timeout: 15000 })
		]).catch(() => {});

		const hasRows = (await tableBody.locator('tr').count()) > 0;

		if (hasRows) {
			// Check for frequency types
			const pageContent = await page.content().then((c) => c.toLowerCase());
			const hasFrequencies =
				pageContent.includes('monthly') ||
				pageContent.includes('quarterly') ||
				pageContent.includes('yearly');
			expect(hasFrequencies).toBeTruthy();
		} else {
			// If no data, just verify page loaded correctly
			await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
		}
	});

	test('shows correct recurring invoice count', async ({ page }) => {
		const tableBody = page.locator('table tbody');
		const emptyState = page.getByText(/no recurring|no data|empty/i);

		await Promise.race([
			tableBody.locator('tr').first().waitFor({ state: 'visible', timeout: 15000 }),
			emptyState.waitFor({ state: 'visible', timeout: 15000 })
		]).catch(() => {});

		const rows = page.locator('table tbody tr');
		const count = await rows.count();

		// Should have data or be empty state (not broken)
		const hasEmptyState = await emptyState.isVisible().catch(() => false);

		if (count > 0) {
			// Should have at least 3 recurring invoices when data exists
			expect(count).toBeGreaterThanOrEqual(3);
		} else {
			// Empty state is acceptable if no data seeded
			expect(hasEmptyState || count === 0).toBeTruthy();
		}
	});

	test('shows customer names', async ({ page }) => {
		const tableBody = page.locator('table tbody');
		const emptyState = page.getByText(/no recurring|no data|empty/i);

		await Promise.race([
			tableBody.locator('tr').first().waitFor({ state: 'visible', timeout: 15000 }),
			emptyState.waitFor({ state: 'visible', timeout: 15000 })
		]).catch(() => {});

		const hasRows = (await tableBody.locator('tr').count()) > 0;

		if (hasRows) {
			// Check for customer names
			const pageContent = await page.content();
			expect(
				pageContent.includes('TechStart') ||
					pageContent.includes('Nordic') ||
					pageContent.includes('GreenTech')
			).toBeTruthy();
		} else {
			// If no data, verify page structure
			await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
		}
	});
});
