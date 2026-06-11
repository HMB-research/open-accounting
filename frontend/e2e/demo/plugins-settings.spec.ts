import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant, waitForRouteReady } from './utils';

interface TenantPluginPayload {
	plugin_id: string;
	is_enabled: boolean;
	plugin?: {
		name: string;
		display_name: string;
	};
}

const demoPluginName = 'Demo Bank Import';
const demoPluginSlug = 'demo-bank-import';

function responsePath(responseUrl: string) {
	return new URL(responseUrl).pathname;
}

async function openPluginsPage(page: Page, testInfo: TestInfo): Promise<TenantPluginPayload[]> {
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
	const [tenantPluginsResponse] = await Promise.all([tenantPluginsLoaded, permissionsLoaded]);
	await waitForRouteReady(page, 'main h1, main .plugins-list, main .empty-state, main .alert-error');
	return (await tenantPluginsResponse.json()) as TenantPluginPayload[];
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
	test('toggles a tenant plugin and loads the admin plugins route', async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);

		const tenantPlugins = await openPluginsPage(page, testInfo);

		await expect(page.getByRole('heading', { name: /plugin|extension|integration/i }).first()).toBeVisible();
		await expect(page.locator('.plugins-list, .empty-state, .alert-error').first()).toBeVisible();

		const demoPlugin = tenantPlugins.find((plugin) => plugin.plugin?.name === demoPluginSlug);
		expect(demoPlugin, `${demoPluginSlug} should be seeded for demo plugin workflow coverage`).toBeTruthy();
		expect(demoPlugin?.is_enabled).toBe(false);

		const pluginCard = page.locator('.plugin-card').filter({ hasText: demoPluginName }).first();
		await expect(pluginCard).toBeVisible();
		await expect(pluginCard.locator('.plugin-info')).toContainText(demoPluginName);
		await expect(pluginCard.locator('.badge')).toContainText(/disabled|keelatud/i);
		await expect(pluginCard.locator('.permission-badge')).toContainText('banking:read');

		const enableResponsePromise = page.waitForResponse((response) => {
			return (
				response.request().method() === 'POST' &&
				new RegExp(`/api/v1/tenants/[^/]+/plugins/${demoPlugin!.plugin_id}/enable$`).test(
					responsePath(response.url())
				)
			);
		});
		await pluginCard.getByRole('button', { name: /enable|aktiveeri|luba/i }).click();
		const enableResponse = await enableResponsePromise;
		expect(enableResponse.ok()).toBeTruthy();
		const enabledPlugin = (await enableResponse.json()) as TenantPluginPayload;
		expect(enabledPlugin.plugin_id).toBe(demoPlugin?.plugin_id);
		expect(enabledPlugin.is_enabled).toBe(true);
		await expect(pluginCard.locator('.badge')).toContainText(/enabled|lubatud/i);

		const disableResponsePromise = page.waitForResponse((response) => {
			return (
				response.request().method() === 'POST' &&
				new RegExp(`/api/v1/tenants/[^/]+/plugins/${demoPlugin!.plugin_id}/disable$`).test(
					responsePath(response.url())
				)
			);
		});
		page.once('dialog', async (dialog) => {
			expect(dialog.type()).toBe('confirm');
			await dialog.accept();
		});
		await pluginCard.getByRole('button', { name: /disable|keela/i }).click();
		const disableResponse = await disableResponsePromise;
		expect(disableResponse.ok()).toBeTruthy();
		const disabledPlugin = (await disableResponse.json()) as { status: string };
		expect(disabledPlugin.status).toBe('disabled');
		await expect(pluginCard.locator('.badge')).toContainText(/disabled|keelatud/i);

		await openAdminPluginsPage(page, testInfo);

		await expect(page.getByRole('heading', { name: /plugin|admin/i }).first()).toBeVisible();
		await expect(page.locator('main .plugins-grid, main .plugin-card, main .empty-state').first()).toBeVisible();
		await expect(page.locator('main .plugin-card').filter({ hasText: demoPluginName })).toBeVisible();
		await expect(page.getByRole('button', { name: /search plugins|install from url/i }).first()).toBeVisible();
	});
});
