<script lang="ts">
	/**
	 * Tenant selector dropdown for switching between organizations.
	 * Fetches user's tenant memberships on mount and allows switching
	 * the active tenant via URL query parameter.
	 *
	 * @example
	 * ```svelte
	 * <TenantSelector />
	 * ```
	 */
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { api, type TenantMembership } from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';

	let tenants = $state<TenantMembership[]>([]);
	let isLoading = $state(true);
	let isOpen = $state(false);

	let selectedTenant = $derived(
		tenants.find((t) => t.tenant.id === $page.url.searchParams.get('tenant'))
	);

	$effect(() => {
		loadTenants();
	});

	async function loadTenants() {
		try {
			isLoading = true;
			tenants = await api.getMyTenants();
		} catch {
			tenants = [];
		} finally {
			isLoading = false;
		}
	}

	function selectTenant(tenantId: string) {
		const url = new URL($page.url);
		url.searchParams.set('tenant', tenantId);
		goto(url.toString(), { replaceState: true, noScroll: true });
		isOpen = false;
	}

	function toggleDropdown() {
		isOpen = !isOpen;
	}

	function handleClickOutside(event: MouseEvent) {
		const target = event.target as HTMLElement;
		if (!target.closest('.tenant-selector')) {
			isOpen = false;
		}
	}

	$effect(() => {
		if (isOpen) {
			document.addEventListener('click', handleClickOutside);
			return () => document.removeEventListener('click', handleClickOutside);
		}
	});
</script>

<div class="tenant-selector" class:open={isOpen}>
	<button class="tenant-trigger" onclick={toggleDropdown} disabled={isLoading || tenants.length === 0}>
		<span class="tenant-icon">
			<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg>
		</span>
		<span class="tenant-name">
			{#if isLoading}
				...
			{:else if selectedTenant}
				{selectedTenant.tenant.name}
			{:else if tenants.length === 0}
				{m.nav_noOrganizations?.() || 'No organizations'}
			{:else}
				{m.nav_selectOrganization?.() || 'Select Organization'}
			{/if}
		</span>
		<span class="tenant-arrow">
			<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"/></svg>
		</span>
	</button>

	{#if isOpen && tenants.length > 0}
		<div class="tenant-dropdown">
			{#each tenants as membership (membership.tenant.id)}
				<button
					class="tenant-option"
					class:selected={membership.tenant.id === selectedTenant?.tenant.id}
					onclick={() => selectTenant(membership.tenant.id)}
				>
					<span class="option-name">{membership.tenant.name}</span>
					<span class="option-role">{membership.role}</span>
				</button>
			{/each}
		</div>
	{/if}
</div>

<style>
	.tenant-selector {
		position: relative;
	}

	.tenant-trigger {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.5rem 0.75rem;
		background: var(--color-bg);
		border: 1px solid var(--color-border);
		border-radius: 0.375rem;
		cursor: pointer;
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--color-text);
		transition: border-color 0.15s ease, background-color 0.15s ease;
		min-width: 160px;
		max-width: 220px;
	}

	.tenant-trigger:hover:not(:disabled) {
		border-color: var(--color-primary);
		background: var(--color-surface);
	}

	.tenant-trigger:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.tenant-icon {
		display: flex;
		color: var(--color-text-muted);
	}

	.tenant-name {
		flex: 1;
		text-align: left;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.tenant-arrow {
		display: flex;
		color: var(--color-text-muted);
		transition: transform 0.15s ease;
	}

	.open .tenant-arrow {
		transform: rotate(180deg);
	}

	.tenant-dropdown {
		position: absolute;
		top: calc(100% + 0.25rem);
		left: 0;
		right: 0;
		background: var(--color-surface);
		border: 1px solid var(--color-border);
		border-radius: 0.375rem;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
		z-index: 100;
		max-height: 300px;
		overflow-y: auto;
	}

	.tenant-option {
		display: flex;
		align-items: center;
		justify-content: space-between;
		width: 100%;
		padding: 0.625rem 0.75rem;
		background: none;
		border: none;
		cursor: pointer;
		font-size: 0.875rem;
		text-align: left;
		color: var(--color-text);
		transition: background-color 0.15s ease;
	}

	.tenant-option:hover {
		background: var(--color-bg);
	}

	.tenant-option.selected {
		background: var(--color-primary-bg, rgba(37, 99, 235, 0.1));
		color: var(--color-primary);
	}

	.option-name {
		font-weight: 500;
	}

	.option-role {
		font-size: 0.75rem;
		color: var(--color-text-muted);
		text-transform: capitalize;
	}

	.tenant-option.selected .option-role {
		color: var(--color-primary);
		opacity: 0.8;
	}
</style>
