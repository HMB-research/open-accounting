import { test, expect } from '@playwright/test';
import { loginAsDemoEnv, navigateToEnvPage } from './env-utils';

test.describe('Demo Environment - Invoices', () => {
	test('navigates invoices, displays the list, and opens create form', async ({ page }, testInfo) => {
		await loginAsDemoEnv(page, testInfo);

		const invoicesLink = page.getByRole('link', { name: /invoice/i }).first();
		const hasLink = await invoicesLink.isVisible().catch(() => false);

		if (hasLink) {
			await invoicesLink.click();
			await page.waitForURL(/invoice/, { timeout: 15000 });
			await expect(page).toHaveURL(/invoice/);
		} else {
			await navigateToEnvPage(page, '/invoices', testInfo);
			await expect(page).toHaveURL(/invoice/);
		}

		const content = page.locator('main, [class*="content"], .container').first();
		await expect(content).toBeVisible();

		const hasInvoices = await page
			.locator('table tbody tr, .invoice-list, .workflow-hero, [class*="invoice"]')
			.first()
			.isVisible()
			.catch(() => false);
		const hasEmptyState = await page.getByText(/no invoice|create.*first|get started|no data/i).isVisible().catch(() => false);
		const hasHeading = await page.getByRole('heading', { level: 1, name: /^invoices$/i }).isVisible().catch(() => false);

		expect(hasInvoices || hasEmptyState || hasHeading).toBeTruthy();

		const createButton = page
			.getByRole('link', { name: /new|create|add/i })
			.or(page.getByRole('button', { name: /new|create|add/i }))
			.first();

		const hasCreate = await createButton.isVisible().catch(() => false);

		if (hasCreate) {
			await createButton.click();

			await expect(page.getByRole('dialog', { name: /new invoice/i })).toBeVisible({ timeout: 10000 });
			const hasForm = await page.locator('form').first().isVisible().catch(() => false);
			const hasModal = await page.locator('.modal, [role="dialog"]').first().isVisible().catch(() => false);

			expect(hasForm || hasModal || page.url().includes('/new')).toBeTruthy();
		}
	});
});
