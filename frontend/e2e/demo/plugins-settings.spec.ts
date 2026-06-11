import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant, waitForRouteReady } from './utils';

async function openPluginsPage(page: Page, testInfo: TestInfo): Promise<void> {
	const tenantPluginsLoaded = page.waitForResponse((response) => {
		const url = new URL(response.url());
		return (
			response.request().method() === 'GET' &&
			url.pathname.match(/\/api\/v1\/tenants\/[^/]+\/plugins$/) !== null &&
			response.status() === 200
		);
	});
	const permissionsLoaded = page.waitForResponse(
		(response) =>
			response.request().method() === 'GET' &&
			new URL(response.url()).pathname === '/api/v1/admin/plugins/permissions' &&
			response.status() === 200
	);

	await navigateTo(page, '/settings/plugins', testInfo, { waitForNetworkIdle: false });
	await Promise.all([tenantPluginsLoaded, permissionsLoaded]);
	await waitForRouteReady(page, 'main h1, main .plugins-list, main .empty-state, main .alert-error');
}

async function openAdminPluginsPage(page: Page, testInfo: TestInfo): Promise<void> {
	const pluginsLoaded = page.waitForResponse(
		(response) =>
			response.request().method() === 'GET' &&
			new URL(response.url()).pathname === '/api/v1/admin/plugins' &&
			response.status() === 200
	);
	const registriesLoaded = page.waitForResponse(
		(response) =>
			response.request().method() === 'GET' &&
			new URL(response.url()).pathname === '/api/v1/admin/plugin-registries' &&
			response.status() === 200
	);
	const permissionsLoaded = page.waitForResponse(
		(response) =>
			response.request().method() === 'GET' &&
			new URL(response.url()).pathname === '/api/v1/admin/plugins/permissions' &&
			response.status() === 200
	);

	await navigateTo(page, '/admin/plugins', testInfo, { waitForNetworkIdle: false });
	await Promise.all([pluginsLoaded, registriesLoaded, permissionsLoaded]);
	await waitForRouteReady(page, 'main h1, main .plugins-grid, main .plugin-card, main .empty-state, main .alert-error');
}

test.describe('Plugins Settings View', () => {
	test('renders tenant plugin settings and loads the admin plugins route', async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);

		await openPluginsPage(page, testInfo);

		await expect(page.getByRole('heading', { name: /plugin|extension|integration/i }).first()).toBeVisible();
		await expect(page.locator('.plugins-list, .empty-state, .alert-error').first()).toBeVisible();

		const pluginCards = page.locator('.plugin-card');
		if ((await pluginCards.count()) === 0) {
			await expect(page.locator('.empty-state, .alert-error').first()).toBeVisible();
		} else {
			await expect(pluginCards.first().locator('.plugin-info')).toBeVisible();
			await expect(
				pluginCards.first().getByRole('button', { name: /enable|disable|activate/i }).first()
			).toBeVisible();

			const permissionsSection = page.locator('.permissions-section').first();
			if (await permissionsSection.isVisible().catch(() => false)) {
				await expect(permissionsSection.locator('.permission-badge').first()).toBeVisible();
			}

			const settingsButton = page.getByRole('button', { name: /setting|configure|config/i }).first();
			if (await settingsButton.isVisible().catch(() => false)) {
				await expect(settingsButton).toBeVisible();
			} else {
				await expect(pluginCards.first().locator('.plugin-actions')).toBeVisible();
			}
		}

		await openAdminPluginsPage(page, testInfo);

		await expect(page.getByRole('heading', { name: /plugin|admin/i }).first()).toBeVisible();
		await expect(page.locator('main .plugins-grid, main .plugin-card, main .empty-state').first()).toBeVisible();
		await expect(page.getByRole('button', { name: /search plugins|install from url/i }).first()).toBeVisible();
	});
});
