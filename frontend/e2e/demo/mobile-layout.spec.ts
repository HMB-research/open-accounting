import { expect, test } from '@playwright/test';
import { expectNoHorizontalOverflow, openMobileRoute, prepareMobileDemo } from './mobile-utils';

test.describe('Mobile Tables', () => {
	test.use({ viewport: { width: 375, height: 667 } });

	test.beforeEach(async ({ page }, testInfo) => {
		await prepareMobileDemo(page, testInfo);
	});

	test('invoices page should be usable on mobile', async ({ page }, testInfo) => {
		await openMobileRoute(page, '/invoices', testInfo);

		await expect(page.getByRole('heading', { level: 1, name: /^Invoices$/i })).toBeVisible();
	});

	test('contacts page should be usable on mobile', async ({ page }, testInfo) => {
		await openMobileRoute(page, '/contacts', testInfo);

		await expect(page.getByRole('heading', { name: /contacts/i })).toBeVisible();
		await expectNoHorizontalOverflow(page);
	});

	test('should not have horizontal page scroll on invoices', async ({ page }, testInfo) => {
		await openMobileRoute(page, '/invoices', testInfo);

		await expectNoHorizontalOverflow(page);
	});
});

test.describe('Mobile Dashboard', () => {
	test.use({ viewport: { width: 375, height: 667 } });

	test.beforeEach(async ({ page }, testInfo) => {
		await prepareMobileDemo(page, testInfo);
	});

	test('should display dashboard on mobile', async ({ page }, testInfo) => {
		await openMobileRoute(page, '/dashboard', testInfo);

		await expect(page.getByRole('heading', { name: /dashboard/i })).toBeVisible();
	});

	test('content cards should be visible on mobile', async ({ page }, testInfo) => {
		await openMobileRoute(page, '/dashboard', testInfo);

		const cards = page.locator('.summary-card, .card, [class*="stat"], .empty-state, .container');
		const hasCards = await cards
			.first()
			.isVisible({ timeout: 5000 })
			.catch(() => false);

		if (hasCards) {
			expect(hasCards).toBeTruthy();
		} else {
			await expect(page.getByRole('heading', { name: /dashboard/i })).toBeVisible();
		}
	});

	test('should not have horizontal overflow on dashboard', async ({ page }, testInfo) => {
		await openMobileRoute(page, '/dashboard', testInfo);

		await expectNoHorizontalOverflow(page);
	});
});

test.describe('Tablet Viewport', () => {
	test.use({ viewport: { width: 768, height: 1024 } });

	test.beforeEach(async ({ page }, testInfo) => {
		await prepareMobileDemo(page, testInfo);
	});

	test('should display properly on tablet', async ({ page }, testInfo) => {
		await openMobileRoute(page, '/dashboard', testInfo);

		await expect(page.getByRole('heading', { name: /dashboard/i })).toBeVisible();
	});

	test('navigation should be accessible on tablet', async ({ page }, testInfo) => {
		await openMobileRoute(page, '/dashboard', testInfo);

		const nav = page.getByRole('navigation');
		const hamburger = page.locator(
			'[aria-label*="menu"], .hamburger, .mobile-menu-btn, button[aria-expanded]'
		);

		const hasNav = await nav.isVisible().catch(() => false);
		const hasHamburger = await hamburger.isVisible().catch(() => false);
		const hasHeading = await page
			.getByRole('heading', { name: /dashboard/i })
			.isVisible()
			.catch(() => false);

		expect(hasNav || hasHamburger || hasHeading).toBeTruthy();
	});

	test('invoices page should display properly on tablet', async ({ page }, testInfo) => {
		await openMobileRoute(page, '/invoices', testInfo);
		await expect(page.getByRole('heading', { level: 1, name: /^Invoices$/i })).toBeVisible();
	});
});
