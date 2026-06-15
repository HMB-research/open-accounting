import { test, expect } from '@playwright/test';
import { addCostCenterButton, setupCostCentersPage } from './cost-centers-utils';

test.describe('Demo Cost Centers - Page Structure', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupCostCentersPage(page, testInfo);
	});

	test('renders cost center list and allocation controls', async ({ page }) => {
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
		await expect(addCostCenterButton(page)).toBeVisible({ timeout: 10000 });

		const hasEmptyState = await page
			.getByRole('heading', { name: /no cost centers yet|kulukohti pole veel/i })
			.isVisible()
			.catch(() => false);
		const hasTable = await page.locator('table.table').first().isVisible().catch(() => false);
		expect(hasEmptyState || hasTable).toBeTruthy();

		await expect(
			page.getByRole('heading', {
				name: /cost allocation assignments|kulukohade jaotused/i
			})
		).toBeVisible({ timeout: 10000 });
		await expect(page.locator('#allocationJournalLine')).toBeVisible({ timeout: 10000 });
		await expect(page.locator('#allocationCostCenter')).toBeVisible({ timeout: 10000 });
		await expect(page.locator('#allocationAmount')).toBeVisible({ timeout: 10000 });
		await expect(page.getByRole('button', { name: /create allocation|loo jaotus/i })).toBeVisible({
			timeout: 10000
		});
	});
});
