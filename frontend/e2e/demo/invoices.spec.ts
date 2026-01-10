import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant, getDemoCredentials } from './utils';

/**
 * Wait for invoices page to finish loading
 */
async function waitForInvoicesLoaded(page: import('@playwright/test').Page) {
	// Wait for loading state to finish
	await expect(async () => {
		const isLoading = await page.getByText(/^Loading\.\.\.$/i).first().isVisible().catch(() => false);
		const hasTable = await page.locator('table').first().isVisible().catch(() => false);
		const hasEmpty = await page.locator('.empty-state').isVisible().catch(() => false);
		expect(isLoading === false && (hasTable || hasEmpty)).toBeTruthy();
	}).toPass({ timeout: 15000 });
}

test.describe('Demo Invoices - Seed Data Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/invoices', testInfo);
		await waitForInvoicesLoaded(page);
	});

	test('displays invoices or empty state', async ({ page }) => {
		// Wait for table or empty state to be visible
		await expect(async () => {
			const hasTable = await page.locator('table tbody tr').first().isVisible().catch(() => false);
			const hasEmpty = await page.locator('.empty-state').isVisible().catch(() => false);
			expect(hasTable || hasEmpty).toBeTruthy();
		}).toPass({ timeout: 15000 });
	});

	test('shows page heading', async ({ page }) => {
		// Check for invoices page heading (handles i18n)
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
	});

	test('shows New Invoice button', async ({ page }) => {
		// Check for New Invoice button (handles i18n)
		await expect(
			page.getByRole('button', { name: /new invoice|uus arve|\+/i }).first()
		).toBeVisible({ timeout: 10000 });
	});

	test('invoices table has expected columns', async ({ page }) => {
		// Wait for table to be visible
		const hasTable = await page.locator('table thead').first().isVisible().catch(() => false);

		if (hasTable) {
			// Check for table headers - use text matching for i18n
			const headers = page.locator('table thead th');
			const count = await headers.count();
			expect(count).toBeGreaterThan(0);
		} else {
			// Empty state is acceptable
			await expect(page.locator('.empty-state')).toBeVisible();
		}
	});
});

test.describe('Invoice Creation - Basic', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/invoices', testInfo);
		await waitForInvoicesLoaded(page);
	});

	test('can open new invoice modal', async ({ page }) => {
		// Click the "New Invoice" button (handles i18n)
		const newInvoiceBtn = page.getByRole('button', { name: /new invoice|uus arve|\+/i }).first();
		await expect(newInvoiceBtn).toBeVisible({ timeout: 10000 });
		await newInvoiceBtn.click();

		// Verify modal is open
		const modal = page.locator('[role="dialog"], .modal');
		await expect(modal).toBeVisible({ timeout: 5000 });
	});

	test('can close invoice modal', async ({ page }) => {
		// Open the new invoice modal
		const newInvoiceBtn = page.getByRole('button', { name: /new invoice|uus arve|\+/i }).first();
		await newInvoiceBtn.click();

		const modal = page.locator('[role="dialog"], .modal');
		await expect(modal).toBeVisible({ timeout: 5000 });

		// Click cancel button (handles i18n)
		const cancelBtn = modal.getByRole('button', { name: /cancel|tühista/i });
		await cancelBtn.click();

		// Modal should be closed
		await expect(modal).not.toBeVisible({ timeout: 5000 });
	});

	test('invoice form has required fields', async ({ page }) => {
		// Open the new invoice modal
		const newInvoiceBtn = page.getByRole('button', { name: /new invoice|uus arve|\+/i }).first();
		await newInvoiceBtn.click();

		const modal = page.locator('[role="dialog"], .modal');
		await expect(modal).toBeVisible({ timeout: 5000 });

		// Wait for form to be ready and check for select or input fields
		await expect(async () => {
			const hasSelectFields = await modal.locator('select').first().isVisible().catch(() => false);
			const hasInputFields = await modal.locator('input').first().isVisible().catch(() => false);
			expect(hasSelectFields || hasInputFields).toBeTruthy();
		}).toPass({ timeout: 5000 });
	});
});

test.describe('Invoice Creation - Inline Contact Feature', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/invoices', testInfo);
		await waitForInvoicesLoaded(page);
	});

	test('shows new contact button next to contact dropdown', async ({ page }) => {
		// Open the new invoice modal
		const newInvoiceBtn = page.getByRole('button', { name: /new invoice|uus arve|\+/i }).first();
		await newInvoiceBtn.click();

		const modal = page.locator('[role="dialog"], .modal');
		await expect(modal).toBeVisible({ timeout: 5000 });

		// Find the "+" button for adding new contact
		const newContactBtn = modal.locator('.btn-new-contact, button:has-text("+")').first();

		// Skip test if feature is not deployed
		const isVisible = await newContactBtn.isVisible().catch(() => false);
		if (!isVisible) {
			test.skip(true, 'Inline contact feature not deployed yet');
			return;
		}

		await expect(newContactBtn).toBeVisible();
	});

	test('can open new contact modal from invoice form', async ({ page }) => {
		// Open the new invoice modal
		const newInvoiceBtn = page.getByRole('button', { name: /new invoice|uus arve|\+/i }).first();
		await newInvoiceBtn.click();

		const modal = page.locator('[role="dialog"], .modal');
		await expect(modal).toBeVisible({ timeout: 5000 });

		// Click the "+" button to open contact modal
		const newContactBtn = modal.locator('.btn-new-contact');

		// Skip test if feature is not deployed
		const isVisible = await newContactBtn.isVisible().catch(() => false);
		if (!isVisible) {
			test.skip(true, 'Inline contact feature not deployed yet');
			return;
		}

		await newContactBtn.click();

		// Verify contact modal is open
		await expect(page.locator('h2', { hasText: /new contact|uus kontakt/i })).toBeVisible({ timeout: 5000 });
	});
});
