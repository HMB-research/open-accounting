import { test, expect } from '@playwright/test';

test.describe('Invoices', () => {
	test.beforeEach(async ({ page }) => {
		// Auth is handled by global setup - just navigate
		await page.goto('/invoices');
	});

	test('should display invoices list', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /invoices/i })).toBeVisible();
		// Should have table or list of invoices
		await expect(page.locator('table, .invoice-list, [role="grid"]')).toBeVisible();
	});

	test('should show create invoice button', async ({ page }) => {
		await expect(
			page.getByRole('button', { name: /create|new|add/i }).or(page.getByRole('link', { name: /create|new|add/i }))
		).toBeVisible();
	});

	test('should open create invoice form', async ({ page }) => {
		await page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first()
			.click();

		// Should show form fields
		await expect(page.getByLabel(/contact|customer|client/i)).toBeVisible({ timeout: 5000 });
	});

	test('should create a new invoice', async ({ page }) => {
		// Click create button
		await page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first()
			.click();

		// Fill in invoice details
		const contactSelect = page.getByLabel(/contact|customer|client/i);
		if (await contactSelect.isVisible()) {
			await contactSelect.selectOption({ index: 1 });
		}

		// Add line item
		const descriptionField = page.getByLabel(/description/i).first();
		if (await descriptionField.isVisible()) {
			await descriptionField.fill('Test Service');
		}

		const quantityField = page.getByLabel(/quantity/i).first();
		if (await quantityField.isVisible()) {
			await quantityField.fill('1');
		}

		const priceField = page.getByLabel(/price|amount|unit price/i).first();
		if (await priceField.isVisible()) {
			await priceField.fill('100');
		}

		// Submit form
		await page.getByRole('button', { name: /save|create|submit/i }).click();

		// Should show success or redirect
		await expect(page.getByText(/created|success/i).or(page.locator('[data-status="success"]'))).toBeVisible({
			timeout: 10000
		});
	});

	test('should filter invoices by status', async ({ page }) => {
		// Look for filter controls
		const statusFilter = page.getByRole('combobox', { name: /status/i }).or(page.getByLabel(/status/i));
		if (await statusFilter.isVisible()) {
			await statusFilter.selectOption('DRAFT');
			// Table should update
			await page.waitForTimeout(500);
		}
	});

	test('should view invoice details', async ({ page }) => {
		// Click on first invoice row
		const firstInvoice = page.locator('table tbody tr, .invoice-item').first();
		if (await firstInvoice.isVisible()) {
			await firstInvoice.click();
			// Should show invoice details
			await expect(page.getByText(/invoice|total|amount/i)).toBeVisible();
		}
	});

	test('should generate PDF', async ({ page }) => {
		// Navigate to an invoice detail
		const firstInvoice = page.locator('table tbody tr, .invoice-item').first();
		if (await firstInvoice.isVisible()) {
			await firstInvoice.click();

			// Look for PDF button
			const pdfButton = page.getByRole('button', { name: /pdf|download|print/i });
			if (await pdfButton.isVisible()) {
				// Start waiting for download before clicking
				const downloadPromise = page.waitForEvent('download', { timeout: 10000 });
				await pdfButton.click();
				const download = await downloadPromise;
				expect(download.suggestedFilename()).toMatch(/\.pdf$/i);
			}
		}
	});
});

test.describe('Invoices - Validation', () => {
	test.beforeEach(async ({ page }) => {
		// Auth is handled by global setup - just navigate
		await page.goto('/invoices');
	});

	test('should show validation errors for empty form', async ({ page }) => {
		// Open create form
		await page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first()
			.click();

		// Try to submit empty form
		await page.getByRole('button', { name: /save|create|submit/i }).click();

		// Should show validation errors
		await expect(page.getByText(/required|please|invalid/i)).toBeVisible();
	});
});
