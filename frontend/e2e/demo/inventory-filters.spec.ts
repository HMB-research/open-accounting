import { test, expect } from '@playwright/test';
import {
	filterCategory,
	filterSearch,
	filterType,
	productRow,
	setupInventoryPage,
	waitForProductsReload
} from './inventory-utils';

test.describe('Demo Inventory - Filters', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupInventoryPage(page, testInfo);
	});

	test('filters products by search, type, and category', async ({ page }) => {
		await waitForProductsReload(
			page,
			async () => {
				await filterSearch(page).fill('USB-C');
				await filterSearch(page).dispatchEvent('change');
			},
			{ search: 'USB-C' }
		);
		await expect(productRow(page, /PROD-002.*USB-C Hub/)).toBeVisible();
		await expect(productRow(page, /PROD-001.*Laptop Stand/)).toBeHidden();

		await waitForProductsReload(
			page,
			async () => {
				await filterSearch(page).fill('');
				await filterSearch(page).dispatchEvent('change');
			}
		);

		await waitForProductsReload(
			page,
			async () => {
				await filterType(page).selectOption('SERVICE');
			},
			{ product_type: 'SERVICE' }
		);
		await expect(productRow(page, /SVC-001.*IT Support.*Service/)).toBeVisible();
		await expect(productRow(page, /PROD-001.*Laptop Stand/)).toBeHidden();

		await waitForProductsReload(
			page,
			async () => {
				await filterType(page).selectOption('');
			}
		);

		const officeSuppliesCategoryId = await filterCategory(page)
			.locator('option', { hasText: 'Office Supplies' })
			.getAttribute('value');
		expect(officeSuppliesCategoryId).toBeTruthy();

		await waitForProductsReload(
			page,
			async () => {
				await filterCategory(page).selectOption(officeSuppliesCategoryId ?? '');
			},
			{ category_id: officeSuppliesCategoryId ?? '' }
		);
		await expect(productRow(page, /PROD-004.*Notebook A5/)).toBeVisible();
		await expect(productRow(page, /PROD-005.*Pen Set/)).toBeVisible();
		await expect(productRow(page, /PROD-001.*Laptop Stand/)).toBeHidden();
	});
});
