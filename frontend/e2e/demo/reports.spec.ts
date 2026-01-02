import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant } from './utils';

test.describe('Demo Reports - Page Structure Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/reports');
		await page.waitForTimeout(2000);
	});

	test('displays reports page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /report/i })).toBeVisible();
	});

	test('shows report type selector or buttons', async ({ page }) => {
		const hasSelector = await page.locator('select').first().isVisible().catch(() => false);
		const hasButtons = await page.getByRole('button', { name: /trial|balance|income|generate/i }).first().isVisible().catch(() => false);
		expect(hasSelector || hasButtons).toBeTruthy();
	});

	test('shows date range controls', async ({ page }) => {
		const hasDateInputs = await page.locator('input[type="date"]').first().isVisible().catch(() => false);
		const hasDatePicker = await page.getByText(/from|to|period|date/i).first().isVisible().catch(() => false);
		expect(hasDateInputs || hasDatePicker).toBeTruthy();
	});
});
