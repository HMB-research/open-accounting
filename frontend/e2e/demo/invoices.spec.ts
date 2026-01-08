import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureDemoTenant, getDemoCredentials } from './utils';

test.describe('Demo Invoices - Seed Data Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/invoices', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('displays seeded invoices', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Verify invoice numbers are visible (format: INV{N}-YYYY-NNN)
		const pageContent = await page.content();
		expect(pageContent).toMatch(/INV\d?-?202[45]-\d{3}/);
	});

	test('shows invoices with various statuses', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check for status indicators (PAID, SENT, DRAFT)
		const pageContent = await page.content();
		const hasStatuses =
			pageContent.toLowerCase().includes('paid') ||
			pageContent.toLowerCase().includes('sent') ||
			pageContent.toLowerCase().includes('draft');
		expect(hasStatuses).toBeTruthy();
	});

	test('shows correct invoice count', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Should have at least 9 invoices
		const rows = page.locator('table tbody tr');
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(9);
	});

	test('invoices table has customer column', async ({ page }) => {
		await expect(page.locator('table tbody tr').first()).toBeVisible({ timeout: 10000 });

		// Check that Customer column exists in the table header
		await expect(page.getByRole('columnheader', { name: /customer/i })).toBeVisible();
	});
});

test.describe('Invoice Creation - Basic', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/invoices', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('can open new invoice modal', async ({ page }) => {
		// Click the "New Invoice" button
		const newInvoiceBtn = page.getByRole('button', { name: /new invoice|uus arve/i });
		await expect(newInvoiceBtn).toBeVisible({ timeout: 10000 });
		await newInvoiceBtn.click();

		// Verify modal is open
		await expect(page.locator('.modal')).toBeVisible({ timeout: 5000 });
		await expect(page.locator('h2', { hasText: /new invoice|uus arve/i })).toBeVisible();
	});

	test('can select existing contact in invoice form', async ({ page }) => {
		// Open the new invoice modal
		const newInvoiceBtn = page.getByRole('button', { name: /new invoice|uus arve/i });
		await newInvoiceBtn.click();
		await expect(page.locator('.modal')).toBeVisible({ timeout: 5000 });

		// Wait for contacts to load in the dropdown
		const contactSelect = page.locator('#contact');
		await expect(contactSelect).toBeVisible();

		// Click to open dropdown and wait for options
		await contactSelect.click();
		await page.waitForTimeout(500);

		// Should have at least one contact option besides the placeholder
		const options = await contactSelect.locator('option').all();
		expect(options.length).toBeGreaterThan(1);
	});

	test('validates required fields', async ({ page }) => {
		// Open the new invoice modal
		const newInvoiceBtn = page.getByRole('button', { name: /new invoice|uus arve/i });
		await newInvoiceBtn.click();
		await expect(page.locator('.modal')).toBeVisible({ timeout: 5000 });

		// Try to submit without filling required fields
		const submitBtn = page.getByRole('button', { name: /new invoice|uus arve/i }).last();
		await submitBtn.click();

		// Modal should still be visible (form not submitted due to validation)
		await expect(page.locator('.modal')).toBeVisible();

		// Contact field should be required
		const contactSelect = page.locator('#contact');
		const isInvalid = await contactSelect.evaluate((el: HTMLSelectElement) => !el.checkValidity());
		expect(isInvalid).toBeTruthy();
	});

	test('can close invoice modal', async ({ page }) => {
		// Open the new invoice modal
		const newInvoiceBtn = page.getByRole('button', { name: /new invoice|uus arve/i });
		await newInvoiceBtn.click();
		await expect(page.locator('.modal')).toBeVisible({ timeout: 5000 });

		// Click cancel button
		const cancelBtn = page.getByRole('button', { name: /cancel|tühista/i });
		await cancelBtn.click();

		// Modal should be closed
		await expect(page.locator('.modal-large')).not.toBeVisible();
	});

	test('can add and remove line items', async ({ page }) => {
		// Open the new invoice modal
		const newInvoiceBtn = page.getByRole('button', { name: /new invoice|uus arve/i });
		await newInvoiceBtn.click();
		await expect(page.locator('.modal')).toBeVisible({ timeout: 5000 });

		// Initially should have 1 line
		const lineRows = page.locator('.lines-table tbody tr');
		await expect(lineRows).toHaveCount(1);

		// Click "Add Line" button
		const addLineBtn = page.getByRole('button', { name: /add line|lisa rida/i });
		await addLineBtn.click();

		// Should now have 2 lines
		await expect(lineRows).toHaveCount(2);

		// Click remove button on second line
		const removeBtn = page.locator('.lines-table tbody tr').last().locator('.btn-danger');
		await removeBtn.click();

		// Should be back to 1 line
		await expect(lineRows).toHaveCount(1);
	});
});

test.describe('Invoice Creation - Inline Contact Feature', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/invoices', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('shows new contact button next to contact dropdown', async ({ page }) => {
		// Open the new invoice modal
		const newInvoiceBtn = page.getByRole('button', { name: /new invoice|uus arve/i });
		await newInvoiceBtn.click();
		await expect(page.locator('.modal')).toBeVisible({ timeout: 5000 });

		// Find the "+" button for adding new contact
		const newContactBtn = page.locator('.btn-new-contact');

		// Skip test if feature is not deployed
		const isVisible = await newContactBtn.isVisible().catch(() => false);
		if (!isVisible) {
			test.skip(true, 'Inline contact feature not deployed yet');
			return;
		}

		await expect(newContactBtn).toBeVisible();
		expect(await newContactBtn.textContent()).toBe('+');
	});

	test('can open new contact modal from invoice form', async ({ page }) => {
		// Open the new invoice modal
		const newInvoiceBtn = page.getByRole('button', { name: /new invoice|uus arve/i });
		await newInvoiceBtn.click();
		await expect(page.locator('.modal')).toBeVisible({ timeout: 5000 });

		// Click the "+" button to open contact modal
		const newContactBtn = page.locator('.btn-new-contact');

		// Skip test if feature is not deployed
		const isVisible = await newContactBtn.isVisible().catch(() => false);
		if (!isVisible) {
			test.skip(true, 'Inline contact feature not deployed yet');
			return;
		}

		await newContactBtn.click();

		// Verify contact modal is open (should have higher z-index)
		// Look for a second modal with contact-specific content
		await expect(page.locator('h2', { hasText: /new contact|uus kontakt/i })).toBeVisible({ timeout: 5000 });

		// Verify contact form fields are visible
		await expect(page.locator('#contact-name')).toBeVisible();
		await expect(page.locator('#contact-type')).toBeVisible();
	});

	test('can create new contact from invoice form', async ({ page }, testInfo) => {
		const uniqueName = `E2E Test Contact ${Date.now()}`;

		// Open the new invoice modal
		const newInvoiceBtn = page.getByRole('button', { name: /new invoice|uus arve/i });
		await newInvoiceBtn.click();
		await expect(page.locator('.modal')).toBeVisible({ timeout: 5000 });

		// Click the "+" button to open contact modal
		const newContactBtn = page.locator('.btn-new-contact');

		// Skip test if feature is not deployed
		const isVisible = await newContactBtn.isVisible().catch(() => false);
		if (!isVisible) {
			test.skip(true, 'Inline contact feature not deployed yet');
			return;
		}

		await newContactBtn.click();
		await expect(page.locator('#contact-name')).toBeVisible({ timeout: 5000 });

		// Fill in contact form
		await page.locator('#contact-name').fill(uniqueName);
		await page.locator('#contact-type').selectOption('CUSTOMER');
		await page.locator('#contact-email').fill('e2e-test@example.com');

		// Submit contact form
		const createBtn = page.getByRole('button', { name: /create|loo/i }).last();
		await createBtn.click();

		// Wait for modal to close and contact to be selected
		await expect(page.locator('#contact-name')).not.toBeVisible({ timeout: 10000 });

		// Verify the new contact is selected in the dropdown
		const contactSelect = page.locator('#contact');
		await expect(contactSelect).toBeVisible();

		// The newly created contact should be the selected option
		const selectedText = await contactSelect.locator('option:checked').textContent();
		expect(selectedText).toContain(uniqueName);
	});
});
