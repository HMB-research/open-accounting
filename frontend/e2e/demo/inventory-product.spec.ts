import { test, expect } from '@playwright/test';
import { productRow, setupInventoryPage } from './inventory-utils';

test.describe('Demo Inventory - Product Modal', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupInventoryPage(page, testInfo);
	});

	test('creates and deletes a product through the UI', async ({ page }, testInfo) => {
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

		const newProductRow = productRow(page, new RegExp(`${productCode}.*${productName}`));
		await expect(newProductRow).toBeVisible();
		await expect(newProductRow).toContainText('Goods');
		await expect(newProductRow).toContainText('Electronics');

		page.once('dialog', async (confirmDialog) => {
			await confirmDialog.accept();
		});
		await newProductRow.getByRole('button', { name: 'Delete' }).click();
		await expect(newProductRow).toBeHidden();
	});
});
