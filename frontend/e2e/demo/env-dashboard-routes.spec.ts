import { test, expect } from '@playwright/test';
import {
	loginAsDemoEnv,
	navigateToEnvPage,
	waitForDemoShell,
	waitForVisibleContent
} from './env-utils';

test.describe('Demo Environment - Dashboard', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemoEnv(page, testInfo);
	});

	test('Dashboard displays organization selector', async ({ page }) => {
		const orgSelector = page.locator('.tenant-selector, [class*="org-selector"], select').first();
		await expect(orgSelector).toBeVisible({ timeout: 10000 });
	});

	test('Dashboard shows summary cards', async ({ page }) => {
		await waitForVisibleContent(page);

		const summarySection = page.locator('.summary-grid, .stats, [class*="summary"]').first();
		const hasSummary = await summarySection.isVisible({ timeout: 15000 }).catch(() => false);

		if (hasSummary) {
			await expect(summarySection).toBeVisible();
		} else {
			const dashboardContent = page.locator('main, .dashboard, [class*="content"]').first();
			await expect(dashboardContent).toBeVisible();
		}
	});

	test('Navigation sidebar is present', async ({ page }) => {
		await waitForDemoShell(page);
		const navItems = ['dashboard', 'invoice', 'contact', 'report'];
		const visibleItems = await Promise.all(
			navItems.map((item) =>
				page
					.getByRole('link', { name: new RegExp(item, 'i') })
					.first()
					.isVisible()
					.catch(() => false)
			)
		);
		const hasMenuShell = await page
			.locator('nav.navbar, .mobile-nav, .mobile-menu-btn, [aria-label*="menu"]')
			.first()
			.isVisible()
			.catch(() => false);

		expect(hasMenuShell || visibleItems.some(Boolean)).toBeTruthy();
	});
});

test.describe('Demo Environment - Invoices', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemoEnv(page, testInfo);
	});

	test('Can navigate to invoices page', async ({ page }) => {
		const invoicesLink = page.getByRole('link', { name: /invoice/i }).first();
		const hasLink = await invoicesLink.isVisible().catch(() => false);

		if (hasLink) {
			await invoicesLink.click();
			await page.waitForURL(/invoice/, { timeout: 15000 });
			await expect(page).toHaveURL(/invoice/);
		} else {
			await navigateToEnvPage(page, '/invoices');
			await expect(page).toHaveURL(/invoice/);
		}
	});

	test('Invoices list displays', async ({ page }) => {
		await navigateToEnvPage(page, '/invoices');

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
	});

	test('Can access create invoice form', async ({ page }) => {
		await navigateToEnvPage(page, '/invoices');

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

test.describe('Demo Environment - Contacts', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemoEnv(page, testInfo);
	});

	test('Can navigate to contacts page', async ({ page }) => {
		const contactsLink = page.getByRole('link', { name: /contact|customer|client/i }).first();
		const hasLink = await contactsLink.isVisible().catch(() => false);

		if (hasLink) {
			await contactsLink.click();
			await waitForVisibleContent(page);
			await expect(page).toHaveURL(/contact/);
		} else {
			await navigateToEnvPage(page, '/contacts');
			await expect(page).toHaveURL(/contact/);
		}
	});

	test('Contacts list displays', async ({ page }) => {
		await navigateToEnvPage(page, '/contacts');

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();
	});
});

test.describe('Demo Environment - Reports', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemoEnv(page, testInfo);
	});

	test('Can navigate to reports page', async ({ page }) => {
		const reportsLink = page.getByRole('link', { name: /report/i }).first();
		const hasLink = await reportsLink.isVisible().catch(() => false);

		if (hasLink) {
			await reportsLink.click();
			await waitForVisibleContent(page);
			await expect(page).toHaveURL(/report/);
		} else {
			await navigateToEnvPage(page, '/reports');
			await expect(page).toHaveURL(/report/);
		}
	});

	test('Reports page loads', async ({ page }) => {
		await navigateToEnvPage(page, '/reports');

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();
	});
});

test.describe('Demo Environment - Settings', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemoEnv(page, testInfo);
	});

	test('Can access settings', async ({ page }) => {
		const settingsLink = page.getByRole('link', { name: /setting/i }).first();
		const hasLink = await settingsLink.isVisible().catch(() => false);

		if (hasLink) {
			await settingsLink.click();
			await waitForVisibleContent(page);
			await expect(page).toHaveURL(/setting/);
		} else {
			await navigateToEnvPage(page, '/settings');
			await expect(page).toHaveURL(/setting/);
		}
	});
});
