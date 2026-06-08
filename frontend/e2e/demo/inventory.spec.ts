import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from './utils';

async function openInventory(page: Parameters<typeof navigateTo>[0], testInfo: Parameters<typeof navigateTo>[2]) {
	await navigateTo(page, '/inventory', testInfo);
	await expect(page.getByRole('heading', { name: 'Inventory Management' })).toBeVisible();
	await expect(page.getByRole('button', { name: 'New Product' })).toBeVisible();
}

async function waitForProductsReload(page: Parameters<typeof navigateTo>[0], action: () => Promise<void>) {
	const response = page.waitForResponse((res) =>
		res.request().method() === 'GET' && res.url().includes('/products')
	).catch(() => null);
	await action();
	await response;
}

test.describe('Inventory View', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('displays seeded products and inventory controls', async ({ page }, testInfo) => {
		await openInventory(page, testInfo);

		await expect(page.getByRole('button', { name: 'Products' })).toHaveClass(/active/);
		await expect(page.getByRole('button', { name: 'Warehouses' })).toBeVisible();
		await expect(page.getByRole('button', { name: 'Product Categories' })).toBeVisible();

		await expect(page.getByPlaceholder('Search')).toBeVisible();
		await expect(page.locator('.filters select').nth(0)).toContainText('Filter by type');
		await expect(page.locator('.filters select').nth(1)).toContainText('Filter by status');
		await expect(page.locator('.filters select').nth(2)).toContainText('Filter by category');
		await expect(page.getByLabel('Show low stock only')).toBeVisible();

		const laptopRow = page.getByRole('row', { name: /PROD-001.*Laptop Stand/ });
		await expect(laptopRow).toBeVisible();
		await expect(laptopRow).toContainText('Goods');
		await expect(laptopRow).toContainText('Electronics');
		await expect(laptopRow).toContainText(/49[,.]99/);
		await expect(laptopRow.getByRole('button', { name: 'Adjust Stock' })).toBeVisible();
		await expect(laptopRow.getByRole('button', { name: 'Transfer Stock' })).toBeVisible();
		await expect(laptopRow.getByRole('button', { name: 'Movements' })).toBeVisible();
		await expect(laptopRow.getByRole('button', { name: 'Delete' })).toBeVisible();

		await expect(page.getByRole('row', { name: /SVC-001.*IT Support.*Service/ })).toBeVisible();
	});

	test('filters products by search, type, and category', async ({ page }, testInfo) => {
		await openInventory(page, testInfo);

		const search = page.getByPlaceholder('Search');
		await waitForProductsReload(page, async () => {
			await search.fill('USB-C');
			await search.dispatchEvent('change');
		});
		await expect(page.getByRole('row', { name: /PROD-002.*USB-C Hub/ })).toBeVisible();
		await expect(page.getByRole('row', { name: /PROD-001.*Laptop Stand/ })).toBeHidden();

		await waitForProductsReload(page, async () => {
			await search.fill('');
			await search.dispatchEvent('change');
		});

		await waitForProductsReload(page, async () => {
			await page.locator('.filters select').nth(0).selectOption('SERVICE');
		});
		await expect(page.getByRole('row', { name: /SVC-001.*IT Support.*Service/ })).toBeVisible();
		await expect(page.getByRole('row', { name: /PROD-001.*Laptop Stand/ })).toBeHidden();

		await waitForProductsReload(page, async () => {
			await page.locator('.filters select').nth(0).selectOption('');
			await page.locator('.filters select').nth(2).selectOption({ label: 'Office Supplies' });
		});
		await expect(page.getByRole('row', { name: /PROD-004.*Notebook A5/ })).toBeVisible();
		await expect(page.getByRole('row', { name: /PROD-005.*Pen Set/ })).toBeVisible();
		await expect(page.getByRole('row', { name: /PROD-001.*Laptop Stand/ })).toBeHidden();
	});

	test('shows warehouse and category tabs with seeded data', async ({ page }, testInfo) => {
		await openInventory(page, testInfo);

		await page.getByRole('button', { name: 'Warehouses' }).click();
		await expect(page.getByRole('button', { name: 'Warehouses' })).toHaveClass(/active/);
		await expect(page.getByRole('button', { name: 'New Warehouse' })).toBeVisible();
		await expect(page.getByRole('columnheader', { name: 'Warehouse Code' })).toBeVisible();
		await expect(page.getByRole('row', { name: /WH-MAIN.*Main Warehouse.*Yes.*Active/ })).toBeVisible();
		await expect(page.getByRole('row', { name: /WH-BACKUP.*Backup Storage/ })).toBeVisible();

		await page.getByRole('button', { name: 'Product Categories' }).click();
		await expect(page.getByRole('button', { name: 'Product Categories' })).toHaveClass(/active/);
		await expect(page.getByRole('button', { name: 'New Category' })).toBeVisible();
		await expect(page.getByRole('row', { name: /Electronics.*Electronic devices and components/ })).toBeVisible();
		await expect(page.getByRole('row', { name: /Office Supplies.*General office supplies and stationery/ })).toBeVisible();
		await expect(page.getByRole('row', { name: /Services.*Professional services and consulting/ })).toBeVisible();
	});

	test('creates and deletes a product through the UI', async ({ page }, testInfo) => {
		await openInventory(page, testInfo);

		const suffix = `${testInfo.parallelIndex}-${testInfo.retry}-${Date.now().toString(36)}`;
		const productCode = `E2E-P-${suffix}`;
		const productName = `E2E Product ${suffix}`;

		await page.getByRole('button', { name: 'New Product' }).click();
		const dialog = page.getByRole('dialog', { name: 'New Product' });
		await expect(dialog).toBeVisible();

		await dialog.getByLabel(/Product Name/).fill(productName);
		await dialog.locator('#product-code').fill(productCode);
		await dialog.getByLabel('Product Type').selectOption('GOODS');
		await dialog.getByLabel('Category').selectOption({ label: 'Electronics' });
		await dialog.getByLabel('Unit').fill('pcs');
		await dialog.getByLabel('Purchase Price').fill('12.50');
		await dialog.getByLabel(/Sales Price/).fill('29.90');
		await dialog.getByLabel(/VAT Rate/).fill('22');
		await dialog.getByLabel('Min Stock Level').fill('2');
		await dialog.getByLabel('Reorder Point').fill('4');
		await dialog.getByRole('button', { name: 'Create Product' }).click();
		await expect(dialog).toBeHidden();

		const newProductRow = page.getByRole('row', { name: new RegExp(`${productCode}.*${productName}`) });
		await expect(newProductRow).toBeVisible();
		await expect(newProductRow).toContainText('Goods');
		await expect(newProductRow).toContainText('Electronics');

		page.once('dialog', async (confirmDialog) => {
			await confirmDialog.accept();
		});
		await newProductRow.getByRole('button', { name: 'Delete' }).click();
		await expect(newProductRow).toBeHidden();
	});

	test('transfers stock between warehouses and records a movement', async ({ page }, testInfo) => {
		await openInventory(page, testInfo);

		const productRow = page.getByRole('row', { name: /PROD-001.*Laptop Stand/ });
		await productRow.getByRole('button', { name: 'Transfer Stock' }).click();

		const transferDialog = page.getByRole('dialog', { name: /Transfer Stock: Laptop Stand/ });
		await expect(transferDialog).toBeVisible();
		await expect(transferDialog.getByLabel('From Warehouse *')).toContainText('Main Warehouse');
		await expect(transferDialog.getByLabel('To Warehouse *')).toContainText('Backup Storage');

		await transferDialog.getByLabel('Quantity *').fill('1');
		await transferDialog.getByLabel('Notes').fill(`E2E transfer ${testInfo.parallelIndex}-${testInfo.retry}`);
		await transferDialog.getByRole('button', { name: 'Transfer Stock' }).click();
		await expect(transferDialog).toBeHidden();

		await productRow.getByRole('button', { name: 'Movements' }).click();
		const movementsDialog = page.getByRole('dialog', { name: /Movements: Laptop Stand/ });
		await expect(movementsDialog).toBeVisible();
		await expect(movementsDialog.getByText('Transfer').first()).toBeVisible();
		await expect(movementsDialog.getByText('Main Warehouse').first()).toBeVisible();
		await expect(movementsDialog.getByText('Backup Storage').first()).toBeVisible();

		await movementsDialog.getByRole('button', { name: 'Close' }).click();
		await expect(movementsDialog).toBeHidden();
	});

	test('records and displays stock lot metadata', async ({ page }, testInfo) => {
		await openInventory(page, testInfo);

		const suffix = `${testInfo.parallelIndex}-${testInfo.retry}-${Date.now().toString(36)}`;
		const lotNumber = `LOT-E2E-${suffix}`;
		const serialNumber = `SN-E2E-${suffix}`;

		const productRow = page.getByRole('row', { name: /PROD-001/ });
		await expect(productRow).toBeVisible();

		await productRow.getByRole('button', { name: 'Adjust Stock' }).click();

		const adjustDialog = page.getByRole('dialog', { name: /Adjust Stock:/ });
		await expect(adjustDialog).toBeVisible();
		await expect(adjustDialog.getByLabel('Lot Number')).toBeVisible();
		await expect(adjustDialog.getByLabel('Serial Number')).toBeVisible();
		await expect(adjustDialog.getByLabel('Expiry Date')).toBeVisible();

		await adjustDialog.getByLabel(/Quantity/).fill('1');
		await adjustDialog.getByLabel('Unit Cost').fill('10');
		await adjustDialog.getByLabel('Lot Number').fill(lotNumber);
		await adjustDialog.getByLabel('Serial Number').fill(serialNumber);
		await adjustDialog.getByLabel('Expiry Date').fill('2027-05-30');
		await adjustDialog.getByLabel('Reason').fill('E2E metadata check');
		await adjustDialog.getByRole('button', { name: 'Adjust Stock' }).click();
		await expect(adjustDialog).toBeHidden();

		await productRow.getByRole('button', { name: 'Movements' }).click();

		const movementsDialog = page.getByRole('dialog', { name: /Movements:/ });
		await expect(movementsDialog).toBeVisible();
		await expect(movementsDialog.getByRole('columnheader', { name: 'Lot Number' })).toBeVisible();
		await expect(movementsDialog.getByRole('columnheader', { name: 'Serial Number' })).toBeVisible();
		await expect(movementsDialog.getByRole('columnheader', { name: 'Expiry Date' })).toBeVisible();
		await expect(movementsDialog.getByText(lotNumber)).toBeVisible();
		await expect(movementsDialog.getByText(serialNumber)).toBeVisible();
		await expect(movementsDialog.getByText(/30\.0?5\.2027/)).toBeVisible();

		await movementsDialog.getByRole('button', { name: 'Close' }).click();
		await expect(movementsDialog).toBeHidden();
	});
});
