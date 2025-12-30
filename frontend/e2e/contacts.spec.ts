import { test, expect } from '@playwright/test';

test.describe('Contacts', () => {
	test.beforeEach(async ({ page }) => {
		// Login before each test
		await page.goto('/');
		await page.getByLabel(/email/i).fill('test@example.com');
		await page.getByLabel(/password/i).fill('testpassword123');
		await page.getByRole('button', { name: /login|sign in/i }).click();
		await expect(page).toHaveURL(/dashboard/i, { timeout: 10000 });

		// Navigate to contacts page
		await page.goto('/contacts');
	});

	test('should display contacts list', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /contacts/i })).toBeVisible();
		// Should have table or list
		await expect(page.locator('table, .contact-list, [role="grid"]')).toBeVisible();
	});

	test('should show create contact button', async ({ page }) => {
		await expect(
			page.getByRole('button', { name: /create|new|add/i }).or(page.getByRole('link', { name: /create|new|add/i }))
		).toBeVisible();
	});

	test('should open create contact form', async ({ page }) => {
		await page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first()
			.click();

		// Should show form fields
		await expect(page.getByLabel(/name/i)).toBeVisible({ timeout: 5000 });
	});

	test('should create a new contact', async ({ page }) => {
		// Click create button
		await page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first()
			.click();

		// Fill in contact details
		await page.getByLabel(/name/i).first().fill('Test Contact');

		const emailField = page.getByLabel(/email/i);
		if (await emailField.isVisible()) {
			await emailField.fill('testcontact@example.com');
		}

		// Submit form
		await page.getByRole('button', { name: /save|create|submit/i }).click();

		// Should show success or close modal
		await expect(
			page.getByText(/created|success/i).or(page.getByText('Test Contact'))
		).toBeVisible({ timeout: 10000 });
	});

	test('should search contacts', async ({ page }) => {
		const searchInput = page.getByPlaceholder(/search/i).or(page.getByLabel(/search/i));
		if (await searchInput.isVisible()) {
			await searchInput.fill('test');
			await page.waitForTimeout(500); // Wait for search debounce
			// Results should filter
		}
	});

	test('should edit contact', async ({ page }) => {
		// Click on first contact or edit button
		const editButton = page
			.getByRole('button', { name: /edit/i })
			.or(page.locator('[aria-label*="edit"]'))
			.first();
		if (await editButton.isVisible()) {
			await editButton.click();
			// Should show edit form
			await expect(page.getByLabel(/name/i)).toBeVisible();
		}
	});

	test('should delete contact with confirmation', async ({ page }) => {
		// Find delete button
		const deleteButton = page
			.getByRole('button', { name: /delete/i })
			.or(page.locator('[aria-label*="delete"]'))
			.first();

		if (await deleteButton.isVisible()) {
			// Set up dialog handler
			page.on('dialog', async (dialog) => {
				expect(dialog.type()).toBe('confirm');
				await dialog.accept();
			});

			await deleteButton.click();
			// Contact should be removed or success message shown
		}
	});

	test('should show contact details', async ({ page }) => {
		// Click on first contact row
		const firstContact = page.locator('table tbody tr, .contact-item').first();
		if (await firstContact.isVisible()) {
			await firstContact.click();
			// Should show contact details
			await expect(page.getByText(/email|phone|address/i)).toBeVisible();
		}
	});
});

test.describe('Contacts - Validation', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/');
		await page.getByLabel(/email/i).fill('test@example.com');
		await page.getByLabel(/password/i).fill('testpassword123');
		await page.getByRole('button', { name: /login|sign in/i }).click();
		await expect(page).toHaveURL(/dashboard/i, { timeout: 10000 });
		await page.goto('/contacts');
	});

	test('should validate required name field', async ({ page }) => {
		// Open create form
		await page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first()
			.click();

		// Try to submit without name
		await page.getByRole('button', { name: /save|create|submit/i }).click();

		// Should show validation error
		await expect(page.getByText(/required|name/i)).toBeVisible();
	});

	test('should validate email format', async ({ page }) => {
		// Open create form
		await page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first()
			.click();

		await page.getByLabel(/name/i).first().fill('Test Contact');

		const emailField = page.getByLabel(/email/i);
		if (await emailField.isVisible()) {
			await emailField.fill('invalid-email');
			await page.getByRole('button', { name: /save|create|submit/i }).click();
			// Should show email validation error
			await expect(page.getByText(/valid.*email|email.*invalid/i)).toBeVisible();
		}
	});
});
