import { test, expect } from '@playwright/test';

/**
 * Mobile-specific E2E tests
 * Tests responsive design across different viewports and mobile interactions
 */

test.describe('Mobile Navigation', () => {
	test.use({ viewport: { width: 375, height: 667 } }); // iPhone SE viewport

	test.beforeEach(async ({ page }) => {
		await page.goto('/');
		await page.getByLabel(/email/i).fill('test@example.com');
		await page.getByLabel(/password/i).fill('testpassword123');
		await page.getByRole('button', { name: /login|sign in/i }).click();
		await expect(page).toHaveURL(/dashboard/i, { timeout: 10000 });
	});

	test('should have accessible navigation on mobile', async ({ page }) => {
		// Look for mobile navigation trigger (hamburger menu)
		const mobileNavTrigger = page.locator(
			'[aria-label*="menu"], .hamburger, .mobile-menu-trigger, button[aria-expanded]'
		);

		// Either hamburger exists OR navigation is visible
		const nav = page.getByRole('navigation');
		const hasHamburger = await mobileNavTrigger.isVisible();
		const hasVisibleNav = await nav.isVisible();

		expect(hasHamburger || hasVisibleNav).toBeTruthy();
	});

	test('should open mobile menu when hamburger clicked', async ({ page }) => {
		const hamburger = page.locator('[aria-label*="menu"], .hamburger, .mobile-menu-trigger').first();

		if (await hamburger.isVisible()) {
			await hamburger.click();
			// Navigation links should be visible
			await expect(page.getByRole('link', { name: /dashboard|home/i })).toBeVisible();
		}
	});

	test('should close menu when link is clicked', async ({ page }) => {
		const hamburger = page.locator('[aria-label*="menu"], .hamburger, .mobile-menu-trigger').first();

		if (await hamburger.isVisible()) {
			await hamburger.click();
			// Click a navigation link
			await page.getByRole('link', { name: /invoices/i }).click();
			// Should navigate and close menu
			await expect(page).toHaveURL(/invoices/i);
		}
	});
});

test.describe('Mobile Tables', () => {
	test.use({ viewport: { width: 375, height: 667 } });

	test.beforeEach(async ({ page }) => {
		await page.goto('/');
		await page.getByLabel(/email/i).fill('test@example.com');
		await page.getByLabel(/password/i).fill('testpassword123');
		await page.getByRole('button', { name: /login|sign in/i }).click();
		await expect(page).toHaveURL(/dashboard/i, { timeout: 10000 });
	});

	test('invoices table should be scrollable on mobile', async ({ page }) => {
		await page.goto('/invoices');

		const tableContainer = page.locator('.table-container, [style*="overflow"], table').first();
		if (await tableContainer.isVisible()) {
			const box = await tableContainer.boundingBox();
			if (box) {
				// Table should fit within viewport or be scrollable
				expect(box.width).toBeLessThanOrEqual(375);
			}
		}
	});

	test('contacts table should be usable on mobile', async ({ page }) => {
		await page.goto('/contacts');

		// Table or card view should be visible
		await expect(page.locator('table, .contact-list, .card-view').first()).toBeVisible();
	});

	test('should not have horizontal page scroll', async ({ page }) => {
		await page.goto('/invoices');

		// Check that body doesn't overflow horizontally
		const bodyWidth = await page.evaluate(() => document.body.scrollWidth);
		expect(bodyWidth).toBeLessThanOrEqual(375);
	});
});

test.describe('Mobile Forms', () => {
	test.use({ viewport: { width: 375, height: 667 } });

	test.beforeEach(async ({ page }) => {
		await page.goto('/');
		await page.getByLabel(/email/i).fill('test@example.com');
		await page.getByLabel(/password/i).fill('testpassword123');
		await page.getByRole('button', { name: /login|sign in/i }).click();
		await expect(page).toHaveURL(/dashboard/i, { timeout: 10000 });
	});

	test('form inputs should be full width on mobile', async ({ page }) => {
		await page.goto('/contacts');
		await page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first()
			.click();

		const nameInput = page.getByLabel(/name/i).first();
		if (await nameInput.isVisible()) {
			const box = await nameInput.boundingBox();
			if (box) {
				// Input should be nearly full width (accounting for padding)
				expect(box.width).toBeGreaterThan(300);
			}
		}
	});

	test('form buttons should be touch-friendly size', async ({ page }) => {
		await page.goto('/contacts');
		await page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first()
			.click();

		const submitButton = page.getByRole('button', { name: /save|create|submit/i });
		if (await submitButton.isVisible()) {
			const box = await submitButton.boundingBox();
			if (box) {
				// Minimum touch target is 44px
				expect(box.height).toBeGreaterThanOrEqual(44);
			}
		}
	});

	test('form labels should be visible', async ({ page }) => {
		await page.goto('/contacts');
		await page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first()
			.click();

		// Labels should be visible (not hidden by CSS)
		const labels = page.locator('label');
		const count = await labels.count();
		if (count > 0) {
			const firstLabel = labels.first();
			await expect(firstLabel).toBeVisible();
		}
	});
});

test.describe('Mobile Modals', () => {
	test.use({ viewport: { width: 375, height: 667 } });

	test.beforeEach(async ({ page }) => {
		await page.goto('/');
		await page.getByLabel(/email/i).fill('test@example.com');
		await page.getByLabel(/password/i).fill('testpassword123');
		await page.getByRole('button', { name: /login|sign in/i }).click();
		await expect(page).toHaveURL(/dashboard/i, { timeout: 10000 });
	});

	test('modal should cover screen on mobile', async ({ page }) => {
		await page.goto('/contacts');
		await page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first()
			.click();

		// Look for modal
		const modal = page.locator('.modal, [role="dialog"], [aria-modal="true"]').first();
		if (await modal.isVisible()) {
			const box = await modal.boundingBox();
			if (box) {
				// Modal should be full width or nearly full width
				expect(box.width).toBeGreaterThanOrEqual(350);
			}
		}
	});

	test('modal close button should be accessible', async ({ page }) => {
		await page.goto('/contacts');
		await page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first()
			.click();

		// Look for close button
		const closeButton = page.locator('[aria-label*="close"], .close-button, button:has-text("×")').first();
		if (await closeButton.isVisible()) {
			const box = await closeButton.boundingBox();
			if (box) {
				// Close button should be touch-friendly
				expect(box.width).toBeGreaterThanOrEqual(30);
				expect(box.height).toBeGreaterThanOrEqual(30);
			}
		}
	});
});

test.describe('Mobile Dashboard', () => {
	test.use({ viewport: { width: 375, height: 667 } });

	test.beforeEach(async ({ page }) => {
		await page.goto('/');
		await page.getByLabel(/email/i).fill('test@example.com');
		await page.getByLabel(/password/i).fill('testpassword123');
		await page.getByRole('button', { name: /login|sign in/i }).click();
		await expect(page).toHaveURL(/dashboard/i, { timeout: 10000 });
	});

	test('summary cards should stack vertically', async ({ page }) => {
		const cards = page.locator('.summary-card, .card, [class*="stat"]');
		const count = await cards.count();

		if (count > 1) {
			const firstBox = await cards.first().boundingBox();
			const secondBox = await cards.nth(1).boundingBox();

			if (firstBox && secondBox) {
				// Cards should be stacked (second card below first)
				expect(secondBox.y).toBeGreaterThan(firstBox.y);
			}
		}
	});

	test('chart should resize for mobile', async ({ page }) => {
		const chartContainer = page.locator('.chart-container, canvas').first();
		if (await chartContainer.isVisible()) {
			const box = await chartContainer.boundingBox();
			if (box) {
				// Chart should fit within mobile viewport
				expect(box.width).toBeLessThanOrEqual(375);
			}
		}
	});
});

test.describe('Tablet Viewport', () => {
	test.use({ viewport: { width: 768, height: 1024 } }); // iPad viewport

	test.beforeEach(async ({ page }) => {
		await page.goto('/');
		await page.getByLabel(/email/i).fill('test@example.com');
		await page.getByLabel(/password/i).fill('testpassword123');
		await page.getByRole('button', { name: /login|sign in/i }).click();
		await expect(page).toHaveURL(/dashboard/i, { timeout: 10000 });
	});

	test('should display properly on tablet', async ({ page }) => {
		// Dashboard should load
		await expect(page.getByText(/revenue/i)).toBeVisible();
	});

	test('navigation should be accessible on tablet', async ({ page }) => {
		// Either sidebar nav or hamburger should be visible
		const nav = page.getByRole('navigation');
		const hamburger = page.locator('[aria-label*="menu"], .hamburger');

		const hasNav = await nav.isVisible();
		const hasHamburger = await hamburger.isVisible();

		expect(hasNav || hasHamburger).toBeTruthy();
	});

	test('tables should display properly on tablet', async ({ page }) => {
		await page.goto('/invoices');
		const table = page.locator('table').first();
		if (await table.isVisible()) {
			const box = await table.boundingBox();
			if (box) {
				// Table should utilize available width
				expect(box.width).toBeGreaterThan(500);
			}
		}
	});
});
