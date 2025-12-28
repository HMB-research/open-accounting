<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { api, type TrialBalance, type AccountBalance } from '$lib/api';
	import Decimal from 'decimal.js';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let isLoading = $state(false);
	let error = $state('');

	let asOfDate = $state(new Date().toISOString().split('T')[0]);
	let trialBalance = $state<TrialBalance | null>(null);

	let selectedReport = $state<'trial-balance' | 'balance-sheet' | 'income-statement'>(
		'trial-balance'
	);

	onMount(() => {
		if (tenantId) {
			loadTrialBalance();
		}
	});

	async function loadTrialBalance() {
		isLoading = true;
		error = '';

		try {
			trialBalance = await api.getTrialBalance(tenantId, asOfDate);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load trial balance';
		} finally {
			isLoading = false;
		}
	}

	function formatAmount(amount: Decimal | undefined): string {
		if (!amount || amount.isZero()) return '-';
		return amount.toFixed(2);
	}

	function formatBalance(balance: AccountBalance): { debit: string; credit: string } {
		const net = balance.net_balance;
		if (net.isZero()) {
			return { debit: '-', credit: '-' };
		}
		if (net.greaterThan(0)) {
			return { debit: net.toFixed(2), credit: '-' };
		}
		return { debit: '-', credit: net.abs().toFixed(2) };
	}

	function groupByType(accounts: AccountBalance[]): Record<string, AccountBalance[]> {
		const groups: Record<string, AccountBalance[]> = {
			ASSET: [],
			LIABILITY: [],
			EQUITY: [],
			REVENUE: [],
			EXPENSE: []
		};

		for (const account of accounts) {
			if (groups[account.account_type]) {
				groups[account.account_type].push(account);
			}
		}

		return groups;
	}

	function getTypeLabel(type: string): string {
		const labels: Record<string, string> = {
			ASSET: 'Assets',
			LIABILITY: 'Liabilities',
			EQUITY: 'Equity',
			REVENUE: 'Revenue',
			EXPENSE: 'Expenses'
		};
		return labels[type] || type;
	}

	function printReport() {
		window.print();
	}
</script>

<svelte:head>
	<title>Reports - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>Financial Reports</h1>
		{#if trialBalance}
			<button class="btn btn-secondary" onclick={printReport}>Print</button>
		{/if}
	</div>

	{#if !tenantId}
		<div class="card empty-state">
			<p>Please select a tenant from the <a href="/dashboard">dashboard</a>.</p>
		</div>
	{:else}
		<div class="report-controls card">
			<div class="control-row">
				<div class="form-group">
					<label class="label" for="reportType">Report Type</label>
					<select class="input" id="reportType" bind:value={selectedReport}>
						<option value="trial-balance">Trial Balance</option>
						<option value="balance-sheet" disabled>Balance Sheet (coming soon)</option>
						<option value="income-statement" disabled>Income Statement (coming soon)</option>
					</select>
				</div>

				<div class="form-group">
					<label class="label" for="asOfDate">As of Date</label>
					<input class="input" type="date" id="asOfDate" bind:value={asOfDate} />
				</div>

				<button class="btn btn-primary" onclick={loadTrialBalance} disabled={isLoading}>
					{isLoading ? 'Loading...' : 'Generate Report'}
				</button>
			</div>
		</div>

		{#if error}
			<div class="alert alert-error">{error}</div>
		{/if}

		{#if trialBalance}
			<div class="report card">
				<div class="report-header">
					<h2>Trial Balance</h2>
					<p class="report-date">As of {trialBalance.as_of_date}</p>
				</div>

				{#if trialBalance.accounts.length === 0}
					<p class="empty-message">No account balances found for this date.</p>
				{:else}
					{@const groupedAccounts = groupByType(trialBalance.accounts)}

					<table class="report-table">
						<thead>
							<tr>
								<th>Account Code</th>
								<th>Account Name</th>
								<th class="amount">Debit</th>
								<th class="amount">Credit</th>
							</tr>
						</thead>
						<tbody>
							{#each Object.entries(groupedAccounts) as [type, accounts]}
								{#if accounts.length > 0}
									<tr class="section-header">
										<td colspan="4">{getTypeLabel(type)}</td>
									</tr>
									{#each accounts as account}
										{@const { debit, credit } = formatBalance(account)}
										<tr>
											<td class="account-code">{account.account_code}</td>
											<td>{account.account_name}</td>
											<td class="amount">{debit}</td>
											<td class="amount">{credit}</td>
										</tr>
									{/each}
								{/if}
							{/each}
						</tbody>
						<tfoot>
							<tr class="totals">
								<td colspan="2"><strong>Totals</strong></td>
								<td class="amount"><strong>{formatAmount(trialBalance.total_debits)}</strong></td>
								<td class="amount"><strong>{formatAmount(trialBalance.total_credits)}</strong></td>
							</tr>
							<tr class="balance-status">
								<td colspan="4" class:balanced={trialBalance.is_balanced} class:unbalanced={!trialBalance.is_balanced}>
									{#if trialBalance.is_balanced}
										Trial balance is in balance
									{:else}
										Trial balance is NOT in balance!
										Difference: {trialBalance.total_debits.minus(trialBalance.total_credits).toFixed(2)}
									{/if}
								</td>
							</tr>
						</tfoot>
					</table>
				{/if}

				<p class="report-footer">
					Generated on {new Date(trialBalance.generated_at).toLocaleString()}
				</p>
			</div>
		{/if}
	{/if}
</div>

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

	.empty-state {
		text-align: center;
		padding: 3rem;
	}

	.report-controls {
		margin-bottom: 1.5rem;
	}

	.control-row {
		display: flex;
		gap: 1rem;
		align-items: flex-end;
	}

	.control-row .form-group {
		flex: 1;
		max-width: 250px;
	}

	.report {
		padding: 2rem;
	}

	.report-header {
		text-align: center;
		margin-bottom: 2rem;
		padding-bottom: 1rem;
		border-bottom: 2px solid var(--color-border);
	}

	.report-header h2 {
		font-size: 1.5rem;
		margin-bottom: 0.5rem;
	}

	.report-date {
		color: var(--color-text-muted);
	}

	.report-table {
		width: 100%;
		border-collapse: collapse;
	}

	.report-table th,
	.report-table td {
		padding: 0.75rem 1rem;
		text-align: left;
		border-bottom: 1px solid var(--color-border);
	}

	.report-table th {
		font-weight: 600;
		font-size: 0.75rem;
		text-transform: uppercase;
		color: var(--color-text-muted);
		background: var(--color-bg);
	}

	.report-table .amount {
		text-align: right;
		font-family: monospace;
		width: 120px;
	}

	.account-code {
		font-family: monospace;
		width: 100px;
	}

	.section-header td {
		font-weight: 600;
		background: var(--color-bg);
		padding-top: 1rem;
	}

	.report-table tfoot tr.totals {
		border-top: 2px solid var(--color-border);
	}

	.report-table tfoot tr.totals td {
		padding-top: 1rem;
	}

	.balance-status td {
		text-align: center;
		padding: 1rem;
		font-weight: 600;
	}

	.balanced {
		color: var(--color-success, #10b981);
		background: var(--color-success-bg, #d1fae5);
	}

	.unbalanced {
		color: var(--color-error, #ef4444);
		background: var(--color-error-bg, #fee2e2);
	}

	.report-footer {
		margin-top: 2rem;
		text-align: center;
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}

	.empty-message {
		text-align: center;
		color: var(--color-text-muted);
		padding: 2rem;
	}

	@media print {
		.header button,
		.report-controls {
			display: none;
		}

		.report {
			box-shadow: none;
			border: none;
		}
	}
</style>
