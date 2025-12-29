// Plugin system exports
export { pluginManager } from './manager';
export type {
	PluginNavigationItem,
	PluginSlotRegistration,
	PluginManagerState
} from './manager';

// Re-export Slot component
export { default as Slot } from './Slot.svelte';

/**
 * Available slot names in the application
 *
 * Plugins can register components for these slots in their manifest:
 *
 * ```yaml
 * frontend:
 *   slots:
 *     - name: dashboard.widgets
 *       component: MyWidget.svelte
 * ```
 */
export const SLOT_NAMES = {
	/** Dashboard widget area - displays cards/charts on the main dashboard */
	DASHBOARD_WIDGETS: 'dashboard.widgets',

	/** Dashboard quick action buttons */
	DASHBOARD_ACTIONS: 'dashboard.actions',

	/** Invoice detail page sidebar */
	INVOICE_SIDEBAR: 'invoice.sidebar',

	/** Invoice action buttons (next to Send, Void, etc.) */
	INVOICE_ACTIONS: 'invoice.actions',

	/** Contact detail page sidebar */
	CONTACT_SIDEBAR: 'contact.sidebar',

	/** Payment detail page sidebar */
	PAYMENT_SIDEBAR: 'payment.sidebar',

	/** Settings page additional tabs */
	SETTINGS_TABS: 'settings.tabs',

	/** Reports page custom report options */
	REPORTS_CUSTOM: 'reports.custom',

	/** Global header area (next to logout button) */
	HEADER_ACTIONS: 'header.actions'
} as const;

export type SlotName = (typeof SLOT_NAMES)[keyof typeof SLOT_NAMES];
