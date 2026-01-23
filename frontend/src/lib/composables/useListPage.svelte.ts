/**
 * Composable for common list page state management.
 * Encapsulates loading states, error handling, success messages,
 * filtering, and data fetching patterns.
 */

export interface UseListPageConfig<TItem, TFilter, TDeps = Record<string, unknown>> {
	/** Function to fetch items from the API */
	fetchItems: (tenantId: string, filter?: TFilter) => Promise<TItem[]>;
	/** Optional function to fetch dependencies (contacts, accounts, etc.) */
	fetchDependencies?: (tenantId: string) => Promise<TDeps>;
	/** Initial filter values */
	initialFilter?: TFilter;
	/** Auto-clear success message after this many milliseconds (default: 3000) */
	successTimeout?: number;
}

export interface UseListPageReturn<TItem, TFilter, TDeps> {
	/** Array of items */
	items: TItem[];
	/** Dependencies (contacts, etc.) */
	dependencies: TDeps;
	/** Whether initial data is loading */
	isLoading: boolean;
	/** Error message */
	error: string;
	/** Success message */
	success: string;
	/** Whether an action is in progress */
	actionLoading: boolean;
	/** Current filter state */
	filter: TFilter;
	/** Load data for a tenant */
	loadData: (tenantId: string) => Promise<void>;
	/** Apply current filter and reload */
	handleFilter: (tenantId: string) => Promise<void>;
	/** Set items directly (for optimistic updates) */
	setItems: (items: TItem[]) => void;
	/** Set error message */
	setError: (message: string) => void;
	/** Set success message with optional auto-clear */
	setSuccess: (message: string, autoClear?: boolean) => void;
	/** Clear error message */
	clearError: () => void;
	/** Clear success message */
	clearSuccess: () => void;
	/** Set action loading state */
	setActionLoading: (loading: boolean) => void;
	/** Update filter */
	setFilter: (newFilter: Partial<TFilter>) => void;
}

/**
 * Create list page state management with Svelte 5 runes.
 *
 * @example
 * ```typescript
 * const listPage = useListPage({
 *   fetchItems: (tenantId, filter) => api.listInvoices(tenantId, filter),
 *   fetchDependencies: (tenantId) => api.listContacts(tenantId, { active_only: true }),
 *   initialFilter: { status: '', from_date: '', to_date: '' }
 * });
 *
 * // In $effect:
 * listPage.loadData(tenantId);
 *
 * // On filter change:
 * listPage.handleFilter(tenantId);
 * ```
 */
export function useListPage<TItem, TFilter extends object, TDeps = Record<string, unknown>>(
	config: UseListPageConfig<TItem, TFilter, TDeps>
): UseListPageReturn<TItem, TFilter, TDeps> {
	const { fetchItems, fetchDependencies, initialFilter, successTimeout = 3000 } = config;

	// State using Svelte 5 runes - these need to be in a .svelte.ts file
	let items = $state<TItem[]>([]);
	let dependencies = $state<TDeps>({} as TDeps);
	let isLoading = $state(true);
	let error = $state('');
	let success = $state('');
	let actionLoading = $state(false);
	let filter = $state<TFilter>((initialFilter || {}) as TFilter);

	let successTimeoutId: ReturnType<typeof setTimeout> | null = null;

	async function loadData(tenantId: string): Promise<void> {
		isLoading = true;
		error = '';

		try {
			const promises: Promise<unknown>[] = [fetchItems(tenantId, filter)];

			if (fetchDependencies) {
				promises.push(fetchDependencies(tenantId));
			}

			const results = await Promise.all(promises);
			items = results[0] as TItem[];

			if (fetchDependencies && results[1]) {
				dependencies = results[1] as TDeps;
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load data';
		} finally {
			isLoading = false;
		}
	}

	async function handleFilter(tenantId: string): Promise<void> {
		await loadData(tenantId);
	}

	function setItems(newItems: TItem[]): void {
		items = newItems;
	}

	function setError(message: string): void {
		error = message;
	}

	function setSuccess(message: string, autoClear = true): void {
		success = message;
		if (autoClear) {
			if (successTimeoutId) {
				clearTimeout(successTimeoutId);
			}
			successTimeoutId = setTimeout(() => {
				success = '';
				successTimeoutId = null;
			}, successTimeout);
		}
	}

	function clearError(): void {
		error = '';
	}

	function clearSuccess(): void {
		if (successTimeoutId) {
			clearTimeout(successTimeoutId);
			successTimeoutId = null;
		}
		success = '';
	}

	function setActionLoading(loading: boolean): void {
		actionLoading = loading;
	}

	function setFilter(newFilter: Partial<TFilter>): void {
		filter = { ...filter, ...newFilter };
	}

	return {
		get items() {
			return items;
		},
		get dependencies() {
			return dependencies;
		},
		get isLoading() {
			return isLoading;
		},
		get error() {
			return error;
		},
		get success() {
			return success;
		},
		get actionLoading() {
			return actionLoading;
		},
		get filter() {
			return filter;
		},
		loadData,
		handleFilter,
		setItems,
		setError,
		setSuccess,
		clearError,
		clearSuccess,
		setActionLoading,
		setFilter
	};
}
