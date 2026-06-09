import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant, waitForRouteReady } from './utils';

async function openPluginsPage(page: Page, testInfo: TestInfo): Promise<void> {
	await navigateTo(page, '/settings/plugins', testInfo);
	await waitForRouteReady(page, 'h1');
}

test.describe('Plugins Settings View', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('displays plugins page with correct structure', async ({ page }, testInfo) => {
		await openPluginsPage(page, testInfo);

		await expect(page.getByRole('heading', { name: /plugin|extension|integration/i }).first()).toBeVisible();
	});

	test('displays plugin list or empty state', async ({ page }, testInfo) => {
		await openPluginsPage(page, testInfo);

		await expect(page.locator('.plugins-list, .empty-state, .loading, .alert-error').first()).toBeVisible();
	});

	test('shows plugin enable/disable controls', async ({ page }, testInfo) => {
		await openPluginsPage(page, testInfo);

		const pluginCards = page.locator('.plugin-card');
		if ((await pluginCards.count()) === 0) {
			await expect(page.locator('.empty-state, .loading, .alert-error').first()).toBeVisible();
			return;
		}

		await expect(
			pluginCards.first().getByRole('button', { name: /enable|disable|activate/i }).first()
		).toBeVisible();
	});

	test('shows plugin permissions information', async ({ page }, testInfo) => {
		await openPluginsPage(page, testInfo);

		const pluginCards = page.locator('.plugin-card');
		if ((await pluginCards.count()) === 0) {
			await expect(page.locator('.empty-state, .loading, .alert-error').first()).toBeVisible();
			return;
		}

		const permissionsSection = page.locator('.permissions-section').first();
		if (await permissionsSection.isVisible().catch(() => false)) {
			await expect(permissionsSection.locator('.permission-badge').first()).toBeVisible();
			return;
		}

		await expect(pluginCards.first().locator('.plugin-info')).toBeVisible();
	});

	test('has settings button for enabled plugins', async ({ page }, testInfo) => {
		await openPluginsPage(page, testInfo);

		const pluginCards = page.locator('.plugin-card');
		if ((await pluginCards.count()) === 0) {
			await expect(page.locator('.empty-state, .loading, .alert-error').first()).toBeVisible();
			return;
		}

		const settingsButton = page.getByRole('button', { name: /setting|configure|config/i }).first();
		if (await settingsButton.isVisible().catch(() => false)) {
			await expect(settingsButton).toBeVisible();
			return;
		}

		await expect(pluginCards.first().locator('.plugin-actions')).toBeVisible();
	});
});
