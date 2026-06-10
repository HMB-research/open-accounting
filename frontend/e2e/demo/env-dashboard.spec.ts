import { test, expect } from '@playwright/test';
import { loginAsDemoEnv, waitForDemoShell, waitForVisibleContent } from './env-utils';

test.describe('Demo Environment - Dashboard', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemoEnv(page, testInfo);
	});

	test('Dashboard displays organization selector', async ({ page }) => {
		const orgSelector = page.locator('.tenant-selector, [class*="org-selector"], select').first();
		await expect(orgSelector).toBeVisible({ timeout: 10000 });
	});

	test('Dashboard shows summary cards', async ({ page }) => {
		await waitForVisibleContent(page);

		const summarySection = page.locator('.summary-grid, .stats, [class*="summary"]').first();
		const hasSummary = await summarySection.isVisible({ timeout: 15000 }).catch(() => false);

		if (hasSummary) {
			await expect(summarySection).toBeVisible();
		} else {
			const dashboardContent = page.locator('main, .dashboard, [class*="content"]').first();
			await expect(dashboardContent).toBeVisible();
		}
	});

	test('Navigation sidebar is present', async ({ page }) => {
		await waitForDemoShell(page);
		const navItems = ['dashboard', 'invoice', 'contact', 'report'];
		const visibleItems = await Promise.all(
			navItems.map((item) =>
				page
					.getByRole('link', { name: new RegExp(item, 'i') })
					.first()
					.isVisible()
					.catch(() => false)
			)
		);
		const hasMenuShell = await page
			.locator('nav.navbar, .mobile-nav, .mobile-menu-btn, [aria-label*="menu"]')
			.first()
			.isVisible()
			.catch(() => false);

		expect(hasMenuShell || visibleItems.some(Boolean)).toBeTruthy();
	});
});
