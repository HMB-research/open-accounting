import { test, expect } from '@playwright/test';
import { loginAsDemo, ensureDemoTenant, getDemoCredentials } from './utils';

test.describe('Demo Dashboard - Seeded Data Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('displays Demo Company in organization selector', async ({ page }, testInfo) => {
		const creds = getDemoCredentials(testInfo);
		// Find the org selector by looking for selects that contain the demo tenant name
		const selects = page.locator('select');
		const count = await selects.count();
		let foundDemo = false;
		for (let i = 0; i < count; i++) {
			const select = selects.nth(i);
			const text = await select.locator('option:checked').textContent();
			if (text && text.toLowerCase().includes('demo')) {
				foundDemo = true;
				break;
			}
		}
		expect(foundDemo).toBeTruthy();
	});

	test('shows Cash Flow card on dashboard', async ({ page }) => {
		// Dashboard shows Cash Flow card
		await expect(page.getByText(/Cash Flow/i).first()).toBeVisible();
	});

	test('shows Recent Activity section', async ({ page }) => {
		// Dashboard shows Recent Activity section
		await expect(page.getByText(/Recent Activity/i).first()).toBeVisible();
	});

	test('shows Revenue vs Expenses chart', async ({ page }) => {
		// Dashboard shows Revenue vs Expenses chart
		await expect(page.getByText(/Revenue vs Expenses/i).first()).toBeVisible();
	});

	test('shows New Organization button', async ({ page }) => {
		// Dashboard has + New Organization button
		await expect(page.getByRole('button', { name: /New Organization/i })).toBeVisible();
	});

	test('navigation header is visible with main menu items', async ({ page, browserName }) => {
		// Skip on mobile - navigation is hidden behind hamburger menu
		test.skip(browserName === 'webkit' || page.viewportSize()?.width! < 768, 'Mobile navigation is collapsed');

		// Check for navigation links in the nav element
		const nav = page.getByRole('navigation');
		await expect(nav.getByRole('link', { name: /Dashboard/i })).toBeVisible();
		await expect(nav.getByRole('link', { name: /Accounts/i })).toBeVisible();
		await expect(nav.getByRole('link', { name: /Journal/i })).toBeVisible();
		await expect(nav.getByRole('link', { name: /Contacts/i })).toBeVisible();
		await expect(nav.getByRole('link', { name: /Invoices/i })).toBeVisible();
	});
});
