import { test, expect } from '@playwright/test';
import { openCostCenterModal, setupCostCentersPage } from './cost-centers-utils';

test.describe('Demo Cost Centers - Create Modal', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupCostCentersPage(page, testInfo);
	});

	test('opens create modal with form fields and closes it', async ({ page }) => {
		const modal = await openCostCenterModal(page);

		await expect(modal.locator('#code')).toBeVisible({ timeout: 5000 });
		await expect(modal.locator('#name')).toBeVisible();
		await expect(modal.locator('#description')).toBeVisible();
		await expect(modal.locator('#budgetAmount')).toBeVisible();
		await expect(modal.locator('#budgetPeriod')).toBeVisible();
		await expect(modal.locator('#isActive')).toBeVisible();
		await expect(modal.locator('input').first()).toBeVisible();

		await modal.getByRole('button', { name: /cancel|tühista/i }).click();
		await expect(modal).not.toBeVisible({ timeout: 5000 });
	});
});
