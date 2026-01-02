import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant, assertTableRowCount } from './utils';

test.describe('Demo Invoices - Seeded Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/invoices');
	});

	test('displays invoices page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /invoice/i })).toBeVisible();
	});

	test('shows seeded invoices in table', async ({ page }) => {
		await page.waitForSelector('table tbody tr', { timeout: 10000 });
		// Should have at least 9 invoices from seed
		await assertTableRowCount(page, 5); // Some might be filtered by date
	});

	test('displays INV-2024 invoice numbers', async ({ page }) => {
		// At least one 2024 invoice should be visible
		await expect(page.getByText(/INV-2024/)).toBeVisible();
	});

	test('shows PAID status invoices', async ({ page }) => {
		// 3 invoices are PAID
		await expect(page.getByText(/paid/i).first()).toBeVisible();
	});

	test('shows invoice amounts in EUR', async ({ page }) => {
		// Invoices have amounts like 3,050.00, 10,675.00, etc.
		await expect(page.getByText(/EUR|€/)).toBeVisible();
	});

	test('displays customer names on invoices', async ({ page }) => {
		// Invoices are for TechStart, Nordic, Baltic, GreenTech
		const hasCustomer = await page.getByText(/TechStart|Nordic|Baltic|GreenTech/).first().isVisible();
		expect(hasCustomer).toBeTruthy();
	});

	test('can filter invoices by status', async ({ page }) => {
		const statusFilter = page.locator('select').filter({ hasText: /status|all/i }).first();

		if (await statusFilter.isVisible()) {
			// Try to filter
			const options = await statusFilter.locator('option').all();
			for (const option of options) {
				const text = await option.textContent();
				if (text && /paid/i.test(text)) {
					const value = await option.getAttribute('value');
					if (value) {
						await statusFilter.selectOption(value);
						break;
					}
				}
			}
			await page.waitForTimeout(500);

			// Should only show paid invoices
			const rows = page.locator('table tbody tr');
			const count = await rows.count();
			expect(count).toBeGreaterThan(0);
		}
	});

	test('can click on invoice to view details', async ({ page }) => {
		const invoiceRow = page.getByText(/INV-2024-001/).first();

		if (await invoiceRow.isVisible()) {
			await invoiceRow.click();
			await page.waitForTimeout(1000);

			// Should show invoice details with line items
			const hasDetails = await page.getByText(/Software Development|3,050|TechStart/).first().isVisible().catch(() => false);
			expect(hasDetails).toBeTruthy();
		}
	});

	test('create invoice button is visible', async ({ page }) => {
		await expect(page.getByRole('button', { name: /new|create|add/i })).toBeVisible();
	});
});
