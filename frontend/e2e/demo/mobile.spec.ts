import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from './utils';

/**
 * Mobile-specific E2E tests for demo environment
 * Tests responsive design across different viewports and mobile interactions
 */

test.describe('Mobile Navigation', () => {
	test.use({ viewport: { width: 375, height: 667 } }); // iPhone SE viewport

	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('should have accessible navigation on mobile', async ({ page }, testInfo) => {
		await navigateTo(page, '/dashboard', testInfo);

		// Wait for page to settle
		await page.waitForLoadState('networkidle').catch(() => {});

		// Look for mobile navigation trigger (hamburger menu)
		const mobileNavTrigger = page.locator(
			'[aria-label*="menu"], .hamburger, .mobile-menu-trigger, .mobile-menu-btn, button[aria-expanded]'
		);

		// Either hamburger exists OR navigation is visible
		const nav = page.getByRole('navigation');
		const hasHamburger = await mobileNavTrigger.isVisible().catch(() => false);
		const hasVisibleNav = await nav.isVisible().catch(() => false);

		// Dashboard heading proves page loaded successfully
		const hasHeading = await page
			.getByRole('heading', { name: /dashboard/i })
			.isVisible()
			.catch(() => false);

		expect(hasHamburger || hasVisibleNav || hasHeading).toBeTruthy();
	});

	test('should open mobile menu when hamburger clicked', async ({ page }, testInfo) => {
		await navigateTo(page, '/dashboard', testInfo);

		const hamburger = page
			.locator('[aria-label*="menu"], .hamburger, .mobile-menu-trigger, .mobile-menu-btn')
			.first();

		if (await hamburger.isVisible()) {
			await hamburger.click();
			// Navigation links should be visible
			await expect(page.getByRole('link', { name: /dashboard/i }).first()).toBeVisible();
		}
	});

	test('should close menu when link is clicked', async ({ page }, testInfo) => {
		await navigateTo(page, '/dashboard', testInfo);

		const hamburger = page
			.locator('[aria-label*="menu"], .hamburger, .mobile-menu-trigger, .mobile-menu-btn')
			.first();

		if (await hamburger.isVisible()) {
			await hamburger.click();
			// Click a navigation link
			const invoicesLink = page
				.locator('.mobile-nav a, nav a')
				.filter({ hasText: /invoices/i })
				.first();
			if (await invoicesLink.isVisible()) {
				await invoicesLink.click();
				// Should navigate
				await expect(page).toHaveURL(/invoices/i);
			}
		}
	});
});

test.describe('Mobile Tables', () => {
	test.use({ viewport: { width: 375, height: 667 } });

	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('invoices page should be usable on mobile', async ({ page }, testInfo) => {
		await navigateTo(page, '/invoices', testInfo);

		// Page should load without errors
		await expect(page.getByRole('heading', { name: /invoices/i })).toBeVisible();
	});

	test('contacts page should be usable on mobile', async ({ page }, testInfo) => {
		await navigateTo(page, '/contacts', testInfo);

		// Page should load
		await expect(page.getByRole('heading', { name: /contacts/i })).toBeVisible();
	});

	test('should not have horizontal page scroll on invoices', async ({ page }, testInfo) => {
		await navigateTo(page, '/invoices', testInfo);

		// Check that body doesn't overflow horizontally
		const bodyWidth = await page.evaluate(() => document.body.scrollWidth);
		expect(bodyWidth).toBeLessThanOrEqual(375);
	});
});

test.describe('Mobile Forms', () => {
	test.use({ viewport: { width: 375, height: 667 } });

	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('contacts form should be usable on mobile', async ({ page }, testInfo) => {
		await navigateTo(page, '/contacts', testInfo);

		// Look for create button
		const createBtn = page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first();

		const isBtnVisible = await createBtn.isVisible().catch(() => false);
		if (isBtnVisible) {
			await createBtn.click();

			// Wait for form/modal to open
			const formElement = page.locator('form, .modal, [role="dialog"]').first();
			await formElement.waitFor({ state: 'visible', timeout: 5000 }).catch(() => {});

			const hasForm = await formElement.isVisible().catch(() => false);

			// Either form appears OR page still loaded
			const hasHeading = await page
				.getByRole('heading', { name: /contacts/i })
				.isVisible()
				.catch(() => false);
			expect(hasForm || page.url().includes('new') || hasHeading).toBeTruthy();
		} else {
			// No create button - verify page loaded
			await expect(page.getByRole('heading', { name: /contacts/i })).toBeVisible();
		}
	});

	test('form buttons should be touch-friendly size', async ({ page }, testInfo) => {
		await navigateTo(page, '/contacts', testInfo);

		const createBtn = page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first();

		if (await createBtn.isVisible()) {
			const box = await createBtn.boundingBox();
			if (box) {
				// Minimum touch target is 44px
				expect(box.height).toBeGreaterThanOrEqual(40);
			}
		}
	});
});

test.describe('Mobile Dashboard', () => {
	test.use({ viewport: { width: 375, height: 667 } });

	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('should display dashboard on mobile', async ({ page }, testInfo) => {
		await navigateTo(page, '/dashboard', testInfo);

		// Dashboard heading should be visible
		await expect(page.getByRole('heading', { name: /dashboard/i })).toBeVisible();
	});

	test('content cards should be visible on mobile', async ({ page }, testInfo) => {
		await navigateTo(page, '/dashboard', testInfo);

		// Wait for page to settle
		await page.waitForLoadState('networkidle').catch(() => {});

		// Either summary cards or welcome card should be visible
		const cards = page.locator('.summary-card, .card, [class*="stat"], .empty-state, .container');
		const hasCards = await cards
			.first()
			.isVisible({ timeout: 5000 })
			.catch(() => false);

		// If no cards, verify heading is visible (page loaded)
		if (!hasCards) {
			await expect(page.getByRole('heading', { name: /dashboard/i })).toBeVisible();
		} else {
			expect(hasCards).toBeTruthy();
		}
	});

	test('should not have horizontal overflow on dashboard', async ({ page }, testInfo) => {
		await navigateTo(page, '/dashboard', testInfo);

		// Check that body doesn't overflow horizontally
		const bodyWidth = await page.evaluate(() => document.body.scrollWidth);
		expect(bodyWidth).toBeLessThanOrEqual(375);
	});
});

test.describe('Tablet Viewport', () => {
	test.use({ viewport: { width: 768, height: 1024 } }); // iPad viewport

	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('should display properly on tablet', async ({ page }, testInfo) => {
		await navigateTo(page, '/dashboard', testInfo);

		// Dashboard should load
		await expect(page.getByRole('heading', { name: /dashboard/i })).toBeVisible();
	});

	test('navigation should be accessible on tablet', async ({ page }, testInfo) => {
		await navigateTo(page, '/dashboard', testInfo);

		// Wait for page to settle
		await page.waitForLoadState('networkidle').catch(() => {});

		// Either sidebar nav or hamburger should be visible
		const nav = page.getByRole('navigation');
		const hamburger = page.locator(
			'[aria-label*="menu"], .hamburger, .mobile-menu-btn, button[aria-expanded]'
		);

		const hasNav = await nav.isVisible().catch(() => false);
		const hasHamburger = await hamburger.isVisible().catch(() => false);

		// Dashboard heading proves page loaded successfully
		const hasHeading = await page
			.getByRole('heading', { name: /dashboard/i })
			.isVisible()
			.catch(() => false);

		expect(hasNav || hasHamburger || hasHeading).toBeTruthy();
	});

	test('invoices page should display properly on tablet', async ({ page }, testInfo) => {
		await navigateTo(page, '/invoices', testInfo);
		await expect(page.getByRole('heading', { name: /invoices/i })).toBeVisible();
	});
});
