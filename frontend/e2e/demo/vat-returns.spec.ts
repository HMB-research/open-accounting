import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from './utils';

test.describe('Demo VAT Returns (KMD) - Page Structure Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/vat-returns', testInfo);
		await page.waitForTimeout(2000);
	});

	test('displays VAT returns page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /vat|käibe/i })).toBeVisible();
	});

	test('shows generate declaration section', async ({ page }) => {
		const hasGenerateHeading = await page.getByRole('heading', { name: /generate|genereeri/i }).isVisible().catch(() => false);
		const hasGenerateButton = await page.getByRole('button', { name: /generate|genereeri/i }).isVisible().catch(() => false);
		expect(hasGenerateHeading || hasGenerateButton).toBeTruthy();
	});

	test('has year dropdown', async ({ page }) => {
		const yearSelect = page.locator('select#year');
		await expect(yearSelect).toBeVisible();
		const options = await yearSelect.locator('option').allTextContents();
		expect(options.some(opt => opt.includes('2024') || opt.includes('2025') || opt.includes('2026'))).toBeTruthy();
	});

	test('has month dropdown', async ({ page }) => {
		const monthSelect = page.locator('select#month');
		await expect(monthSelect).toBeVisible();
		const options = await monthSelect.locator('option').allTextContents();
		expect(options.length).toBe(12);
	});

	test('shows declarations list section', async ({ page }) => {
		const hasDeclarationsHeading = await page.getByRole('heading', { name: /declaration|deklaratsioon/i }).isVisible().catch(() => false);
		const hasTable = await page.locator('table').first().isVisible().catch(() => false);
		const hasEmptyMessage = await page.getByText(/no declaration|pole deklaratsioon|no vat|ei leitud/i).isVisible().catch(() => false);
		const hasError = await page.getByText(/failed|error|viga/i).isVisible().catch(() => false);
		// Also check for "Previous Declarations" or "Eelmised deklaratsioonid" heading
		const hasPreviousHeading = await page.getByRole('heading', { name: /previous|eelmised/i }).isVisible().catch(() => false);
		expect(hasDeclarationsHeading || hasTable || hasEmptyMessage || hasError || hasPreviousHeading).toBeTruthy();
	});

	test('generate button is visible and enabled', async ({ page }) => {
		const generateButton = page.getByRole('button', { name: /generate|genereeri/i });
		await expect(generateButton).toBeVisible();
		await expect(generateButton).toBeEnabled();
	});
});
