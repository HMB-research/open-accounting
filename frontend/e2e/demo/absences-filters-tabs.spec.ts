import { test, expect } from '@playwright/test';
import { absenceTabs, setupAbsencesPage, switchAbsenceTab } from './absences-utils';

test.describe('Demo Leave Management - Filters And Tabs', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupAbsencesPage(page, testInfo);
	});

	test('can switch between records and balances tabs', async ({ page }) => {
		await expect(page.locator('.tabs')).toBeVisible({ timeout: 10000 });

		const tabs = absenceTabs(page);
		await expect(async () => {
			const count = await tabs.count();
			expect(count).toBeGreaterThanOrEqual(2);
		}).toPass({ timeout: 5000 });
		await switchAbsenceTab(page, 1);
		await switchAbsenceTab(page, 0);
	});
});
