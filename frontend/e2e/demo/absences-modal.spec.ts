import { test, expect } from '@playwright/test';
import { openRequestLeaveModal, setupAbsencesPage } from './absences-utils';

test.describe('Demo Leave Management - Request Leave Modal', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupAbsencesPage(page, testInfo);
	});

	test('opens request modal with required fields and closes it', async ({ page }) => {
		const modal = await openRequestLeaveModal(page);

		await expect(modal.locator('#employee, select').first()).toBeVisible({ timeout: 5000 });
		await expect(modal.locator('#absenceType, select').nth(1)).toBeVisible({ timeout: 5000 });
		await expect(modal.locator('#startDate, input[type="date"]').first()).toBeVisible();

		await modal.getByRole('button', { name: /cancel|tühista/i }).click();
		await expect(modal).not.toBeVisible({ timeout: 5000 });
	});
});
