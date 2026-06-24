import { describe, it, expect, vi, beforeEach } from 'vitest';
import { parsePosition, pluginManager } from '$lib/plugins/manager';
import {
	getPluginFrontendComponentCandidateIds,
	clearPluginFrontendComponents,
	normalizePluginComponentReference,
	registerPluginFrontendComponent,
	resolvePluginFrontendComponent,
	SLOT_NAMES,
	unregisterPluginFrontendComponent,
	type PluginFrontendComponent,
	type SlotName
} from '$lib/plugins';
import { api } from '$lib/api';

describe('parsePosition', () => {
	it('should return default position for undefined', () => {
		expect(parsePosition(undefined)).toBe(1000);
	});

	it('should return default position for empty string', () => {
		expect(parsePosition('')).toBe(1000);
	});

	it('should parse numeric positions', () => {
		expect(parsePosition('100')).toBe(100);
		expect(parsePosition('500')).toBe(500);
		expect(parsePosition('0')).toBe(0);
		expect(parsePosition('-50')).toBe(-50);
	});

	it('should handle after: positions', () => {
		expect(parsePosition('after:dashboard')).toBe(150);
		expect(parsePosition('after:invoices')).toBe(550);
		expect(parsePosition('after:admin')).toBe(950);
	});

	it('should handle before: positions', () => {
		expect(parsePosition('before:dashboard')).toBe(50);
		expect(parsePosition('before:invoices')).toBe(450);
		expect(parsePosition('before:reports')).toBe(650);
	});

	it('should return default for unknown targets in after:', () => {
		expect(parsePosition('after:unknown')).toBe(550); // 500 + 50
	});

	it('should return default for unknown targets in before:', () => {
		expect(parsePosition('before:unknown')).toBe(450); // 500 - 50
	});

	it('should handle all known positions', () => {
		expect(parsePosition('after:accounts')).toBe(250);
		expect(parsePosition('after:journal')).toBe(350);
		expect(parsePosition('after:contacts')).toBe(450);
		expect(parsePosition('after:payments')).toBe(650);
		expect(parsePosition('after:payroll')).toBe(850);
	});

	it('should maintain sort order', () => {
		const positions = [
			parsePosition('after:admin'),
			parsePosition('before:dashboard'),
			parsePosition('after:dashboard'),
			parsePosition('100'),
			parsePosition(undefined)
		];

		const sorted = [...positions].sort((a, b) => a - b);
		expect(sorted).toEqual([50, 100, 150, 950, 1000]);
	});
});

describe('pluginManager', () => {
	beforeEach(() => {
		pluginManager.clear();
	});

	it('should start with empty state', () => {
		expect(pluginManager.isLoaded()).toBe(false);
		expect(pluginManager.getNavigation()).toEqual([]);
		expect(pluginManager.getEnabledPlugins()).toEqual([]);
	});

	it('should return empty array for non-existent slot', () => {
		expect(pluginManager.getSlotRegistrations('nonexistent')).toEqual([]);
	});

	it('should return false for hasSlotContent on empty slot', () => {
		expect(pluginManager.hasSlotContent('dashboard-widgets')).toBe(false);
	});

	it('should clear state', () => {
		pluginManager.clear();
		expect(pluginManager.isLoaded()).toBe(false);
		expect(pluginManager.getNavigation()).toEqual([]);
	});

	it('should support subscription', () => {
		const callback = vi.fn();
		const unsubscribe = pluginManager.subscribe(callback);

		// Should be called immediately with initial state
		expect(callback).toHaveBeenCalledTimes(1);

		// Clear triggers notification
		pluginManager.clear();
		expect(callback).toHaveBeenCalledTimes(2);

		// After unsubscribe, no more calls
		unsubscribe();
		pluginManager.clear();
		expect(callback).toHaveBeenCalledTimes(2);
	});
});

describe('Plugin Navigation Sorting', () => {
	it('should sort navigation items by position', () => {
		interface NavItem {
			label: string;
			position?: string;
		}

		const items: NavItem[] = [
			{ label: 'Custom Reports', position: 'after:reports' },
			{ label: 'Quick Actions', position: 'after:dashboard' },
			{ label: 'Analytics', position: 'before:invoices' },
			{ label: 'Settings', position: undefined }
		];

		const sorted = [...items].sort((a, b) => {
			return parsePosition(a.position) - parsePosition(b.position);
		});

		expect(sorted.map((i) => i.label)).toEqual([
			'Quick Actions', // 150
			'Analytics', // 450
			'Custom Reports', // 750
			'Settings' // 1000
		]);
	});
});

describe('pluginManager.loadPlugins', () => {
	beforeEach(() => {
		pluginManager.clear();
		vi.clearAllMocks();
	});

	it('should load plugins and extract navigation', async () => {
		const mockPlugins = [
			{
				id: 'tp-1',
				tenant_id: 'tenant-1',
				plugin_id: 'plugin-1',
				is_enabled: true,
				config: {},
				settings: {},
				created_at: '2024-01-01T00:00:00Z',
				updated_at: '2024-01-01T00:00:00Z',
				plugin: {
					id: 'plugin-1',
					name: 'Test Plugin',
					version: '1.0.0',
					description: 'A test plugin',
					manifest: {
						id: 'plugin-1',
						name: 'Test Plugin',
						version: '1.0.0',
						frontend: {
							navigation: [
								{
									label: 'Test Nav',
									path: '/test',
									icon: 'test-icon',
									position: 'after:dashboard'
								}
							],
							slots: [
								{
									name: 'dashboard-widgets',
									component: 'TestWidget',
									label: 'Review exceptions',
									description: 'Open the plugin exception queue',
									path: '/plugins/test-plugin/exceptions',
									kind: 'card',
									badge: '2 open',
									order: 25
								}
							]
						}
					}
				}
			}
		];

		vi.spyOn(api, 'listTenantPlugins').mockResolvedValueOnce(mockPlugins as unknown as import('$lib/api').TenantPlugin[]);

		await pluginManager.loadPlugins('tenant-1');

		expect(pluginManager.isLoaded()).toBe(true);
		expect(pluginManager.getNavigation()).toHaveLength(1);
		expect(pluginManager.getNavigation()[0]).toMatchObject({
			label: 'Test Nav',
			path: '/test',
			pluginId: 'plugin-1',
			pluginName: 'Test Plugin'
		});
		expect(pluginManager.hasSlotContent('dashboard-widgets')).toBe(true);
		expect(pluginManager.getSlotRegistrations('dashboard-widgets')).toHaveLength(1);
		expect(pluginManager.getSlotRegistrations('dashboard-widgets')[0]).toMatchObject({
			componentName: 'TestWidget',
			componentRef: 'TestWidget',
			label: 'Review exceptions',
			description: 'Open the plugin exception queue',
			path: '/plugins/test-plugin/exceptions',
			kind: 'card',
			badge: '2 open',
			order: 25
		});
	});

	it('should sort slot registrations and strip unsafe paths', async () => {
		const mockPlugins = [
			{
				id: 'tp-1',
				tenant_id: 'tenant-1',
				plugin_id: 'plugin-1',
				is_enabled: true,
				config: {},
				settings: {},
				created_at: '2024-01-01T00:00:00Z',
				updated_at: '2024-01-01T00:00:00Z',
				plugin: {
					id: 'plugin-1',
					name: 'Alpha Plugin',
					version: '1.0.0',
					description: 'Test',
					manifest: {
						id: 'plugin-1',
						name: 'Alpha',
						version: '1.0.0',
						frontend: {
							slots: [
								{
									name: 'dashboard.widgets',
									component: 'LateWidget',
									label: 'Late',
									path: 'https://example.com/unsafe',
									order: 50
								},
								{
									name: 'dashboard.widgets',
									component: 'SameOrderBWidget',
									label: 'B label',
									path: '//example.com/unsafe',
									order: 10
								},
								{
									name: 'dashboard.widgets',
									component: 'EarlyWidget',
									label: 'A label',
									path: '/plugins/alpha/early',
									kind: 'unsupported',
									order: 10
								}
							]
						}
					}
				}
			},
			{
				id: 'tp-2',
				tenant_id: 'tenant-1',
				plugin_id: 'plugin-2',
				is_enabled: true,
				config: {},
				settings: {},
				created_at: '2024-01-01T00:00:00Z',
				updated_at: '2024-01-01T00:00:00Z',
				plugin: {
					id: 'plugin-2',
					name: 'Beta Plugin',
					version: '1.0.0',
					description: 'Test',
					manifest: {
						id: 'plugin-2',
						name: 'Beta',
						version: '1.0.0',
						frontend: {
							slots: [
								{
									name: 'dashboard.widgets',
									component: 'BetaWidget',
									label: 'Beta',
									path: '/plugins/beta/widget',
									order: 10
								}
							]
						}
					}
				}
			}
		];

		vi.spyOn(api, 'listTenantPlugins').mockResolvedValueOnce(mockPlugins as unknown as import('$lib/api').TenantPlugin[]);

		await pluginManager.loadPlugins('tenant-1');

		const registrations = pluginManager.getSlotRegistrations('dashboard.widgets');
		expect(registrations.map((slot) => slot.label)).toEqual(['A label', 'B label', 'Beta', 'Late']);
		expect(registrations[0]).toMatchObject({
			path: '/plugins/alpha/early',
			kind: 'link'
		});
		expect(registrations[1]).toMatchObject({
			path: undefined,
			kind: 'card'
		});
	});

	it('should reject unsafe slot component references for dynamic resolution', async () => {
		const mockPlugins = [
			{
				id: 'tp-1',
				tenant_id: 'tenant-1',
				plugin_id: 'plugin-1',
				is_enabled: true,
				config: {},
				settings: {},
				created_at: '2024-01-01T00:00:00Z',
				updated_at: '2024-01-01T00:00:00Z',
				plugin: {
					id: 'plugin-1',
					name: 'Alpha Plugin',
					version: '1.0.0',
					description: 'Test',
					manifest: {
						id: 'plugin-1',
						name: 'Alpha',
						version: '1.0.0',
						frontend: {
							slots: [
								{
									name: 'dashboard.widgets',
									component: '../UnsafeWidget.svelte',
									label: 'Unsafe widget',
									path: '/plugins/alpha/unsafe',
									order: 10
								}
							]
						}
					}
				}
			}
		];

		vi.spyOn(api, 'listTenantPlugins').mockResolvedValueOnce(
			mockPlugins as unknown as import('$lib/api').TenantPlugin[]
		);

		await pluginManager.loadPlugins('tenant-1');

		const [registration] = pluginManager.getSlotRegistrations('dashboard.widgets');
		expect(registration).toMatchObject({
			componentName: '../UnsafeWidget.svelte',
			componentRef: undefined,
			label: 'Unsafe widget',
			path: '/plugins/alpha/unsafe'
		});
		expect(getPluginFrontendComponentCandidateIds(registration)).toEqual([]);
	});

	it('should skip already loaded tenant', async () => {
		const mockPlugins = [
			{
				id: 'tp-1',
				tenant_id: 'tenant-1',
				plugin_id: 'plugin-1',
				is_enabled: true,
				config: {},
				settings: {},
				created_at: '2024-01-01T00:00:00Z',
				updated_at: '2024-01-01T00:00:00Z',
				plugin: {
					id: 'plugin-1',
					name: 'Test Plugin',
					version: '1.0.0',
					description: 'A test plugin',
					manifest: { id: 'plugin-1', name: 'Test', version: '1.0.0' }
				}
			}
		];

		const spy = vi.spyOn(api, 'listTenantPlugins').mockResolvedValue(mockPlugins as unknown as import('$lib/api').TenantPlugin[]);

		await pluginManager.loadPlugins('tenant-1');
		expect(spy).toHaveBeenCalledTimes(1);

		// Second call should be skipped
		await pluginManager.loadPlugins('tenant-1');
		expect(spy).toHaveBeenCalledTimes(1);
	});

	it('should reload for different tenant', async () => {
		const mockPlugins = [
			{
				id: 'tp-1',
				tenant_id: 'tenant-1',
				plugin_id: 'plugin-1',
				is_enabled: true,
				config: {},
				settings: {},
				created_at: '2024-01-01T00:00:00Z',
				updated_at: '2024-01-01T00:00:00Z',
				plugin: {
					id: 'plugin-1',
					name: 'Test Plugin',
					version: '1.0.0',
					description: 'Test',
					manifest: { id: 'plugin-1', name: 'Test', version: '1.0.0' }
				}
			}
		];

		const spy = vi.spyOn(api, 'listTenantPlugins').mockResolvedValue(mockPlugins as unknown as import('$lib/api').TenantPlugin[]);

		await pluginManager.loadPlugins('tenant-1');
		await pluginManager.loadPlugins('tenant-2');

		expect(spy).toHaveBeenCalledTimes(2);
	});

	it('should handle API errors gracefully', async () => {
		vi.spyOn(api, 'listTenantPlugins').mockRejectedValueOnce(new Error('Network error'));
		const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

		await pluginManager.loadPlugins('tenant-1');

		expect(pluginManager.isLoaded()).toBe(false);
		expect(consoleErrorSpy).toHaveBeenCalled();
		consoleErrorSpy.mockRestore();
	});

	it('should filter out disabled plugins', async () => {
		const mockPlugins = [
			{
				id: 'tp-1',
				tenant_id: 'tenant-1',
				plugin_id: 'plugin-1',
				is_enabled: false,
				config: {},
				settings: {},
				created_at: '2024-01-01T00:00:00Z',
				updated_at: '2024-01-01T00:00:00Z',
				plugin: {
					id: 'plugin-1',
					name: 'Disabled Plugin',
					version: '1.0.0',
					description: 'Test',
					manifest: {
						id: 'plugin-1',
						name: 'Disabled',
						version: '1.0.0',
						frontend: {
							navigation: [{ label: 'Should Not Show', path: '/hidden', icon: 'x' }]
						}
					}
				}
			}
		];

		vi.spyOn(api, 'listTenantPlugins').mockResolvedValueOnce(mockPlugins as unknown as import('$lib/api').TenantPlugin[]);

		await pluginManager.loadPlugins('tenant-1');

		expect(pluginManager.getNavigation()).toHaveLength(0);
		expect(pluginManager.getEnabledPlugins()).toHaveLength(0);
	});

	it('should handle plugins without manifest', async () => {
		const mockPlugins = [
			{
				id: 'tp-1',
				tenant_id: 'tenant-1',
				plugin_id: 'plugin-1',
				is_enabled: true,
				config: {},
				settings: {},
				created_at: '2024-01-01T00:00:00Z',
				updated_at: '2024-01-01T00:00:00Z',
				plugin: {
					id: 'plugin-1',
					name: 'Plugin Without Manifest',
					version: '1.0.0',
					description: 'Test',
					manifest: null
				}
			}
		];

		vi.spyOn(api, 'listTenantPlugins').mockResolvedValueOnce(mockPlugins as unknown as import('$lib/api').TenantPlugin[]);

		await pluginManager.loadPlugins('tenant-1');

		expect(pluginManager.isLoaded()).toBe(true);
		expect(pluginManager.getNavigation()).toHaveLength(0);
	});

	it('should sort navigation by position', async () => {
		const mockPlugins = [
			{
				id: 'tp-1',
				tenant_id: 'tenant-1',
				plugin_id: 'plugin-1',
				is_enabled: true,
				config: {},
				settings: {},
				created_at: '2024-01-01T00:00:00Z',
				updated_at: '2024-01-01T00:00:00Z',
				plugin: {
					id: 'plugin-1',
					name: 'Multi-Nav Plugin',
					version: '1.0.0',
					description: 'Test',
					manifest: {
						id: 'plugin-1',
						name: 'Multi-Nav',
						version: '1.0.0',
						frontend: {
							navigation: [
								{ label: 'Last', path: '/last', icon: 'z', position: 'after:admin' },
								{ label: 'First', path: '/first', icon: 'a', position: 'before:dashboard' },
								{ label: 'Middle', path: '/middle', icon: 'm', position: 'after:invoices' }
							]
						}
					}
				}
			}
		];

		vi.spyOn(api, 'listTenantPlugins').mockResolvedValueOnce(mockPlugins as unknown as import('$lib/api').TenantPlugin[]);

		await pluginManager.loadPlugins('tenant-1');

		const nav = pluginManager.getNavigation();
		expect(nav.map((n) => n.label)).toEqual(['First', 'Middle', 'Last']);
	});
});

describe('plugin frontend component registry', () => {
	const component = (() => null) as unknown as PluginFrontendComponent;

	beforeEach(() => {
		clearPluginFrontendComponents();
	});

	it('should normalize safe component references', () => {
		expect(normalizePluginComponentReference('RiskWidget.svelte')).toBe('RiskWidget.svelte');
		expect(normalizePluginComponentReference('risk-tools/RiskWidget.svelte')).toBe(
			'risk-tools/RiskWidget.svelte'
		);
	});

	it('should reject unsafe component references', () => {
		expect(normalizePluginComponentReference()).toBeUndefined();
		expect(normalizePluginComponentReference(' RiskWidget.svelte')).toBeUndefined();
		expect(normalizePluginComponentReference('x'.repeat(161))).toBeUndefined();
		expect(normalizePluginComponentReference('../RiskWidget.svelte')).toBeUndefined();
		expect(normalizePluginComponentReference('./RiskWidget.svelte')).toBeUndefined();
		expect(normalizePluginComponentReference('/RiskWidget.svelte')).toBeUndefined();
		expect(normalizePluginComponentReference('~/RiskWidget.svelte')).toBeUndefined();
		expect(normalizePluginComponentReference('risk\\RiskWidget.svelte')).toBeUndefined();
		expect(
			normalizePluginComponentReference('https://example.com/RiskWidget.svelte')
		).toBeUndefined();
		expect(normalizePluginComponentReference('RiskWidget.svelte?raw')).toBeUndefined();
		expect(normalizePluginComponentReference('RiskWidget.svelte#fragment')).toBeUndefined();
		expect(normalizePluginComponentReference('risk//RiskWidget.svelte')).toBeUndefined();
		expect(normalizePluginComponentReference('risk/Risk Widget.svelte')).toBeUndefined();
		expect(() => registerPluginFrontendComponent('../RiskWidget.svelte', component)).toThrow(
			/Unsafe plugin frontend component id/
		);
	});

	it('should generate deterministic component lookup candidates', () => {
		expect(
			getPluginFrontendComponentCandidateIds({
				pluginId: 'plugin-1',
				pluginName: 'risk-tools',
				slotName: 'dashboard.widgets',
				componentName: 'RiskWidget.svelte',
				componentRef: 'RiskWidget.svelte',
				label: 'Risk widget',
				kind: 'card',
				order: 10
			})
		).toEqual(['plugin-1/RiskWidget.svelte', 'risk-tools/RiskWidget.svelte']);
	});

	it('should deduplicate component lookup candidates', () => {
		expect(
			getPluginFrontendComponentCandidateIds({
				pluginId: 'risk-tools',
				pluginName: 'risk-tools',
				slotName: 'dashboard.widgets',
				componentName: 'RiskWidget.svelte',
				componentRef: 'RiskWidget.svelte',
				label: 'Risk widget',
				kind: 'card',
				order: 10
			})
		).toEqual(['risk-tools/RiskWidget.svelte']);
	});

	it('should register resolve and unregister safe components', () => {
		const registration = {
			pluginId: 'plugin-1',
			pluginName: 'risk-tools',
			slotName: 'dashboard.widgets',
			componentName: 'RiskWidget.svelte',
			componentRef: 'RiskWidget.svelte',
			label: 'Risk widget',
			kind: 'card',
			order: 10
		} as const;

		registerPluginFrontendComponent('plugin-1/RiskWidget.svelte', component);

		expect(resolvePluginFrontendComponent(registration)).toBe(component);
		expect(() => registerPluginFrontendComponent(' risk-tools/RiskWidget.svelte ', component)).toThrow(
			/Unsafe plugin frontend component id/
		);
		expect(resolvePluginFrontendComponent({ ...registration, componentRef: undefined })).toBeUndefined();

		unregisterPluginFrontendComponent(' unsafe/component ');
		expect(resolvePluginFrontendComponent(registration)).toBe(component);

		unregisterPluginFrontendComponent('plugin-1/RiskWidget.svelte');
		expect(resolvePluginFrontendComponent(registration)).toBeUndefined();
	});
});

describe('pluginManager.reload', () => {
	beforeEach(() => {
		pluginManager.clear();
		vi.clearAllMocks();
	});

	it('should reload plugins for current tenant', async () => {
		const mockPlugins = [
			{
				id: 'tp-1',
				tenant_id: 'tenant-1',
				plugin_id: 'plugin-1',
				is_enabled: true,
				config: {},
				settings: {},
				created_at: '2024-01-01T00:00:00Z',
				updated_at: '2024-01-01T00:00:00Z',
				plugin: {
					id: 'plugin-1',
					name: 'Test Plugin',
					version: '1.0.0',
					description: 'Test',
					manifest: { id: 'plugin-1', name: 'Test', version: '1.0.0' }
				}
			}
		];

		const spy = vi.spyOn(api, 'listTenantPlugins').mockResolvedValue(mockPlugins as unknown as import('$lib/api').TenantPlugin[]);

		// First load
		await pluginManager.loadPlugins('tenant-1');
		expect(spy).toHaveBeenCalledTimes(1);

		// Reload should fetch again
		await pluginManager.reload();
		expect(spy).toHaveBeenCalledTimes(2);
	});

	it('should do nothing when no tenant loaded', async () => {
		const spy = vi.spyOn(api, 'listTenantPlugins').mockResolvedValue([]);

		await pluginManager.reload();

		expect(spy).not.toHaveBeenCalled();
	});
});

describe('parsePosition edge cases', () => {
	it('should handle non-matching position formats', () => {
		// This should hit the final return 1000 on line 237
		expect(parsePosition('invalid:format')).toBe(1000);
		expect(parsePosition('something_else')).toBe(1000);
	});
});

describe('SLOT_NAMES', () => {
	it('should export all slot names', () => {
		expect(SLOT_NAMES.DASHBOARD_WIDGETS).toBe('dashboard.widgets');
		expect(SLOT_NAMES.DASHBOARD_ACTIONS).toBe('dashboard.actions');
		expect(SLOT_NAMES.INVOICE_SIDEBAR).toBe('invoice.sidebar');
		expect(SLOT_NAMES.INVOICE_ACTIONS).toBe('invoice.actions');
		expect(SLOT_NAMES.CONTACT_SIDEBAR).toBe('contact.sidebar');
		expect(SLOT_NAMES.PAYMENT_SIDEBAR).toBe('payment.sidebar');
		expect(SLOT_NAMES.SETTINGS_TABS).toBe('settings.tabs');
		expect(SLOT_NAMES.REPORTS_CUSTOM).toBe('reports.custom');
		expect(SLOT_NAMES.HEADER_ACTIONS).toBe('header.actions');
	});

	it('should provide type safety for slot names', () => {
		const slotName: SlotName = SLOT_NAMES.DASHBOARD_WIDGETS;
		expect(slotName).toBeDefined();
	});
});
