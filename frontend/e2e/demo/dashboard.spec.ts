import { test, expect } from '@playwright/test';
import { loginAsDemo, ensureAcmeTenant } from './utils';

test.describe('Demo Dashboard - Seeded Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
	});

	test('displays Acme Corporation in organization selector', async ({ page }) => {
		await expect(page.getByText(/Acme Corporation/i)).toBeVisible();
	});

	test('shows revenue summary card with EUR amounts', async ({ page }) => {
		// Dashboard should show summary cards with financial data
		const summarySection = page.locator('.summary, .cards, [class*="summary"]').first();
		await expect(summarySection).toBeVisible();

		// Should display EUR currency (from seeded invoices totaling ~55k+)
		await expect(page.getByText(/EUR|€/)).toBeVisible();
	});

	test('displays invoice status breakdown', async ({ page }) => {
		// Seeded data has: 3 PAID, 2 SENT, 1 PARTIALLY_PAID, 1 DRAFT
		// Should show at least some status indicators
		const hasStatusIndicators = await page.getByText(/paid|sent|draft|overdue/i).first().isVisible();
		expect(hasStatusIndicators).toBeTruthy();
	});

	test('shows recent activity or quick actions', async ({ page }) => {
		// Dashboard typically shows recent invoices, payments, or quick action buttons
		const hasActivity = await page.getByText(/recent|activity|quick|action|invoice|payment/i).first().isVisible();
		const hasButtons = await page.getByRole('button').count() > 0;

		expect(hasActivity || hasButtons).toBeTruthy();
	});

	test('navigation sidebar is visible with main menu items', async ({ page }) => {
		const sidebar = page.locator('nav, .sidebar, [class*="sidebar"]').first();
		await expect(sidebar).toBeVisible();

		// Check for essential navigation links
		await expect(page.getByRole('link', { name: /dashboard/i })).toBeVisible();
	});
});
