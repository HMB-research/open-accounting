<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { api, type TrialBalance, type AccountBalance, type BalanceSheet, type IncomeStatement } from '$lib/api';
	import Decimal from 'decimal.js';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let isLoading = $state(false);
	let error = $state('');

	let asOfDate = $state(new Date().toISOString().split('T')[0]);
	let startDate = $state(new Date(new Date().getFullYear(), 0, 1).toISOString().split('T')[0]);
	let endDate = $state(new Date().toISOString().split('T')[0]);

	let trialBalance = $state<TrialBalance | null>(null);
	let balanceSheet = $state<BalanceSheet | null>(null);
	let incomeStatement = $state<IncomeStatement | null>(null);

	let selectedReport = $state<'trial-balance' | 'balance-sheet' | 'income-statement'>(
		'trial-balance'
	);

	onMount(() => {
		if (tenantId) {
			loadReport();
		}
	});

	async function loadReport() {
		isLoading = true;
		error = '';

		try {
			if (selectedReport === 'trial-balance') {
				trialBalance = await api.getTrialBalance(tenantId, asOfDate);
				balanceSheet = null;
				incomeStatement = null;
			} else if (selectedReport === 'balance-sheet') {
				balanceSheet = await api.getBalanceSheet(tenantId, asOfDate);
				trialBalance = null;
				incomeStatement = null;
			} else if (selectedReport === 'income-statement') {
				incomeStatement = await api.getIncomeStatement(tenantId, startDate, endDate);
				trialBalance = null;
				balanceSheet = null;
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load report';
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
		{#if trialBalance || balanceSheet || incomeStatement}
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
						<option value="balance-sheet">Balance Sheet</option>
						<option value="income-statement">Income Statement</option>
					</select>
				</div>

				{#if selectedReport === 'income-statement'}
					<div class="form-group">
						<label class="label" for="startDate">Start Date</label>
						<input class="input" type="date" id="startDate" bind:value={startDate} />
					</div>
					<div class="form-group">
						<label class="label" for="endDate">End Date</label>
						<input class="input" type="date" id="endDate" bind:value={endDate} />
					</div>
				{:else}
					<div class="form-group">
						<label class="label" for="asOfDate">As of Date</label>
						<input class="input" type="date" id="asOfDate" bind:value={asOfDate} />
					</div>
				{/if}

				<button class="btn btn-primary" onclick={loadReport} disabled={isLoading}>
					{isLoading ? 'Loading...' : 'Generate Report'}
				</button>
			</div>
		</div>

		{#if error}
			<div class="alert alert-error">{error}</div>
		{/if}

		<!-- Trial Balance Report -->
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

		<!-- Balance Sheet Report -->
		{#if balanceSheet}
			<div class="report card">
				<div class="report-header">
					<h2>Balance Sheet</h2>
					<p class="report-date">As of {balanceSheet.as_of_date}</p>
				</div>

				<div class="balance-sheet-layout">
					<div class="bs-section">
						<h3>Assets</h3>
						{#if balanceSheet.assets.length === 0}
							<p class="empty-message">No asset accounts with balances.</p>
						{:else}
							<table class="report-table">
								<thead>
									<tr>
										<th>Account</th>
										<th class="amount">Balance</th>
									</tr>
								</thead>
								<tbody>
									{#each balanceSheet.assets as account}
										<tr>
											<td>{account.account_code} - {account.account_name}</td>
											<td class="amount">{formatAmount(account.net_balance)}</td>
										</tr>
									{/each}
								</tbody>
								<tfoot>
									<tr class="totals">
										<td><strong>Total Assets</strong></td>
										<td class="amount"><strong>{formatAmount(balanceSheet.total_assets)}</strong></td>
									</tr>
								</tfoot>
							</table>
						{/if}
					</div>

					<div class="bs-section">
						<h3>Liabilities</h3>
						{#if balanceSheet.liabilities.length === 0}
							<p class="empty-message">No liability accounts with balances.</p>
						{:else}
							<table class="report-table">
								<thead>
									<tr>
										<th>Account</th>
										<th class="amount">Balance</th>
									</tr>
								</thead>
								<tbody>
									{#each balanceSheet.liabilities as account}
										<tr>
											<td>{account.account_code} - {account.account_name}</td>
											<td class="amount">{formatAmount(account.net_balance.abs())}</td>
										</tr>
									{/each}
								</tbody>
								<tfoot>
									<tr class="totals">
										<td><strong>Total Liabilities</strong></td>
										<td class="amount"><strong>{formatAmount(balanceSheet.total_liabilities)}</strong></td>
									</tr>
								</tfoot>
							</table>
						{/if}

						<h3 class="mt-2">Equity</h3>
						{#if balanceSheet.equity.length === 0 && balanceSheet.retained_earnings.isZero()}
							<p class="empty-message">No equity accounts with balances.</p>
						{:else}
							<table class="report-table">
								<thead>
									<tr>
										<th>Account</th>
										<th class="amount">Balance</th>
									</tr>
								</thead>
								<tbody>
									{#each balanceSheet.equity as account}
										<tr>
											<td>{account.account_code} - {account.account_name}</td>
											<td class="amount">{formatAmount(account.net_balance.abs())}</td>
										</tr>
									{/each}
									{#if !balanceSheet.retained_earnings.isZero()}
										<tr>
											<td><em>Retained Earnings</em></td>
											<td class="amount">{formatAmount(balanceSheet.retained_earnings)}</td>
										</tr>
									{/if}
								</tbody>
								<tfoot>
									<tr class="totals">
										<td><strong>Total Equity</strong></td>
										<td class="amount"><strong>{formatAmount(balanceSheet.total_equity)}</strong></td>
									</tr>
								</tfoot>
							</table>
						{/if}
					</div>
				</div>

				<div class="bs-summary">
					<table class="report-table summary-table">
						<tbody>
							<tr>
								<td><strong>Total Assets</strong></td>
								<td class="amount"><strong>{formatAmount(balanceSheet.total_assets)}</strong></td>
							</tr>
							<tr>
								<td><strong>Total Liabilities + Equity</strong></td>
								<td class="amount"><strong>{formatAmount(balanceSheet.total_liabilities.plus(balanceSheet.total_equity))}</strong></td>
							</tr>
							<tr class="balance-status">
								<td colspan="2" class:balanced={balanceSheet.is_balanced} class:unbalanced={!balanceSheet.is_balanced}>
									{#if balanceSheet.is_balanced}
										Balance sheet is in balance (Assets = Liabilities + Equity)
									{:else}
										Balance sheet is NOT in balance!
									{/if}
								</td>
							</tr>
						</tbody>
					</table>
				</div>

				<p class="report-footer">
					Generated on {new Date(balanceSheet.generated_at).toLocaleString()}
				</p>
			</div>
		{/if}

		<!-- Income Statement Report -->
		{#if incomeStatement}
			<div class="report card">
				<div class="report-header">
					<h2>Income Statement</h2>
					<p class="report-date">{incomeStatement.start_date} to {incomeStatement.end_date}</p>
				</div>

				<div class="is-section">
					<h3>Revenue</h3>
					{#if incomeStatement.revenue.length === 0}
						<p class="empty-message">No revenue for this period.</p>
					{:else}
						<table class="report-table">
							<thead>
								<tr>
									<th>Account</th>
									<th class="amount">Amount</th>
								</tr>
							</thead>
							<tbody>
								{#each incomeStatement.revenue as account}
									<tr>
										<td>{account.account_code} - {account.account_name}</td>
										<td class="amount">{formatAmount(account.net_balance.abs())}</td>
									</tr>
								{/each}
							</tbody>
							<tfoot>
								<tr class="totals">
									<td><strong>Total Revenue</strong></td>
									<td class="amount"><strong>{formatAmount(incomeStatement.total_revenue)}</strong></td>
								</tr>
							</tfoot>
						</table>
					{/if}
				</div>

				<div class="is-section">
					<h3>Expenses</h3>
					{#if incomeStatement.expenses.length === 0}
						<p class="empty-message">No expenses for this period.</p>
					{:else}
						<table class="report-table">
							<thead>
								<tr>
									<th>Account</th>
									<th class="amount">Amount</th>
								</tr>
							</thead>
							<tbody>
								{#each incomeStatement.expenses as account}
									<tr>
										<td>{account.account_code} - {account.account_name}</td>
										<td class="amount">{formatAmount(account.net_balance)}</td>
									</tr>
								{/each}
							</tbody>
							<tfoot>
								<tr class="totals">
									<td><strong>Total Expenses</strong></td>
									<td class="amount"><strong>{formatAmount(incomeStatement.total_expenses)}</strong></td>
								</tr>
							</tfoot>
						</table>
					{/if}
				</div>

				<div class="is-summary">
					<table class="report-table summary-table">
						<tbody>
							<tr class="net-income" class:profit={incomeStatement.net_income.greaterThanOrEqualTo(0)} class:loss={incomeStatement.net_income.lessThan(0)}>
								<td><strong>Net Income</strong></td>
								<td class="amount"><strong>{formatAmount(incomeStatement.net_income)}</strong></td>
							</tr>
						</tbody>
					</table>
				</div>

				<p class="report-footer">
					Generated on {new Date(incomeStatement.generated_at).toLocaleString()}
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
		flex-wrap: wrap;
	}

	.control-row .form-group {
		flex: 1;
		max-width: 200px;
		min-width: 150px;
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

	/* Balance Sheet Layout */
	.balance-sheet-layout {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 2rem;
		margin-bottom: 2rem;
	}

	.bs-section h3,
	.is-section h3 {
		font-size: 1.1rem;
		margin-bottom: 1rem;
		padding-bottom: 0.5rem;
		border-bottom: 1px solid var(--color-border);
	}

	.mt-2 {
		margin-top: 2rem;
	}

	.bs-summary,
	.is-summary {
		margin-top: 2rem;
		padding-top: 1rem;
		border-top: 2px solid var(--color-border);
	}

	.summary-table {
		max-width: 400px;
		margin: 0 auto;
	}

	/* Income Statement */
	.is-section {
		margin-bottom: 2rem;
	}

	.net-income td {
		font-size: 1.1rem;
		padding: 1rem;
	}

	.profit {
		background: var(--color-success-bg, #d1fae5);
	}

	.profit td {
		color: var(--color-success, #10b981);
	}

	.loss {
		background: var(--color-error-bg, #fee2e2);
	}

	.loss td {
		color: var(--color-error, #ef4444);
	}

	@media (max-width: 768px) {
		.balance-sheet-layout {
			grid-template-columns: 1fr;
		}
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
