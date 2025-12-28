<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type TenantMembership, type Tenant } from '$lib/api';

	let tenants = $state<TenantMembership[]>([]);
	let selectedTenant = $state<Tenant | null>(null);
	let isLoading = $state(true);
	let error = $state('');
	let showCreateTenant = $state(false);
	let newTenantName = $state('');
	let newTenantSlug = $state('');

	onMount(async () => {
		try {
			tenants = await api.getMyTenants();
			if (tenants.length > 0) {
				selectedTenant = tenants.find((t) => t.is_default)?.tenant || tenants[0].tenant;
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load tenants';
		} finally {
			isLoading = false;
		}
	});

	async function createTenant(e: Event) {
		e.preventDefault();
		error = '';

		try {
			const tenant = await api.createTenant(newTenantName, newTenantSlug);
			tenants = [...tenants, { tenant, role: 'owner', is_default: tenants.length === 0 }];
			selectedTenant = tenant;
			showCreateTenant = false;
			newTenantName = '';
			newTenantSlug = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create organization';
		}
	}

	function generateSlug() {
		newTenantSlug = newTenantName
			.toLowerCase()
			.replace(/[^a-z0-9]+/g, '-')
			.replace(/^-|-$/g, '');
	}
</script>

<svelte:head>
	<title>Dashboard - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>Dashboard</h1>
		<button class="btn btn-primary" onclick={() => (showCreateTenant = true)}>
			+ New Organization
		</button>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<p>Loading...</p>
	{:else if tenants.length === 0}
		<div class="card empty-state">
			<h2>Welcome to Open Accounting!</h2>
			<p>Create your first organization to get started.</p>
			<button class="btn btn-primary" onclick={() => (showCreateTenant = true)}>
				Create Organization
			</button>
		</div>
	{:else}
		<div class="grid">
			<div class="card">
				<h3>Organizations</h3>
				<ul class="tenant-list">
					{#each tenants as membership}
						<li class:selected={selectedTenant?.id === membership.tenant.id}>
							<button
								class="tenant-item"
								onclick={() => (selectedTenant = membership.tenant)}
							>
								<span class="tenant-name">{membership.tenant.name}</span>
								<span class="tenant-role badge">{membership.role}</span>
							</button>
						</li>
					{/each}
				</ul>
			</div>

			{#if selectedTenant}
				<div class="card org-details">
					<h3>{selectedTenant.name}</h3>
					<dl class="details-grid">
						<dt>Currency</dt>
						<dd>{selectedTenant.settings.default_currency}</dd>
						<dt>Country</dt>
						<dd>{selectedTenant.settings.country_code}</dd>
						<dt>Timezone</dt>
						<dd>{selectedTenant.settings.timezone}</dd>
					</dl>

					<div class="quick-links">
						<a href="/accounts?tenant={selectedTenant.id}" class="btn btn-secondary">
							Chart of Accounts
						</a>
						<a href="/journal?tenant={selectedTenant.id}" class="btn btn-secondary">
							Journal Entries
						</a>
						<a href="/reports?tenant={selectedTenant.id}" class="btn btn-secondary">
							Reports
						</a>
					</div>
				</div>
			{/if}
		</div>
	{/if}
</div>

{#if showCreateTenant}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showCreateTenant = false)} role="presentation">
		<div class="modal card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="create-org-title" tabindex="-1">
			<h2 id="create-org-title">Create Organization</h2>
			<form onsubmit={createTenant}>
				<div class="form-group">
					<label class="label" for="name">Organization Name</label>
					<input
						class="input"
						type="text"
						id="name"
						bind:value={newTenantName}
						oninput={generateSlug}
						required
						placeholder="My Company"
					/>
				</div>

				<div class="form-group">
					<label class="label" for="slug">URL Identifier</label>
					<input
						class="input"
						type="text"
						id="slug"
						bind:value={newTenantSlug}
						required
						pattern="[a-z0-9][a-z0-9-]*[a-z0-9]"
						placeholder="my-company"
					/>
					<small>Only lowercase letters, numbers, and hyphens</small>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={() => (showCreateTenant = false)}>
						Cancel
					</button>
					<button type="submit" class="btn btn-primary">Create</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<style>
	.header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 2rem;
	}

	h1 {
		font-size: 1.75rem;
	}

	.grid {
		display: grid;
		grid-template-columns: 300px 1fr;
		gap: 1.5rem;
	}

	.empty-state {
		text-align: center;
		padding: 3rem;
	}

	.empty-state h2 {
		margin-bottom: 0.5rem;
	}

	.empty-state p {
		color: var(--color-text-muted);
		margin-bottom: 1.5rem;
	}

	.tenant-list {
		list-style: none;
	}

	.tenant-list li {
		border-radius: 0.375rem;
	}

	.tenant-list li.selected {
		background: var(--color-bg);
	}

	.tenant-item {
		width: 100%;
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 0.75rem;
		background: none;
		border: none;
		text-align: left;
	}

	.tenant-item:hover {
		background: var(--color-bg);
	}

	.tenant-name {
		font-weight: 500;
	}

	.tenant-role {
		background: var(--color-bg);
		color: var(--color-text-muted);
	}

	.org-details h3 {
		margin-bottom: 1rem;
	}

	.details-grid {
		display: grid;
		grid-template-columns: auto 1fr;
		gap: 0.5rem 1rem;
		margin-bottom: 1.5rem;
	}

	.details-grid dt {
		font-weight: 500;
		color: var(--color-text-muted);
	}

	.quick-links {
		display: flex;
		gap: 0.5rem;
		flex-wrap: wrap;
	}

	.modal-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.5);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 100;
	}

	.modal {
		width: 100%;
		max-width: 400px;
		margin: 1rem;
	}

	.modal h2 {
		margin-bottom: 1.5rem;
	}

	.modal small {
		color: var(--color-text-muted);
		font-size: 0.75rem;
	}

	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1.5rem;
	}
</style>
