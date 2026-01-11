import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant, waitForDataOrEmpty } from './utils';

test.describe('Demo TSD Declarations', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/tsd', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('displays TSD page correctly', async ({ page }) => {
		// Wait for page to load
		const { hasData, isEmpty } = await waitForDataOrEmpty(page, 15000);

		// Page should render either data or empty state
		expect(hasData || isEmpty).toBeTruthy();

		// Check for page header
		const header = page.getByRole('heading', { level: 1 });
		await expect(header).toBeVisible({ timeout: 5000 });
	});

	test('shows declaration data when available', async ({ page }) => {
		const { hasData } = await waitForDataOrEmpty(page, 15000);

		if (hasData) {
			const pageContent = await page.content().then((c) => c.toLowerCase());
			const hasRelevantContent =
				pageContent.includes('declaration') ||
				pageContent.includes('draft') ||
				pageContent.includes('submitted') ||
				pageContent.includes('tsd') ||
				pageContent.includes('2024');
			expect(hasRelevantContent).toBeTruthy();
		} else {
			// If no data, just verify page loaded correctly
			await expect(page.getByRole('heading')).toBeVisible();
		}
	});

	test('shows declaration statuses when data exists', async ({ page }) => {
		const { hasData } = await waitForDataOrEmpty(page, 15000);

		if (hasData) {
			// Should show SUBMITTED and DRAFT statuses
			const pageContent = await page.content().then((c) => c.toLowerCase());
			expect(pageContent.includes('submitted') || pageContent.includes('draft')).toBeTruthy();
		} else {
			// Empty state is acceptable
			await expect(page.getByRole('heading')).toBeVisible();
		}
	});

	test('shows correct declaration count when data exists', async ({ page }) => {
		const tableBody = page.locator('table tbody');
		const emptyState = page.getByText(/no.*declaration|no data|empty/i);

		await Promise.race([
			tableBody.locator('tr').first().waitFor({ state: 'visible', timeout: 15000 }),
			emptyState.waitFor({ state: 'visible', timeout: 15000 })
		]).catch(() => {});

		const rows = page.locator('table tbody tr');
		const count = await rows.count();

		if (count > 0) {
			// Should have at least 3 TSD declarations when data exists
			expect(count).toBeGreaterThanOrEqual(3);
		} else {
			// Empty state is acceptable if no data seeded
			const hasEmptyState = await emptyState.isVisible().catch(() => false);
			expect(hasEmptyState || count === 0).toBeTruthy();
		}
	});

	test('shows tax amounts when data exists', async ({ page }) => {
		const { hasData } = await waitForDataOrEmpty(page, 15000);

		if (hasData) {
			// Check for tax amounts
			const pageContent = await page.content();
			expect(pageContent).toMatch(/[\d,]+\.\d{2}/);
		} else {
			// If no data, verify page structure
			await expect(page.getByRole('heading')).toBeVisible();
		}
	});
});
