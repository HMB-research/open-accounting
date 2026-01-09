import { test, expect } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from './utils';

/**
 * Wait for cost centers page to be ready
 */
async function waitForPageReady(page: import('@playwright/test').Page) {
	await expect(async () => {
		const isLoading = await page.getByText(/^Loading\.\.\.$/i).first().isVisible().catch(() => false);
		expect(isLoading).toBe(false);
	}).toPass({ timeout: 15000 });
}

test.describe('Demo Cost Centers - Page Structure', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/settings/cost-centers', testInfo);
		await waitForPageReady(page);
	});

	test('displays cost centers page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
	});

	test('has add button', async ({ page }) => {
		await expect(async () => {
			const hasBtn = await page.getByRole('button', { name: /add|lisa|\+/i }).first().isVisible().catch(() => false);
			expect(hasBtn).toBeTruthy();
		}).toPass({ timeout: 10000 });
	});

	test('shows empty state or cost center list', async ({ page }) => {
		await expect(async () => {
			const hasEmptyState = await page.locator('.empty-state').isVisible().catch(() => false);
			const hasTable = await page.locator('table').isVisible().catch(() => false);
			const hasCard = await page.locator('.card').isVisible().catch(() => false);
			expect(hasEmptyState || hasTable || hasCard).toBeTruthy();
		}).toPass({ timeout: 10000 });
	});
});

test.describe('Demo Cost Centers - Create Modal', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await navigateTo(page, '/settings/cost-centers', testInfo);
		await waitForPageReady(page);
	});

	test('opens create modal when clicking add button', async ({ page }) => {
		// Click add button
		const addBtn = page.getByRole('button', { name: /add|lisa|\+/i }).first();
		await expect(addBtn).toBeVisible({ timeout: 10000 });
		await addBtn.click();

		// Modal should appear
		const modal = page.locator('[role="dialog"], .modal');
		await expect(modal).toBeVisible({ timeout: 5000 });
	});

	test('modal can be closed', async ({ page }) => {
		// Open modal
		const addBtn = page.getByRole('button', { name: /add|lisa|\+/i }).first();
		await expect(addBtn).toBeVisible({ timeout: 10000 });
		await addBtn.click();

		// Wait for modal
		const modal = page.locator('[role="dialog"], .modal');
		await expect(modal).toBeVisible({ timeout: 5000 });

		// Close modal (try cancel button or close button)
		const closeBtn = modal.getByRole('button', { name: /cancel|tühista|close|sulge/i }).first()
			.or(modal.locator('.btn-close, button[aria-label*="close"]').first());
		await closeBtn.click();

		// Modal should be hidden
		await expect(modal).not.toBeVisible({ timeout: 5000 });
	});

	test('modal has form fields', async ({ page }) => {
		// Open modal
		const addBtn = page.getByRole('button', { name: /add|lisa|\+/i }).first();
		await expect(addBtn).toBeVisible({ timeout: 10000 });
		await addBtn.click();

		// Wait for modal
		const modal = page.locator('[role="dialog"], .modal');
		await expect(modal).toBeVisible({ timeout: 5000 });

		// Check form fields exist
		await expect(async () => {
			const hasInputs = await modal.locator('input').first().isVisible().catch(() => false);
			expect(hasInputs).toBeTruthy();
		}).toPass({ timeout: 5000 });
	});
});
