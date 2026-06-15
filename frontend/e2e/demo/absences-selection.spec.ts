import { test, expect } from '@playwright/test';
import { absenceTabs, setupAbsencesPage, switchAbsenceTab } from './absences-utils';

test.describe('Demo Leave Management - Employee Selection', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupAbsencesPage(page, testInfo);
	});

	test('shows employee options and balances empty state without selected employee', async ({ page }) => {
		const employeeDropdown = page.locator('#employeeFilter');
		await expect(employeeDropdown).toBeVisible({ timeout: 10000 });

		const options = await employeeDropdown.locator('option').allTextContents();
		expect(options.length).toBeGreaterThanOrEqual(1);
		await switchAbsenceTab(page, 1);

		const tabs = absenceTabs(page);
		await expect(tabs.nth(1)).toHaveClass(/active/);

		await expect(async () => {
			const needsEmployee = await page
				.getByText(/select.*employee|please select/i)
				.isVisible()
				.catch(() => false);
			const hasEmptyState = await page.locator('.empty-state').isVisible().catch(() => false);

			expect(needsEmployee || hasEmptyState).toBeTruthy();
		}).toPass({ timeout: 5000 });
	});
});
