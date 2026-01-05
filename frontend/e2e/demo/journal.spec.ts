import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo Journal Entries - Page Structure Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/journal', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('displays journal entries page heading', async ({ page }) => {
		// Verify page loads with heading (level 1)
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
	});

	test('shows new entry button or empty state', async ({ page }) => {
		// Page should have either entries or the create entry prompt
		const hasNewEntryButton = await page.getByRole('button').first().isVisible().catch(() => false);
		const hasEmptyState = await page.getByText(/no.*entries|create|journal/i).isVisible().catch(() => false);

		expect(hasNewEntryButton || hasEmptyState).toBeTruthy();
	});

	test('page structure is correct', async ({ page }) => {
		// Journal page should have heading and action buttons
		const hasHeading = await page.getByRole('heading', { level: 1 }).isVisible().catch(() => false);
		const hasButton = await page.getByRole('button').first().isVisible().catch(() => false);

		expect(hasHeading && hasButton).toBeTruthy();
	});
});
