<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { api, type Account } from '$lib/api';

	let accounts = $state<Account[]>([]);
	let isLoading = $state(true);
	let error = $state('');
	let showCreateAccount = $state(false);

	// New account form
	let newCode = $state('');
	let newName = $state('');
	let newType = $state<Account['account_type']>('ASSET');
	let newDescription = $state('');

	$effect(() => {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadAccounts(tenantId);
		}
	});

	async function loadAccounts(tenantId: string) {
		isLoading = true;
		error = '';

		try {
			accounts = await api.listAccounts(tenantId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load accounts';
		} finally {
			isLoading = false;
		}
	}

	async function createAccount(e: Event) {
		e.preventDefault();
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const account = await api.createAccount(tenantId, {
				code: newCode,
				name: newName,
				account_type: newType,
				description: newDescription || undefined
			});
			accounts = [...accounts, account].sort((a, b) => a.code.localeCompare(b.code));
			showCreateAccount = false;
			newCode = '';
			newName = '';
			newType = 'ASSET';
			newDescription = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create account';
		}
	}

	function groupByType(accounts: Account[]) {
		const groups: Record<Account['account_type'], Account[]> = {
			ASSET: [],
			LIABILITY: [],
			EQUITY: [],
			REVENUE: [],
			EXPENSE: []
		};

		for (const account of accounts) {
			groups[account.account_type].push(account);
		}

		return groups;
	}

	const typeLabels: Record<Account['account_type'], string> = {
		ASSET: 'Assets',
		LIABILITY: 'Liabilities',
		EQUITY: 'Equity',
		REVENUE: 'Revenue',
		EXPENSE: 'Expenses'
	};
</script>

<svelte:head>
	<title>Chart of Accounts - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>Chart of Accounts</h1>
		<button class="btn btn-primary" onclick={() => (showCreateAccount = true)}>
			+ New Account
		</button>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<p>Loading accounts...</p>
	{:else}
		{@const groups = groupByType(accounts)}
		{#each Object.entries(typeLabels) as [type, label]}
			{@const typeAccounts = groups[type as Account['account_type']]}
			{#if typeAccounts.length > 0}
				<section class="account-section card">
					<h2>{label}</h2>
					<table class="table">
						<thead>
							<tr>
								<th>Code</th>
								<th>Name</th>
								<th>Description</th>
								<th>Status</th>
							</tr>
						</thead>
						<tbody>
							{#each typeAccounts as account}
								<tr class:inactive={!account.is_active}>
									<td class="code">{account.code}</td>
									<td>{account.name}</td>
									<td class="description">{account.description || '-'}</td>
									<td>
										{#if account.is_system}
											<span class="badge badge-system">System</span>
										{:else if !account.is_active}
											<span class="badge badge-inactive">Inactive</span>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</section>
			{/if}
		{/each}
	{/if}
</div>

{#if showCreateAccount}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showCreateAccount = false)} role="presentation">
		<div class="modal card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="create-account-title" tabindex="-1">
			<h2 id="create-account-title">Create Account</h2>
			<form onsubmit={createAccount}>
				<div class="form-group">
					<label class="label" for="code">Account Code</label>
					<input
						class="input"
						type="text"
						id="code"
						bind:value={newCode}
						required
						placeholder="1100"
					/>
				</div>

				<div class="form-group">
					<label class="label" for="name">Account Name</label>
					<input
						class="input"
						type="text"
						id="name"
						bind:value={newName}
						required
						placeholder="Cash and Bank"
					/>
				</div>

				<div class="form-group">
					<label class="label" for="type">Account Type</label>
					<select class="input" id="type" bind:value={newType}>
						<option value="ASSET">Asset</option>
						<option value="LIABILITY">Liability</option>
						<option value="EQUITY">Equity</option>
						<option value="REVENUE">Revenue</option>
						<option value="EXPENSE">Expense</option>
					</select>
				</div>

				<div class="form-group">
					<label class="label" for="description">Description</label>
					<input
						class="input"
						type="text"
						id="description"
						bind:value={newDescription}
						placeholder="Optional description"
					/>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={() => (showCreateAccount = false)}>
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

	.account-section {
		margin-bottom: 1.5rem;
	}

	.account-section h2 {
		font-size: 1.125rem;
		margin-bottom: 1rem;
		color: var(--color-text-muted);
	}

	.code {
		font-family: var(--font-mono);
		font-weight: 500;
	}

	.description {
		color: var(--color-text-muted);
	}

	.inactive {
		opacity: 0.6;
	}

	.badge-system {
		background: #e0e7ff;
		color: #3730a3;
	}

	.badge-inactive {
		background: #f3f4f6;
		color: #6b7280;
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

	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1.5rem;
	}
</style>
