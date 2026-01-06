<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { api, type Account, type TrialBalance } from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';

	let accounts = $state<Account[]>([]);
	let accountBalances = $state<Map<string, { debit: Decimal; credit: Decimal; balance: Decimal }>>(new Map());
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
			// Load accounts and trial balance in parallel
			const [accountsData, trialBalance] = await Promise.all([
				api.listAccounts(tenantId),
				api.getTrialBalance(tenantId).catch(() => null) // Don't fail if trial balance fails
			]);

			accounts = accountsData;

			// Build balance map from trial balance
			if (trialBalance) {
				const balanceMap = new Map<string, { debit: Decimal; credit: Decimal; balance: Decimal }>();
				for (const item of trialBalance.accounts) {
					const debit = item.debit_balance instanceof Decimal ? item.debit_balance : new Decimal(item.debit_balance || 0);
					const credit = item.credit_balance instanceof Decimal ? item.credit_balance : new Decimal(item.credit_balance || 0);
					const balance = item.net_balance instanceof Decimal ? item.net_balance : new Decimal(item.net_balance || 0);
					balanceMap.set(item.account_id, { debit, credit, balance });
				}
				accountBalances = balanceMap;
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load accounts';
		} finally {
			isLoading = false;
		}
	}

	function formatBalance(accountId: string, accountType: Account['account_type']): string {
		const balanceData = accountBalances.get(accountId);
		if (!balanceData) return '-';

		// For assets and expenses, positive is debit (normal)
		// For liabilities, equity, and revenue, positive is credit (normal)
		let balance = balanceData.balance;
		if (accountType === 'LIABILITY' || accountType === 'EQUITY' || accountType === 'REVENUE') {
			balance = balance.neg(); // Show as positive for credit-normal accounts
		}

		if (balance.isZero()) return '-';

		return new Intl.NumberFormat('et-EE', {
			style: 'currency',
			currency: 'EUR',
			minimumFractionDigits: 2
		}).format(balance.toNumber());
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

	function getTypeLabel(type: Account['account_type']): string {
		switch (type) {
			case 'ASSET': return m.accounts_assets();
			case 'LIABILITY': return m.accounts_liabilities();
			case 'EQUITY': return m.accounts_equities();
			case 'REVENUE': return m.accounts_revenues();
			case 'EXPENSE': return m.accounts_expenses();
		}
	}

	const typeOrder: Account['account_type'][] = ['ASSET', 'LIABILITY', 'EQUITY', 'REVENUE', 'EXPENSE'];
</script>

<svelte:head>
	<title>{m.accounts_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="page-header">
		<h1>{m.accounts_title()}</h1>
		<div class="page-actions">
			<button class="btn btn-primary" onclick={() => (showCreateAccount = true)}>
				+ {m.accounts_newAccount()}
			</button>
		</div>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<p>{m.common_loading()}</p>
	{:else}
		{@const groups = groupByType(accounts)}
		{#each typeOrder as type}
			{@const typeAccounts = groups[type]}
			{#if typeAccounts.length > 0}
				<section class="account-section card">
					<h2>{getTypeLabel(type)}</h2>
					<div class="table-container">
						<table class="table table-mobile-cards">
							<thead>
								<tr>
									<th>{m.accounts_code()}</th>
									<th>{m.common_name()}</th>
									<th class="hide-mobile">{m.common_description()}</th>
									<th class="balance-col">{m.common_balance()}</th>
									<th>{m.common_status()}</th>
								</tr>
							</thead>
							<tbody>
								{#each typeAccounts as account}
									<tr class:inactive={!account.is_active}>
										<td class="code" data-label="Code">{account.code}</td>
										<td data-label="Name">{account.name}</td>
										<td class="description hide-mobile" data-label="Description">{account.description || '-'}</td>
										<td class="balance" data-label="Balance">{formatBalance(account.id, account.account_type)}</td>
										<td data-label="Status">
											{#if account.is_system}
												<span class="badge badge-system">{m.accounts_system()}</span>
											{:else if !account.is_active}
												<span class="badge badge-inactive">{m.accounts_inactive()}</span>
											{/if}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
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
			<h2 id="create-account-title">{m.accounts_newAccount()}</h2>
			<form onsubmit={createAccount}>
				<div class="form-group">
					<label class="label" for="code">{m.accounts_accountCode()}</label>
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
					<label class="label" for="name">{m.accounts_accountName()}</label>
					<input
						class="input"
						type="text"
						id="name"
						bind:value={newName}
						required
						placeholder={m.accounts_cashAndBank()}
					/>
				</div>

				<div class="form-group">
					<label class="label" for="type">{m.accounts_accountType()}</label>
					<select class="input" id="type" bind:value={newType}>
						<option value="ASSET">{m.accounts_asset()}</option>
						<option value="LIABILITY">{m.accounts_liability()}</option>
						<option value="EQUITY">{m.accounts_equity()}</option>
						<option value="REVENUE">{m.accounts_revenue()}</option>
						<option value="EXPENSE">{m.accounts_expense()}</option>
					</select>
				</div>

				<div class="form-group">
					<label class="label" for="description">{m.common_description()}</label>
					<input
						class="input"
						type="text"
						id="description"
						bind:value={newDescription}
						placeholder={m.accounts_optionalDescription()}
					/>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={() => (showCreateAccount = false)}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary">{m.common_create()}</button>
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

	.balance-col {
		text-align: right;
	}

	.balance {
		font-family: var(--font-mono);
		text-align: right;
		white-space: nowrap;
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

	/* Mobile responsive */
	@media (max-width: 768px) {
		h1 {
			font-size: 1.25rem;
		}

		.account-section {
			padding: 1rem;
		}

		.account-section h2 {
			font-size: 1rem;
		}

		.modal-backdrop {
			padding: 0;
			align-items: flex-end;
		}

		.modal {
			max-width: 100%;
			max-height: 95vh;
			border-radius: 1rem 1rem 0 0;
			margin: 0;
			overflow-y: auto;
		}

		.modal h2 {
			font-size: 1.25rem;
		}

		.modal-actions {
			flex-direction: column-reverse;
		}

		.modal-actions button {
			width: 100%;
			min-height: 44px;
		}
	}
</style>
