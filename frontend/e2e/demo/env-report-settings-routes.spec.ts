import { test, expect } from '@playwright/test';
import { loginAsDemoEnv, navigateToEnvPage, waitForVisibleContent } from './env-utils';

test.describe('Demo Environment - Reports and Settings', () => {
	test('navigates reports, verifies direct report load, and opens settings', async ({ page }, testInfo) => {
		await loginAsDemoEnv(page, testInfo);

		const reportsLink = page.getByRole('link', { name: /report/i }).first();
		const hasReportsLink = await reportsLink.isVisible().catch(() => false);

		if (hasReportsLink) {
			await reportsLink.click();
			await waitForVisibleContent(page);
			await expect(page).toHaveURL(/report/);
		} else {
			await navigateToEnvPage(page, '/reports', testInfo);
			await expect(page).toHaveURL(/report/);
		}

		await navigateToEnvPage(page, '/reports', testInfo);

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();

		const settingsLink = page.getByRole('link', { name: /setting/i }).first();
		const hasSettingsLink = await settingsLink.isVisible().catch(() => false);

		if (hasSettingsLink) {
			await settingsLink.click();
			await waitForVisibleContent(page);
			await expect(page).toHaveURL(/setting/);
		} else {
			await navigateToEnvPage(page, '/settings', testInfo);
			await expect(page).toHaveURL(/setting/);
		}
	});
});
