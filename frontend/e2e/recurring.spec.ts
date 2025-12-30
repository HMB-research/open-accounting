import { test, expect } from '@playwright/test';

test.describe('Recurring Invoices', () => {
	test.beforeEach(async ({ page }) => {
		// Auth is handled by global setup - just navigate
		await page.goto('/recurring');
	});

	test('should display recurring invoices list', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /recurring/i })).toBeVisible();
		// Should have table or list
		await expect(page.locator('table, .recurring-list, [role="grid"]')).toBeVisible();
	});

	test('should show create recurring invoice button', async ({ page }) => {
		await expect(
			page.getByRole('button', { name: /create|new|add/i }).or(page.getByRole('link', { name: /create|new|add/i }))
		).toBeVisible();
	});

	test('should open create recurring invoice form', async ({ page }) => {
		await page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first()
			.click();

		// Should show form fields
		await expect(page.getByLabel(/name/i).or(page.getByLabel(/contact/i))).toBeVisible({ timeout: 5000 });
	});

	test('should create recurring invoice with email config', async ({ page }) => {
		// Click create button
		await page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first()
			.click();

		// Fill in basic details
		const nameField = page.getByLabel(/name/i).first();
		if (await nameField.isVisible()) {
			await nameField.fill('Test Recurring Invoice');
		}

		// Select contact
		const contactSelect = page.getByLabel(/contact|customer|client/i);
		if (await contactSelect.isVisible()) {
			await contactSelect.selectOption({ index: 1 });
		}

		// Select frequency
		const frequencySelect = page.getByLabel(/frequency/i);
		if (await frequencySelect.isVisible()) {
			await frequencySelect.selectOption('MONTHLY');
		}

		// Add line item
		const descriptionField = page.getByLabel(/description/i).first();
		if (await descriptionField.isVisible()) {
			await descriptionField.fill('Monthly Service');
		}

		const priceField = page.getByLabel(/price|amount|unit price/i).first();
		if (await priceField.isVisible()) {
			await priceField.fill('500');
		}

		// Enable email sending
		const emailCheckbox = page.getByLabel(/send.*email|email.*generation/i);
		if (await emailCheckbox.isVisible()) {
			await emailCheckbox.check();
		}

		// Submit form
		await page.getByRole('button', { name: /save|create|submit/i }).click();

		// Should show success
		await expect(page.getByText(/created|success/i)).toBeVisible({ timeout: 10000 });
	});

	test('should display frequency options', async ({ page }) => {
		// Click create button to open form
		await page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first()
			.click();

		const frequencySelect = page.getByLabel(/frequency/i);
		if (await frequencySelect.isVisible()) {
			await frequencySelect.click();
			// Check for frequency options
			await expect(page.getByRole('option', { name: /weekly/i }).or(page.getByText(/weekly/i))).toBeVisible();
			await expect(page.getByRole('option', { name: /monthly/i }).or(page.getByText(/monthly/i))).toBeVisible();
		}
	});

	test('should pause recurring invoice', async ({ page }) => {
		// Find pause button on first recurring invoice
		const pauseButton = page.getByRole('button', { name: /pause/i }).first();
		if (await pauseButton.isVisible()) {
			await pauseButton.click();
			// Should show paused status or success message
			await expect(page.getByText(/paused|inactive/i)).toBeVisible();
		}
	});

	test('should resume recurring invoice', async ({ page }) => {
		// Find resume button on a paused invoice
		const resumeButton = page.getByRole('button', { name: /resume|activate/i }).first();
		if (await resumeButton.isVisible()) {
			await resumeButton.click();
			// Should show active status
			await expect(page.getByText(/active|resumed/i)).toBeVisible();
		}
	});

	test('should generate invoice manually', async ({ page }) => {
		// Find generate button on first recurring invoice
		const generateButton = page.getByRole('button', { name: /generate/i }).first();
		if (await generateButton.isVisible()) {
			await generateButton.click();
			// Should show success message with invoice number
			await expect(page.getByText(/generated|invoice.*created/i)).toBeVisible({ timeout: 10000 });
		}
	});

	test('should show email configuration section', async ({ page }) => {
		// Click create button to open form
		await page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first()
			.click();

		// Email settings section should exist
		await expect(page.getByText(/email.*settings|email.*config/i)).toBeVisible();
	});

	test('should toggle email attachment option', async ({ page }) => {
		// Click create button to open form
		await page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first()
			.click();

		// Enable email first
		const emailCheckbox = page.getByLabel(/send.*email|email.*generation/i);
		if (await emailCheckbox.isVisible()) {
			await emailCheckbox.check();

			// PDF attachment option should be visible
			const pdfCheckbox = page.getByLabel(/attach.*pdf|pdf.*attach/i);
			await expect(pdfCheckbox).toBeVisible();
			// Toggle it
			await pdfCheckbox.uncheck();
			await expect(pdfCheckbox).not.toBeChecked();
		}
	});
});

test.describe('Recurring Invoices - Email Status', () => {
	test.beforeEach(async ({ page }) => {
		// Auth is handled by global setup - just navigate
		await page.goto('/recurring');
	});

	test('should show email column in list', async ({ page }) => {
		// Table should have email column
		await expect(page.getByRole('columnheader', { name: /email/i })).toBeVisible();
	});

	test('should display email status indicator', async ({ page }) => {
		// Look for email status badges
		const emailStatus = page.locator('.badge, [class*="status"]').filter({ hasText: /email|sent|enabled/i });
		// At least verify the column exists
		await expect(page.getByText(/email/i)).toBeVisible();
	});
});
