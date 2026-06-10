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

	test('should have accessible navigation on mobile', async ({ page }, testInfo) => {
		await openMobileRoute(page, '/dashboard', testInfo);

		await expectMobileMenuButtonVisible(page);

		const drawer = await openMobileDrawer(page);
		await expect(drawer.getByRole('link', { name: /^Dashboard$/i })).toBeVisible();
		await expect(drawer.getByRole('link', { name: /^Reports$/i })).toBeVisible();
		await expect(drawer.getByRole('button', { name: /financial/i })).toBeVisible();
		await expect(drawer.getByRole('button', { name: /sales/i })).toBeVisible();
		await expect(drawer.getByRole('button', { name: /payments/i })).toBeVisible();
		await expect(drawer.getByRole('button', { name: /payroll/i })).toBeVisible();
		await expect(drawer.getByRole('button', { name: /admin/i })).toBeVisible();
	});

	test('should open mobile menu when hamburger clicked', async ({ page }, testInfo) => {
		await openMobileRoute(page, '/dashboard', testInfo);

		const drawer = await openMobileDrawer(page);
		await drawer.getByRole('button', { name: /sales/i }).click();
		await expect(drawer.getByRole('link', { name: /^Invoices$/i })).toBeVisible();
		await expect(drawer.getByRole('link', { name: /^Contacts$/i })).toBeVisible();
		await expect(drawer.getByRole('link', { name: /^Quotes$/i })).toBeVisible();
		await expect(drawer.getByRole('link', { name: /^Orders$/i })).toBeVisible();
	});

	test('should close menu when link is clicked', async ({ page }, testInfo) => {
		await openMobileRoute(page, '/dashboard', testInfo);

		const drawer = await openMobileDrawer(page);
		await drawer.getByRole('link', { name: /^Reports$/i }).click();

		await expect(page).toHaveURL(/\/reports/);
		await expect(page.getByRole('heading', { name: /reports/i }).first()).toBeVisible();
		await expect(drawer).toBeHidden();
	});

	test('should navigate through nested mobile menu links', async ({ page }, testInfo) => {
		await openMobileRoute(page, '/dashboard', testInfo);

		const drawer = await openMobileDrawer(page);
		await drawer.getByRole('button', { name: /sales/i }).click();
		await drawer.getByRole('link', { name: /^Invoices$/i }).click();

		await expect(page).toHaveURL(/\/invoices/);
		await expect(page.getByRole('heading', { level: 1, name: /^Invoices$/i })).toBeVisible();
		await expect(drawer).toBeHidden();
	});
});
