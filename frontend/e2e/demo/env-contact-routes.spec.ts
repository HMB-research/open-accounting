import { test, expect } from '@playwright/test';
import { loginAsDemoEnv, navigateToEnvPage, prepareDemoEnvSession, waitForVisibleContent } from './env-utils';

test.describe('Demo Environment - Contacts', () => {
	test('Can navigate to contacts page', async ({ page }, testInfo) => {
		await loginAsDemoEnv(page, testInfo);

		const contactsLink = page.getByRole('link', { name: /contact|customer|client/i }).first();
		const hasLink = await contactsLink.isVisible().catch(() => false);

		if (hasLink) {
			await contactsLink.click();
			await waitForVisibleContent(page);
			await expect(page).toHaveURL(/contact/);
		} else {
			await navigateToEnvPage(page, '/contacts', testInfo);
			await expect(page).toHaveURL(/contact/);
		}
	});

	test('Contacts list displays', async ({ page }, testInfo) => {
		await prepareDemoEnvSession(page, testInfo);
		await navigateToEnvPage(page, '/contacts', testInfo);

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();
	});
});
