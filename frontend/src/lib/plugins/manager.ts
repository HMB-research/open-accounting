import { api, type TenantPlugin, type PluginNavItem } from '$lib/api';
import { normalizePluginComponentReference } from './componentRegistry';

/**
 * Plugin navigation item with plugin metadata
 */
export interface PluginNavigationItem extends PluginNavItem {
	pluginId: string;
	pluginName: string;
}

export type PluginSlotKind = 'card' | 'link' | 'action';

/**
 * Slot registration from a plugin
 */
export interface PluginSlotRegistration {
	pluginId: string;
	pluginName: string;
	slotName: string;
	componentName: string;
	componentRef?: string;
	label: string;
	description?: string;
	path?: string;
	kind: PluginSlotKind;
	badge?: string;
	order: number;
}

/**
 * Plugin Manager State
 */
export interface PluginManagerState {
	plugins: TenantPlugin[];
	navigation: PluginNavigationItem[];
	slots: Map<string, PluginSlotRegistration[]>;
	isLoaded: boolean;
	currentTenantId: string | null;
}

type EnabledTenantPlugin = TenantPlugin & { plugin: NonNullable<TenantPlugin['plugin']> };

/**
 * Creates a plugin manager for managing loaded plugins and their contributions
 */
function createPluginManager() {
	let state: PluginManagerState = {
		plugins: [],
		navigation: [],
		slots: new Map(),
		isLoaded: false,
		currentTenantId: null
	};

	// Subscribers for reactive updates
	const subscribers: Set<(state: PluginManagerState) => void> = new Set();

	function notify() {
		subscribers.forEach((fn) => fn(state));
	}

	/**
	 * Subscribe to state changes
	 */
	function subscribe(fn: (state: PluginManagerState) => void): () => void {
		subscribers.add(fn);
		fn(state); // Initial call
		return () => subscribers.delete(fn);
	}

	/**
	 * Load plugins for a tenant
	 */
	async function loadPlugins(tenantId: string): Promise<void> {
		// Skip if already loaded for this tenant
		if (state.isLoaded && state.currentTenantId === tenantId) {
			return;
		}

		try {
			const plugins = await api.listTenantPlugins(tenantId) ?? [];
			const enabledPlugins = plugins.filter(
				(p): p is EnabledTenantPlugin => p.is_enabled && p.plugin != null
			);

			// Extract navigation items from enabled plugins
			const navigation: PluginNavigationItem[] = [];
			const slots = new Map<string, PluginSlotRegistration[]>();

			for (const tenantPlugin of enabledPlugins) {
				const plugin = tenantPlugin.plugin;

				// Extract navigation items
				if (plugin.manifest?.frontend?.navigation) {
					for (const navItem of plugin.manifest.frontend.navigation) {
						navigation.push({
							...navItem,
							pluginId: plugin.id,
							pluginName: plugin.name
						});
					}
				}

				// Extract slot registrations
				if (plugin.manifest?.frontend?.slots) {
					for (const slot of plugin.manifest.frontend.slots) {
						const existing = slots.get(slot.name) || [];
						const path = normalizeSlotPath(slot.path);
						existing.push({
							pluginId: plugin.id,
							pluginName: plugin.name,
							slotName: slot.name,
							componentName: slot.component,
							componentRef: normalizePluginComponentReference(slot.component),
							label: slot.label || slot.component,
							description: slot.description,
							path,
							kind: normalizeSlotKind(slot.kind, path),
							badge: slot.badge,
							order: parseSlotOrder(slot.order)
						});
						existing.sort(compareSlotRegistrations);
						slots.set(slot.name, existing);
					}
				}
			}

			// Sort navigation by position hints
			navigation.sort((a, b) => {
				const posA = parsePosition(a.position);
				const posB = parsePosition(b.position);
				return posA - posB;
			});

			state = {
				plugins: enabledPlugins,
				navigation,
				slots,
				isLoaded: true,
				currentTenantId: tenantId
			};

			notify();
		} catch (error) {
			console.error('Failed to load plugins:', error);
			// Keep existing state on error
		}
	}

	/**
	 * Clear loaded plugins (e.g., on logout or tenant switch)
	 */
	function clear(): void {
		state = {
			plugins: [],
			navigation: [],
			slots: new Map(),
			isLoaded: false,
			currentTenantId: null
		};
		notify();
	}

	/**
	 * Get navigation items for the current tenant
	 */
	function getNavigation(): PluginNavigationItem[] {
		return state.navigation;
	}

	/**
	 * Get slot registrations for a specific slot name
	 */
	function getSlotRegistrations(slotName: string): PluginSlotRegistration[] {
		return state.slots.get(slotName) || [];
	}

	/**
	 * Check if a slot has any registered components
	 */
	function hasSlotContent(slotName: string): boolean {
		return (state.slots.get(slotName)?.length || 0) > 0;
	}

	/**
	 * Get all enabled plugins
	 */
	function getEnabledPlugins(): TenantPlugin[] {
		return state.plugins;
	}

	/**
	 * Check if plugins are loaded
	 */
	function isLoaded(): boolean {
		return state.isLoaded;
	}

	/**
	 * Reload plugins for the current tenant
	 */
	async function reload(): Promise<void> {
		if (state.currentTenantId) {
			state.isLoaded = false;
			await loadPlugins(state.currentTenantId);
		}
	}

	return {
		subscribe,
		loadPlugins,
		clear,
		getNavigation,
		getSlotRegistrations,
		hasSlotContent,
		getEnabledPlugins,
		isLoaded,
		reload
	};
}

/**
 * Parse position hint string into numeric sort order
 * Format: "after:invoices" or "before:reports" or numeric "100"
 */
function parsePosition(position?: string): number {
	if (!position) return 1000; // Default to end

	// Numeric position
	const num = parseInt(position, 10);
	if (!isNaN(num)) return num;

	// Named positions (simplified - could be expanded with actual menu item positions)
	const knownPositions: Record<string, number> = {
		dashboard: 100,
		accounts: 200,
		journal: 300,
		contacts: 400,
		invoices: 500,
		payments: 600,
		reports: 700,
		payroll: 800,
		admin: 900
	};

	if (position.startsWith('after:')) {
		const target = position.substring(6);
		const targetPos = knownPositions[target] || 500;
		return targetPos + 50;
	}

	if (position.startsWith('before:')) {
		const target = position.substring(7);
		const targetPos = knownPositions[target] || 500;
		return targetPos - 50;
	}

	return 1000;
}

function normalizeSlotKind(kind?: string, path?: string): PluginSlotKind {
	if (kind === 'card' || kind === 'link' || kind === 'action') {
		return kind;
	}
	return path ? 'link' : 'card';
}

function normalizeSlotPath(path?: string): string | undefined {
	if (!path || !path.startsWith('/') || path.startsWith('//')) {
		return undefined;
	}
	return path;
}

function parseSlotOrder(order?: number): number {
	return typeof order === 'number' && Number.isFinite(order) ? order : 1000;
}

function compareSlotRegistrations(a: PluginSlotRegistration, b: PluginSlotRegistration): number {
	if (a.order !== b.order) {
		return a.order - b.order;
	}
	if (a.pluginName !== b.pluginName) {
		return a.pluginName.localeCompare(b.pluginName);
	}
	return a.label.localeCompare(b.label);
}

// Export singleton instance
export const pluginManager = createPluginManager();

// Export parsePosition for testing
export { parsePosition };
