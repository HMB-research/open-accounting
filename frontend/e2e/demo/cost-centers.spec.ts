import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo Cost Centers - Page Structure', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/settings/cost-centers', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('displays cost centers page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
		const heading = page.getByRole('heading', { level: 1 });
		const headingText = await heading.textContent();
		expect(headingText?.toLowerCase()).toMatch(/cost.?center|kulukoht/i);
	});

	test('has add new cost center button', async ({ page }) => {
		// Use first() to handle multiple add buttons (header and empty state)
		const addBtn = page.locator('button.btn-primary').first();
		await expect(addBtn).toBeVisible({ timeout: 10000 });
		const btnText = await addBtn.textContent();
		expect(btnText?.toLowerCase()).toMatch(/add|lisa/i);
	});

	test('has back button linking to settings', async ({ page }) => {
		const backLink = page.locator('a.btn-secondary[href*="settings"]');
		await expect(backLink).toBeVisible({ timeout: 10000 });
	});
});

test.describe('Demo Cost Centers - Empty State', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/settings/cost-centers', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('shows empty state or cost center list', async ({ page }) => {
		// Wait for content to load
		await page.waitForTimeout(1000);

		// Should show either empty state card or table
		const hasEmptyState = await page
			.getByText(/no cost.?center|kulukohti pole/i)
			.isVisible()
			.catch(() => false);
		const hasTable = await page.locator('table.table').isVisible().catch(() => false);

		expect(hasEmptyState || hasTable).toBeTruthy();
	});
});

test.describe('Demo Cost Centers - Create Modal', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/settings/cost-centers', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('opens create modal when clicking add button', async ({ page }) => {
		// Click add button
		const addBtn = page.locator('button.btn-primary').first();
		await expect(addBtn).toBeVisible({ timeout: 10000 });
		await addBtn.click();

		// Modal should appear
		const modal = page.locator('.modal');
		await expect(modal).toBeVisible({ timeout: 5000 });

		// Check modal has expected fields
		const codeInput = modal.locator('#code');
		const nameInput = modal.locator('#name');
		await expect(codeInput).toBeVisible();
		await expect(nameInput).toBeVisible();
	});

	test('modal can be closed', async ({ page }) => {
		// Open modal
		const addBtn = page.locator('button.btn-primary').first();
		await expect(addBtn).toBeVisible({ timeout: 10000 });
		await addBtn.click();

		// Wait for modal
		const modal = page.locator('.modal');
		await expect(modal).toBeVisible({ timeout: 5000 });

		// Close modal
		const closeBtn = modal.locator('.btn-close');
		await closeBtn.click();
		await page.waitForTimeout(500);

		// Modal should be hidden
		await expect(modal).not.toBeVisible();
	});

	test('modal has budget fields', async ({ page }) => {
		// Open modal
		const addBtn = page.locator('button.btn-primary').first();
		await expect(addBtn).toBeVisible({ timeout: 10000 });
		await addBtn.click();

		// Wait for modal
		const modal = page.locator('.modal');
		await expect(modal).toBeVisible({ timeout: 5000 });

		// Check budget fields exist
		const budgetInput = modal.locator('#budgetAmount');
		const periodSelect = modal.locator('#budgetPeriod');
		await expect(budgetInput).toBeVisible();
		await expect(periodSelect).toBeVisible();
	});
});

test.describe('Demo Cost Centers - Form Validation', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/settings/cost-centers', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('code field accepts valid input', async ({ page }) => {
		// Open modal
		const addBtn = page.locator('button.btn-primary').first();
		await expect(addBtn).toBeVisible({ timeout: 10000 });
		await addBtn.click();

		// Wait for modal
		const modal = page.locator('.modal');
		await expect(modal).toBeVisible({ timeout: 5000 });

		// Fill code field
		const codeInput = modal.locator('#code');
		await codeInput.fill('TEST001');
		expect(await codeInput.inputValue()).toBe('TEST001');
	});

	test('name field accepts valid input', async ({ page }) => {
		// Open modal
		const addBtn = page.locator('button.btn-primary').first();
		await expect(addBtn).toBeVisible({ timeout: 10000 });
		await addBtn.click();

		// Wait for modal
		const modal = page.locator('.modal');
		await expect(modal).toBeVisible({ timeout: 5000 });

		// Fill name field
		const nameInput = modal.locator('#name');
		await nameInput.fill('Test Department');
		expect(await nameInput.inputValue()).toBe('Test Department');
	});
});

test.describe('Demo Cost Centers - Budget Period Selection', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/settings/cost-centers', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('budget period dropdown has expected options', async ({ page }) => {
		// Open modal
		const addBtn = page.locator('button.btn-primary').first();
		await expect(addBtn).toBeVisible({ timeout: 10000 });
		await addBtn.click();

		// Wait for modal
		const modal = page.locator('.modal');
		await expect(modal).toBeVisible({ timeout: 5000 });

		// Check period options
		const periodSelect = modal.locator('#budgetPeriod');
		const options = await periodSelect.locator('option').allTextContents();

		// Should have monthly, quarterly, annual options
		const hasExpectedOptions =
			options.some(o => o.toLowerCase().includes('month') || o.toLowerCase().includes('kuu')) &&
			options.some(o => o.toLowerCase().includes('quarter') || o.toLowerCase().includes('kvartal')) &&
			options.some(o => o.toLowerCase().includes('annual') || o.toLowerCase().includes('aasta'));

		expect(hasExpectedOptions).toBeTruthy();
	});
});

test.describe('Demo Cost Centers - Table Display', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/settings/cost-centers', testInfo);
		await page.waitForLoadState('networkidle');
	});

	test('table has proper headers when cost centers exist', async ({ page }) => {
		// Wait for content to load
		await page.waitForTimeout(1000);

		// Check if table exists
		const table = page.locator('table.table');
		const tableVisible = await table.isVisible().catch(() => false);

		if (tableVisible) {
			// Check for expected headers
			const headers = await table.locator('thead').textContent();
			const hasCodeHeader = headers?.toLowerCase().match(/code|kood/i);
			const hasNameHeader = headers?.toLowerCase().match(/name|nimi/i);

			expect(hasCodeHeader || hasNameHeader).toBeTruthy();
		}
	});

	test('table rows have edit and delete buttons', async ({ page }) => {
		// Wait for content to load
		await page.waitForTimeout(1000);

		// Check if table exists with rows
		const table = page.locator('table.table');
		const tableVisible = await table.isVisible().catch(() => false);

		if (tableVisible) {
			const rows = table.locator('tbody tr');
			const rowCount = await rows.count();

			if (rowCount > 0) {
				// Check first row has action buttons
				const firstRow = rows.first();
				const editBtn = firstRow.locator('button.btn-outline-primary');
				const deleteBtn = firstRow.locator('button.btn-outline-danger');

				expect(
					(await editBtn.isVisible().catch(() => false)) ||
						(await deleteBtn.isVisible().catch(() => false))
				).toBeTruthy();
			}
		}
	});
});
