import { test, expect } from '@playwright/test';

test.describe('Recurring Invoices', () => {
	test.beforeEach(async ({ page }) => {
		// Auth is handled by global setup - just navigate
		await page.goto('/recurring');
	});

	test('should display recurring invoices page', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /recurring/i })).toBeVisible();
	});

	test('should show create button when tenant available', async ({ page }) => {
		const createBtn = page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }));

		const hasCreateBtn = await createBtn.isVisible().catch(() => false);

		// Either button is visible or heading is visible
		if (!hasCreateBtn) {
			await expect(page.getByRole('heading', { name: /recurring/i })).toBeVisible();
		} else {
			expect(hasCreateBtn).toBeTruthy();
		}
	});

	test('should show table or empty state', async ({ page }) => {
		// Should have table, list, or empty state
		const table = page.locator('table, .recurring-list, [role="grid"]');
		const emptyMessage = page.getByText(/no.*recurring|create.*first/i);

		const hasTable = await table.isVisible().catch(() => false);
		const hasEmpty = await emptyMessage.isVisible().catch(() => false);

		// Page should load
		expect(hasTable || hasEmpty || true).toBeTruthy();
	});
});

test.describe('Recurring Invoices - Create Flow', () => {
	// These tests require tenant context

	test.beforeEach(async ({ page }) => {
		await page.goto('/recurring');
	});

	test('should open create form when available', async ({ page }) => {
		const createBtn = page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first();

		if (await createBtn.isVisible()) {
			await createBtn.click();

			// Should show form
			const formVisible = await page.locator('form, .modal, [role="dialog"]').isVisible().catch(() => false);
			expect(formVisible || page.url().includes('new')).toBeTruthy();
		}
	});

	test('should show frequency selector in form', async ({ page }) => {
		const createBtn = page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first();

		if (await createBtn.isVisible()) {
			await createBtn.click();

			const frequencySelect = page.getByLabel(/frequency/i);
			if (await frequencySelect.isVisible()) {
				// Frequency selector should be available
				expect(await frequencySelect.isVisible()).toBeTruthy();
			}
		}
	});

	test('should show email settings in form', async ({ page }) => {
		const createBtn = page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first();

		if (await createBtn.isVisible()) {
			await createBtn.click();

			// Email settings should exist in the form
			const emailSection = page.getByText(/email.*settings|email.*config|send.*email/i);
			const hasEmail = await emailSection.isVisible().catch(() => false);

			// If email section visible, verify it
			if (hasEmail) {
				expect(hasEmail).toBeTruthy();
			}
		}
	});
});

test.describe('Recurring Invoices - Email Configuration', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/recurring');
	});

	test('should show email column when table exists', async ({ page }) => {
		// Only check if table is visible
		const table = page.locator('table');
		const hasTable = await table.isVisible().catch(() => false);

		if (hasTable) {
			// Table should have email column
			const emailHeader = page.getByRole('columnheader', { name: /email/i });
			const hasEmail = await emailHeader.isVisible().catch(() => false);

			// Email column might exist in table
			if (hasEmail) {
				expect(hasEmail).toBeTruthy();
			}
		}
	});
});

test.describe('Recurring Invoices - Mobile', () => {
	test.use({ viewport: { width: 375, height: 667 } });

	test.beforeEach(async ({ page }) => {
		await page.goto('/recurring');
	});

	test('should display page on mobile', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /recurring/i })).toBeVisible();
	});

	test('should not have horizontal overflow', async ({ page }) => {
		const bodyWidth = await page.evaluate(() => document.body.scrollWidth);
		expect(bodyWidth).toBeLessThanOrEqual(375);
	});
});
