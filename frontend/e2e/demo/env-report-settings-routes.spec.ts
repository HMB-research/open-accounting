import { test, expect } from '@playwright/test';
import { loginAsDemoEnv, navigateToEnvPage, prepareDemoEnvSession, waitForVisibleContent } from './env-utils';

test.describe('Demo Environment - Reports', () => {
	test('Can navigate to reports page', async ({ page }, testInfo) => {
		await loginAsDemoEnv(page, testInfo);

		const reportsLink = page.getByRole('link', { name: /report/i }).first();
		const hasLink = await reportsLink.isVisible().catch(() => false);

		if (hasLink) {
			await reportsLink.click();
			await waitForVisibleContent(page);
			await expect(page).toHaveURL(/report/);
		} else {
			await navigateToEnvPage(page, '/reports', testInfo);
			await expect(page).toHaveURL(/report/);
		}
	});

	test('Reports page loads', async ({ page }, testInfo) => {
		await prepareDemoEnvSession(page, testInfo);
		await navigateToEnvPage(page, '/reports', testInfo);

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();
	});
});

test.describe('Demo Environment - Settings', () => {
	test('Can access settings', async ({ page }, testInfo) => {
		await loginAsDemoEnv(page, testInfo);

		const settingsLink = page.getByRole('link', { name: /setting/i }).first();
		const hasLink = await settingsLink.isVisible().catch(() => false);

		if (hasLink) {
			await settingsLink.click();
			await waitForVisibleContent(page);
			await expect(page).toHaveURL(/setting/);
		} else {
			await navigateToEnvPage(page, '/settings', testInfo);
			await expect(page).toHaveURL(/setting/);
		}
	});
});
