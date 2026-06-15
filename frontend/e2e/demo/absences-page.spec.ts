import { test, expect } from '@playwright/test';
import { absenceTabs, requestLeaveButton, setupAbsencesPage } from './absences-utils';

test.describe('Demo Leave Management - Page Structure Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupAbsencesPage(page, testInfo);
	});

	test('renders leave management shell with filters and records content', async ({ page }) => {
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
		await expect(requestLeaveButton(page)).toBeVisible({ timeout: 10000 });

		const yearDropdown = page.locator('#yearFilter');
		await expect(yearDropdown).toBeVisible({ timeout: 10000 });

		const currentYear = new Date().getFullYear();
		await expect(yearDropdown.locator(`option[value="${currentYear}"]`)).toBeAttached();

		const employeeDropdown = page.locator('#employeeFilter');
		await expect(employeeDropdown).toBeVisible({ timeout: 10000 });
		await expect(employeeDropdown.locator('option').first()).toBeAttached();

		await expect(page.locator('.tabs')).toBeVisible({ timeout: 10000 });
		await expect(async () => {
			const count = await absenceTabs(page).count();
			expect(count).toBeGreaterThanOrEqual(2);
		}).toPass({ timeout: 5000 });

		const routeContent = page.locator('table.table, .empty-state, .alert-error').first();
		await expect(routeContent).toBeVisible({ timeout: 10000 });
	});
});
