import { test, expect } from '@playwright/test';
import {
	categoriesTab,
	filterCategory,
	filterSearch,
	filterStatus,
	filterType,
	productRow,
	productsTab,
	setupInventoryPage,
	warehousesTab
} from './inventory-utils';

test.describe('Demo Inventory - Page Structure', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupInventoryPage(page, testInfo);
	});

	test('renders product list, filters, warehouses, and categories', async ({ page }) => {
		await expect(page.getByRole('heading', { name: 'Inventory Management' })).toBeVisible();
		await expect(page.getByRole('button', { name: 'New Product' })).toBeVisible();
		await expect(productsTab(page)).toHaveClass(/active/);
		await expect(warehousesTab(page)).toBeVisible();
		await expect(categoriesTab(page)).toBeVisible();

		await expect(filterSearch(page)).toBeVisible();
		await expect(filterType(page)).toContainText('Filter by type');
		await expect(filterStatus(page)).toContainText('Filter by status');
		await expect(filterCategory(page)).toContainText('Filter by category');
		await expect(page.getByLabel('Show low stock only')).toBeVisible();

		const laptopRow = productRow(page, /PROD-001.*Laptop Stand/);
		await expect(laptopRow).toBeVisible();
		await expect(laptopRow).toContainText('Goods');
		await expect(laptopRow).toContainText('Electronics');
		await expect(laptopRow).toContainText(/49[,.]99/);
		await expect(laptopRow.getByRole('button', { name: 'Adjust Stock' })).toBeVisible();
		await expect(laptopRow.getByRole('button', { name: 'Transfer Stock' })).toBeVisible();
		await expect(laptopRow.getByRole('button', { name: 'Movements' })).toBeVisible();
		await expect(laptopRow.getByRole('button', { name: 'Delete' })).toBeVisible();
		await expect(productRow(page, /SVC-001.*IT Support.*Service/)).toBeVisible();

		await warehousesTab(page).click();
		await expect(warehousesTab(page)).toHaveClass(/active/);
		await expect(page.getByRole('button', { name: 'New Warehouse' })).toBeVisible();
		await expect(page.getByRole('columnheader', { name: 'Warehouse Code' })).toBeVisible();
		await expect(page.getByRole('row', { name: /WH-MAIN.*Main Warehouse.*Yes.*Active/ })).toBeVisible();
		await expect(page.getByRole('row', { name: /WH-BACKUP.*Backup Storage/ })).toBeVisible();

		await categoriesTab(page).click();
		await expect(categoriesTab(page)).toHaveClass(/active/);
		await expect(page.getByRole('button', { name: 'New Category' })).toBeVisible();
		await expect(page.getByRole('row', { name: /Electronics.*Electronic devices and components/ })).toBeVisible();
		await expect(page.getByRole('row', { name: /Office Supplies.*General office supplies and stationery/ })).toBeVisible();
		await expect(page.getByRole('row', { name: /Services.*Professional services and consulting/ })).toBeVisible();
	});
});
