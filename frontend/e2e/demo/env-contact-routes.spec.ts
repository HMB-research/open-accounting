import { test, expect } from '@playwright/test';
import { loginAsDemoEnv, navigateToEnvPage, waitForVisibleContent } from './env-utils';

test.describe('Demo Environment - Contacts', () => {
	test('navigates to contacts and displays the contacts list', async ({ page }, testInfo) => {
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

		await navigateToEnvPage(page, '/contacts', testInfo);
		await expect(page).toHaveURL(/contact/);

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();
	});
});
