import { cleanup, render, screen } from '@testing-library/svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import Slot from '$lib/plugins/Slot.svelte';
import {
	clearPluginFrontendComponents,
	pluginManager,
	registerPluginFrontendComponent
} from '$lib/plugins';
import { api } from '$lib/api';
import RegisteredPluginWidget from './fixtures/PluginSlotWidget.svelte';

describe('Plugin Slot', () => {
	beforeEach(() => {
		cleanup();
		clearPluginFrontendComponents();
		pluginManager.clear();
		vi.clearAllMocks();
	});

	it('renders registered frontend slot components', async () => {
		registerPluginFrontendComponent('plugin-1/RiskWidget.svelte', RegisteredPluginWidget);

		vi.spyOn(api, 'listTenantPlugins').mockResolvedValueOnce([
			{
				id: 'tenant-plugin-1',
				tenant_id: 'tenant-1',
				plugin_id: 'plugin-1',
				is_enabled: true,
				config: {},
				settings: {},
				created_at: '2026-06-14T00:00:00Z',
				updated_at: '2026-06-14T00:00:00Z',
				plugin: {
					id: 'plugin-1',
					name: 'risk-tools',
					display_name: 'Risk Tools',
					version: '1.0.0',
					repository_url: 'https://github.com/example/risk-tools',
					repository_type: 'github',
					state: 'enabled',
					granted_permissions: [],
					installed_at: '2026-06-14T00:00:00Z',
					updated_at: '2026-06-14T00:00:00Z',
					manifest: {
						name: 'risk-tools',
						display_name: 'Risk Tools',
						version: '1.0.0',
						permissions: [],
						frontend: {
							components: './frontend',
							slots: [
								{
									name: 'dashboard.widgets',
									component: 'RiskWidget.svelte',
									label: 'Supplier risk review',
									description: 'Open supplier exceptions before month end.',
									path: '/plugins/risk-tools/suppliers',
									kind: 'card',
									badge: '3 open',
									order: 10
								}
							]
						}
					}
				}
			}
		] as unknown as import('$lib/api').TenantPlugin[]);

		await pluginManager.loadPlugins('tenant-1');
		render(Slot, {
			props: {
				name: 'dashboard.widgets',
				props: { headline: 'Live risk widget', tenantId: 'tenant-1' }
			}
		});

		const widget = screen.getByTestId('registered-plugin-component');
		expect(widget).toHaveAttribute('data-plugin-id', 'plugin-1');
		expect(widget).toHaveAttribute('data-tenant-id', 'tenant-1');
		expect(screen.getByRole('heading', { name: 'Live risk widget' })).toBeInTheDocument();
		expect(screen.getByText('Supplier risk review')).toBeInTheDocument();
		expect(
			screen.queryByText('Open supplier exceptions before month end.')
		).not.toBeInTheDocument();
	});

	it('renders declarative frontend slot registrations', async () => {
		vi.spyOn(api, 'listTenantPlugins').mockResolvedValueOnce([
			{
				id: 'tenant-plugin-1',
				tenant_id: 'tenant-1',
				plugin_id: 'plugin-1',
				is_enabled: true,
				config: {},
				settings: {},
				created_at: '2026-06-14T00:00:00Z',
				updated_at: '2026-06-14T00:00:00Z',
				plugin: {
					id: 'plugin-1',
					name: 'risk-tools',
					display_name: 'Risk Tools',
					version: '1.0.0',
					repository_url: 'https://github.com/example/risk-tools',
					repository_type: 'github',
					state: 'enabled',
					granted_permissions: [],
					installed_at: '2026-06-14T00:00:00Z',
					updated_at: '2026-06-14T00:00:00Z',
					manifest: {
						name: 'risk-tools',
						display_name: 'Risk Tools',
						version: '1.0.0',
						permissions: [],
						frontend: {
							components: './frontend',
							slots: [
								{
									name: 'dashboard.widgets',
									component: 'RiskWidget.svelte',
									label: 'Supplier risk review',
									description: 'Open supplier exceptions before month end.',
									path: '/plugins/risk-tools/suppliers',
									kind: 'card',
									badge: '3 open',
									order: 10
								}
							]
						}
					}
				}
			}
		] as unknown as import('$lib/api').TenantPlugin[]);

		await pluginManager.loadPlugins('tenant-1');
		render(Slot, { props: { name: 'dashboard.widgets' } });

		const title = screen.getByText('Supplier risk review');
		expect(title).toBeInTheDocument();
		expect(screen.getByText('Open supplier exceptions before month end.')).toBeInTheDocument();
		expect(screen.getByText('3 open')).toBeInTheDocument();
		expect(title.closest('a')).toHaveAttribute('href', '/plugins/risk-tools/suppliers');
	});

	it('keeps deterministic order when registered and declarative slots are mixed', async () => {
		registerPluginFrontendComponent('plugin-1/RegisteredWidget.svelte', RegisteredPluginWidget);

		vi.spyOn(api, 'listTenantPlugins').mockResolvedValueOnce([
			{
				id: 'tenant-plugin-1',
				tenant_id: 'tenant-1',
				plugin_id: 'plugin-1',
				is_enabled: true,
				config: {},
				settings: {},
				created_at: '2026-06-14T00:00:00Z',
				updated_at: '2026-06-14T00:00:00Z',
				plugin: {
					id: 'plugin-1',
					name: 'risk-tools',
					display_name: 'Risk Tools',
					version: '1.0.0',
					repository_url: 'https://github.com/example/risk-tools',
					repository_type: 'github',
					state: 'enabled',
					granted_permissions: [],
					installed_at: '2026-06-14T00:00:00Z',
					updated_at: '2026-06-14T00:00:00Z',
					manifest: {
						name: 'risk-tools',
						display_name: 'Risk Tools',
						version: '1.0.0',
						permissions: [],
						frontend: {
							components: './frontend',
							slots: [
								{
									name: 'dashboard.widgets',
									component: 'LateWidget.svelte',
									label: 'Late declarative widget',
									kind: 'card',
									order: 20
								},
								{
									name: 'dashboard.widgets',
									component: 'RegisteredWidget.svelte',
									label: 'Early registered widget',
									kind: 'card',
									order: 10
								}
							]
						}
					}
				}
			}
		] as unknown as import('$lib/api').TenantPlugin[]);

		await pluginManager.loadPlugins('tenant-1');
		render(Slot, { props: { name: 'dashboard.widgets' } });

		const labels = screen
			.getAllByText(/widget/i)
			.map((element) => element.textContent)
			.filter((text): text is string => Boolean(text?.includes('widget')));

		expect(labels).toEqual(['Early registered widget', 'Late declarative widget']);
	});
});
