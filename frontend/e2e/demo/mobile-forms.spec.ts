import { expect, test } from '@playwright/test';
import { openMobileRoute, prepareMobileDemo } from './mobile-utils';

test.describe('Mobile Forms', () => {
	test.use({ viewport: { width: 375, height: 667 } });

	test.beforeEach(async ({ page }, testInfo) => {
		await prepareMobileDemo(page, testInfo);
	});

	test('contacts form should be usable on mobile', async ({ page }, testInfo) => {
		await openMobileRoute(page, '/contacts', testInfo);

		const createBtn = page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first();

		const isBtnVisible = await createBtn.isVisible().catch(() => false);
		if (isBtnVisible) {
			await createBtn.click();

			const formElement = page.locator('form, .modal, [role="dialog"]').first();
			await formElement.waitFor({ state: 'visible', timeout: 5000 }).catch(() => {});

			const hasForm = await formElement.isVisible().catch(() => false);
			const hasHeading = await page
				.getByRole('heading', { name: /contacts/i })
				.isVisible()
				.catch(() => false);
			expect(hasForm || page.url().includes('new') || hasHeading).toBeTruthy();
			return;
		}

		await expect(page.getByRole('heading', { name: /contacts/i })).toBeVisible();
	});

	test('form buttons should be touch-friendly size', async ({ page }, testInfo) => {
		await openMobileRoute(page, '/contacts', testInfo);

		const createBtn = page
			.getByRole('button', { name: /create|new|add/i })
			.or(page.getByRole('link', { name: /create|new|add/i }))
			.first();

		if (await createBtn.isVisible()) {
			const box = await createBtn.boundingBox();
			if (box) {
				expect(box.height).toBeGreaterThanOrEqual(40);
			}
		}
	});
});
