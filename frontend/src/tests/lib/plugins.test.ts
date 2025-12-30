import { describe, it, expect, vi, beforeEach } from 'vitest';
import { parsePosition, pluginManager } from '$lib/plugins/manager';
import { SLOT_NAMES, type SlotName } from '$lib/plugins';
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
									component: 'TestWidget'
								}
							]
						}
					}
				}
			}
		];

		vi.spyOn(api, 'listTenantPlugins').mockResolvedValueOnce(mockPlugins);

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
	});

	it('should skip already loaded tenant', async () => {
		const mockPlugins = [
			{
				id: 'tp-1',
				tenant_id: 'tenant-1',
				plugin_id: 'plugin-1',
				is_enabled: true,
				config: {},
				plugin: {
					id: 'plugin-1',
					name: 'Test Plugin',
					version: '1.0.0',
					description: 'A test plugin',
					manifest: { id: 'plugin-1', name: 'Test', version: '1.0.0' }
				}
			}
		];

		const spy = vi.spyOn(api, 'listTenantPlugins').mockResolvedValue(mockPlugins);

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
				plugin: {
					id: 'plugin-1',
					name: 'Test Plugin',
					version: '1.0.0',
					description: 'Test',
					manifest: { id: 'plugin-1', name: 'Test', version: '1.0.0' }
				}
			}
		];

		const spy = vi.spyOn(api, 'listTenantPlugins').mockResolvedValue(mockPlugins);

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

		vi.spyOn(api, 'listTenantPlugins').mockResolvedValueOnce(mockPlugins);

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
				plugin: {
					id: 'plugin-1',
					name: 'Plugin Without Manifest',
					version: '1.0.0',
					description: 'Test',
					manifest: null
				}
			}
		];

		vi.spyOn(api, 'listTenantPlugins').mockResolvedValueOnce(mockPlugins);

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

		vi.spyOn(api, 'listTenantPlugins').mockResolvedValueOnce(mockPlugins);

		await pluginManager.loadPlugins('tenant-1');

		const nav = pluginManager.getNavigation();
		expect(nav.map((n) => n.label)).toEqual(['First', 'Middle', 'Last']);
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
				plugin: {
					id: 'plugin-1',
					name: 'Test Plugin',
					version: '1.0.0',
					description: 'Test',
					manifest: { id: 'plugin-1', name: 'Test', version: '1.0.0' }
				}
			}
		];

		const spy = vi.spyOn(api, 'listTenantPlugins').mockResolvedValue(mockPlugins);

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
