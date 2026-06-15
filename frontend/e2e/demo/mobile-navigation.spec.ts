import { expect, test } from '@playwright/test';
import {
	expectMobileMenuButtonVisible,
	openMobileDrawer,
	openMobileRoute,
	prepareMobileDemo
} from './mobile-utils';

test.describe('Mobile Navigation', () => {
	test.use({ viewport: { width: 375, height: 667 } });

	test.beforeEach(async ({ page }, testInfo) => {
		await prepareMobileDemo(page, testInfo);
	});

	test('mobile drawer supports top-level, grouped, and link navigation', async ({ page }, testInfo) => {
		await openMobileRoute(page, '/dashboard', testInfo);

		await expectMobileMenuButtonVisible(page);

		let drawer = await openMobileDrawer(page);
		await expect(drawer.getByRole('link', { name: /^Dashboard$/i })).toBeVisible();
		await expect(drawer.getByRole('link', { name: /^Reports$/i })).toBeVisible();
		await expect(drawer.getByRole('button', { name: /financial/i })).toBeVisible();
		await expect(drawer.getByRole('button', { name: /sales/i })).toBeVisible();
		await expect(drawer.getByRole('button', { name: /payments/i })).toBeVisible();
		await expect(drawer.getByRole('button', { name: /payroll/i })).toBeVisible();
		await expect(drawer.getByRole('button', { name: /admin/i })).toBeVisible();

		await drawer.getByRole('button', { name: /sales/i }).click();
		await expect(drawer.getByRole('link', { name: /^Invoices$/i })).toBeVisible();
		await expect(drawer.getByRole('link', { name: /^Contacts$/i })).toBeVisible();
		await expect(drawer.getByRole('link', { name: /^Quotes$/i })).toBeVisible();
		await expect(drawer.getByRole('link', { name: /^Orders$/i })).toBeVisible();

		await drawer.getByRole('link', { name: /^Reports$/i }).click();

		await expect(page).toHaveURL(/\/reports/);
		await expect(page.getByRole('heading', { name: /reports/i }).first()).toBeVisible();
		await expect(drawer).toBeHidden();

		drawer = await openMobileDrawer(page);
		await drawer.getByRole('button', { name: /sales/i }).click();
		await drawer.getByRole('link', { name: /^Invoices$/i }).click();

		await expect(page).toHaveURL(/\/invoices/);
		await expect(page.getByRole('heading', { level: 1, name: /^Invoices$/i })).toBeVisible();
		await expect(drawer).toBeHidden();
	});
});
