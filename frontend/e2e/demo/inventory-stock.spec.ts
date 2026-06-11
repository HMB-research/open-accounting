import { test, expect } from '@playwright/test';
import {
	createInventoryProduct,
	listWarehouses,
	openInventoryPage,
	productRow,
	setupInventoryPage,
	waitForInventoryPageReady
} from './inventory-utils';

test.describe('Demo Inventory - Stock Workflows', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupInventoryPage(page, testInfo);
	});

	test('records lot metadata and transfers stock for a test-owned product', async ({ page }, testInfo) => {
		const suffix = `${testInfo.parallelIndex}-${testInfo.retry}-${Date.now().toString(36)}`;
		const product = await createInventoryProduct(page, testInfo, suffix);
		const warehouses = await listWarehouses(page, testInfo);
		const sourceWarehouse = warehouses.find((warehouse) => warehouse.name === 'Main Warehouse') ?? warehouses[0];
		const destinationWarehouse =
			warehouses.find((warehouse) => warehouse.name === 'Backup Storage') ?? warehouses[1];

		await openInventoryPage(page, testInfo);
		const row = productRow(page, new RegExp(`${product.code}.*${product.name}`));
		await expect(row).toBeVisible();

		const lotNumber = `LOT-E2E-${suffix}`;
		const serialNumber = `SN-E2E-${suffix}`;

		await row.getByRole('button', { name: 'Adjust Stock' }).click();
		const adjustDialog = page.getByRole('dialog', { name: /Adjust Stock:/ });
		await expect(adjustDialog).toBeVisible();
		await expect(adjustDialog.getByLabel('Lot Number')).toBeVisible();
		await expect(adjustDialog.getByLabel('Serial Number')).toBeVisible();
		await expect(adjustDialog.getByLabel('Expiry Date')).toBeVisible();

		await adjustDialog.getByLabel('Warehouses *').selectOption(sourceWarehouse.id);
		await adjustDialog.getByLabel(/Quantity/).fill('3');
		await adjustDialog.getByLabel('Unit Cost').fill('10');
		await adjustDialog.getByLabel('Lot Number').fill(lotNumber);
		await adjustDialog.getByLabel('Serial Number').fill(serialNumber);
		await adjustDialog.getByLabel('Expiry Date').fill('2027-05-30');
		await adjustDialog.getByLabel('Reason').fill(`E2E metadata check ${suffix}`);
		await adjustDialog.getByRole('button', { name: 'Adjust Stock' }).click();
		await expect(adjustDialog).toBeHidden();
		await waitForInventoryPageReady(page);

		await row.getByRole('button', { name: 'Movements' }).click();
		const movementsDialog = page.getByRole('dialog', { name: new RegExp(`Movements: ${product.name}`) });
		await expect(movementsDialog).toBeVisible();
		await expect(movementsDialog.getByRole('columnheader', { name: 'Lot Number' })).toBeVisible();
		await expect(movementsDialog.getByRole('columnheader', { name: 'Serial Number' })).toBeVisible();
		await expect(movementsDialog.getByRole('columnheader', { name: 'Expiry Date' })).toBeVisible();
		await expect(movementsDialog.getByText(lotNumber)).toBeVisible();
		await expect(movementsDialog.getByText(serialNumber)).toBeVisible();
		await expect(movementsDialog.getByText(/30\.0?5\.2027/)).toBeVisible();
		await movementsDialog.getByRole('button', { name: 'Close' }).click();
		await expect(movementsDialog).toBeHidden();

		await row.getByRole('button', { name: 'Transfer Stock' }).click();
		const transferDialog = page.getByRole('dialog', {
			name: new RegExp(`Transfer Stock: ${product.name}`)
		});
		await expect(transferDialog).toBeVisible();
		await transferDialog.getByLabel('From Warehouse *').selectOption(sourceWarehouse.id);
		await transferDialog.getByLabel('To Warehouse *').selectOption(destinationWarehouse.id);
		await transferDialog.getByLabel('Quantity *').fill('1');
		await transferDialog.getByLabel('Notes').fill(`E2E transfer ${suffix}`);
		await transferDialog.getByRole('button', { name: 'Transfer Stock' }).click();
		await expect(transferDialog).toBeHidden();
		await waitForInventoryPageReady(page);

		await row.getByRole('button', { name: 'Movements' }).click();
		await expect(movementsDialog).toBeVisible();
		await expect(movementsDialog.getByText('Transfer').first()).toBeVisible();
		await expect(movementsDialog.getByText(sourceWarehouse.name).first()).toBeVisible();
		await expect(movementsDialog.getByText(destinationWarehouse.name).first()).toBeVisible();
		await movementsDialog.getByRole('button', { name: 'Close' }).click();
		await expect(movementsDialog).toBeHidden();
	});
});
