<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { api, type TrialBalance, type AccountBalance, type BalanceSheet, type IncomeStatement } from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';
	import ExportButton from '$lib/components/ExportButton.svelte';

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
			error = err instanceof Error ? err.message : m.reports_failedToLoad();
		} finally {
			isLoading = false;
		}
	}

	function toDecimal(value: Decimal | number | string | undefined): Decimal {
		if (value === undefined || value === null) return new Decimal(0);
		if (value instanceof Decimal) return value;
		return new Decimal(value);
	}

	function formatAmount(amount: Decimal | number | undefined): string {
		const dec = toDecimal(amount);
		if (dec.isZero()) return '-';
		return dec.toFixed(2);
	}

	function formatBalance(balance: AccountBalance): { debit: string; credit: string } {
		const net = toDecimal(balance.net_balance);
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
		switch (type) {
			case 'ASSET':
				return m.accounts_assets();
			case 'LIABILITY':
				return m.accounts_liabilities();
			case 'EQUITY':
				return m.accounts_equities();
			case 'REVENUE':
				return m.accounts_revenues();
			case 'EXPENSE':
				return m.accounts_expenses();
			default:
				return type;
		}
	}

	function printReport() {
		window.print();
	}

	// Export data preparation functions
	function getTrialBalanceExportData(): { data: Record<string, unknown>[][]; headers: string[][] } {
		if (!trialBalance) return { data: [[]], headers: [[]] };

		const rows: Record<string, unknown>[] = [];
		const groupedAccounts = groupByType(trialBalance.accounts);

		for (const [type, accounts] of Object.entries(groupedAccounts)) {
			if (accounts.length > 0) {
				// Add section header
				rows.push({
					code: getTypeLabel(type),
					name: '',
					debit: '',
					credit: ''
				});

				for (const account of accounts) {
					const { debit, credit } = formatBalance(account);
					rows.push({
						code: account.account_code,
						name: account.account_name,
						debit: debit === '-' ? '' : debit,
						credit: credit === '-' ? '' : credit
					});
				}
			}
		}

		// Add totals
		rows.push({
			code: m.reports_totals(),
			name: '',
			debit: formatAmount(trialBalance.total_debits),
			credit: formatAmount(trialBalance.total_credits)
		});

		return {
			data: [rows],
			headers: [[m.reports_accountCode(), m.reports_accountName(), m.accounts_debit(), m.accounts_credit()]]
		};
	}

	function getBalanceSheetExportData(): { data: Record<string, unknown>[][]; headers: string[][] } {
		if (!balanceSheet) return { data: [[]], headers: [[]] };

		const rows: Record<string, unknown>[] = [];

		// Assets
		rows.push({ account: m.accounts_assets(), balance: '' });
		for (const account of balanceSheet.assets) {
			rows.push({
				account: `  ${account.account_code} - ${account.account_name}`,
				balance: formatAmount(account.net_balance)
			});
		}
		rows.push({ account: m.reports_totalAssets(), balance: formatAmount(balanceSheet.total_assets) });
		rows.push({ account: '', balance: '' });

		// Liabilities
		rows.push({ account: m.accounts_liabilities(), balance: '' });
		for (const account of balanceSheet.liabilities) {
			rows.push({
				account: `  ${account.account_code} - ${account.account_name}`,
				balance: formatAmount(toDecimal(account.net_balance).abs())
			});
		}
		rows.push({ account: m.reports_totalLiabilities(), balance: formatAmount(balanceSheet.total_liabilities) });
		rows.push({ account: '', balance: '' });

		// Equity
		rows.push({ account: m.accounts_equities(), balance: '' });
		for (const account of balanceSheet.equity) {
			rows.push({
				account: `  ${account.account_code} - ${account.account_name}`,
				balance: formatAmount(toDecimal(account.net_balance).abs())
			});
		}
		if (!toDecimal(balanceSheet.retained_earnings).isZero()) {
			rows.push({
				account: `  ${m.reports_retainedEarnings()}`,
				balance: formatAmount(balanceSheet.retained_earnings)
			});
		}
		rows.push({ account: m.reports_totalEquity(), balance: formatAmount(balanceSheet.total_equity) });

		return {
			data: [rows],
			headers: [[m.reports_account(), m.reports_balance()]]
		};
	}

	function getIncomeStatementExportData(): { data: Record<string, unknown>[][]; headers: string[][] } {
		if (!incomeStatement) return { data: [[]], headers: [[]] };

		const rows: Record<string, unknown>[] = [];

		// Revenue
		rows.push({ account: m.accounts_revenues(), amount: '' });
		for (const account of incomeStatement.revenue) {
			rows.push({
				account: `  ${account.account_code} - ${account.account_name}`,
				amount: formatAmount(toDecimal(account.net_balance).abs())
			});
		}
		rows.push({ account: m.reports_totalRevenue(), amount: formatAmount(incomeStatement.total_revenue) });
		rows.push({ account: '', amount: '' });

		// Expenses
		rows.push({ account: m.accounts_expenses(), amount: '' });
		for (const account of incomeStatement.expenses) {
			rows.push({
				account: `  ${account.account_code} - ${account.account_name}`,
				amount: formatAmount(account.net_balance)
			});
		}
		rows.push({ account: m.reports_totalExpenses(), amount: formatAmount(incomeStatement.total_expenses) });
		rows.push({ account: '', amount: '' });

		// Net Income
		rows.push({ account: m.reports_netIncome(), amount: formatAmount(incomeStatement.net_income) });

		return {
			data: [rows],
			headers: [[m.reports_account(), m.reports_amount()]]
		};
	}

	function getExportFilename(): string {
		const date = new Date().toISOString().split('T')[0];
		if (selectedReport === 'trial-balance') {
			return `trial-balance-${asOfDate}`;
		} else if (selectedReport === 'balance-sheet') {
			return `balance-sheet-${asOfDate}`;
		} else {
			return `income-statement-${startDate}-to-${endDate}`;
		}
	}

	let exportData = $derived.by(() => {
		if (selectedReport === 'trial-balance' && trialBalance) {
			return getTrialBalanceExportData();
		} else if (selectedReport === 'balance-sheet' && balanceSheet) {
			return getBalanceSheetExportData();
		} else if (selectedReport === 'income-statement' && incomeStatement) {
			return getIncomeStatementExportData();
		}
		return { data: [[]], headers: [[]] };
	});
</script>

<svelte:head>
	<title>{m.reports_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>{m.reports_financialReports()}</h1>
		{#if trialBalance || balanceSheet || incomeStatement}
			<div class="header-actions">
				<ExportButton
					data={exportData.data}
					headers={exportData.headers}
					filename={getExportFilename()}
				/>
			</div>
		{/if}
	</div>

	{#if !tenantId}
		<div class="card empty-state">
			<p>{m.reports_selectTenantDashboard()} <a href="/dashboard">{m.nav_dashboard()}</a>.</p>
		</div>
	{:else}
		<div class="report-controls card">
			<div class="control-row">
				<div class="form-group">
					<label class="label" for="reportType">{m.reports_reportType()}</label>
					<select class="input" id="reportType" bind:value={selectedReport}>
						<option value="trial-balance">{m.reports_trialBalance()}</option>
						<option value="balance-sheet">{m.reports_balanceSheet()}</option>
						<option value="income-statement">{m.reports_incomeStatement()}</option>
					</select>
				</div>

				{#if selectedReport === 'income-statement'}
					<div class="form-group">
						<label class="label" for="startDate">{m.reports_startDate()}</label>
						<input class="input" type="date" id="startDate" bind:value={startDate} />
					</div>
					<div class="form-group">
						<label class="label" for="endDate">{m.reports_endDate()}</label>
						<input class="input" type="date" id="endDate" bind:value={endDate} />
					</div>
				{:else}
					<div class="form-group">
						<label class="label" for="asOfDate">{m.reports_asOfDate()}</label>
						<input class="input" type="date" id="asOfDate" bind:value={asOfDate} />
					</div>
				{/if}

				<button class="btn btn-primary" onclick={loadReport} disabled={isLoading}>
					{isLoading ? m.reports_generating() : m.reports_generateReport()}
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
					<h2>{m.reports_trialBalance()}</h2>
					<p class="report-date">{m.reports_asOf()} {trialBalance.as_of_date}</p>
				</div>

				{#if trialBalance.accounts.length === 0}
					<p class="empty-message">{m.reports_noBalancesForDate()}</p>
				{:else}
					{@const groupedAccounts = groupByType(trialBalance.accounts)}

					<table class="report-table">
						<thead>
							<tr>
								<th>{m.reports_accountCode()}</th>
								<th>{m.reports_accountName()}</th>
								<th class="amount">{m.accounts_debit()}</th>
								<th class="amount">{m.accounts_credit()}</th>
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
								<td colspan="2"><strong>{m.reports_totals()}</strong></td>
								<td class="amount"><strong>{formatAmount(trialBalance.total_debits)}</strong></td>
								<td class="amount"><strong>{formatAmount(trialBalance.total_credits)}</strong></td>
							</tr>
							<tr class="balance-status">
								<td colspan="4" class:balanced={trialBalance.is_balanced} class:unbalanced={!trialBalance.is_balanced}>
									{#if trialBalance.is_balanced}
										{m.reports_trialBalanceBalanced()}
									{:else}
										{m.reports_trialBalanceNotBalanced()}
										{m.reports_difference()} {toDecimal(trialBalance.total_debits).minus(toDecimal(trialBalance.total_credits)).toFixed(2)}
									{/if}
								</td>
							</tr>
						</tfoot>
					</table>
				{/if}

				<p class="report-footer">
					{m.reports_generatedOn()} {new Date(trialBalance.generated_at).toLocaleString()}
				</p>
			</div>
		{/if}

		<!-- Balance Sheet Report -->
		{#if balanceSheet}
			<div class="report card">
				<div class="report-header">
					<h2>{m.reports_balanceSheet()}</h2>
					<p class="report-date">{m.reports_asOf()} {balanceSheet.as_of_date}</p>
				</div>

				<div class="balance-sheet-layout">
					<div class="bs-section">
						<h3>{m.accounts_assets()}</h3>
						{#if balanceSheet.assets.length === 0}
							<p class="empty-message">{m.reports_noAssetBalances()}</p>
						{:else}
							<table class="report-table">
								<thead>
									<tr>
										<th>{m.reports_account()}</th>
										<th class="amount">{m.reports_balance()}</th>
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
										<td><strong>{m.reports_totalAssets()}</strong></td>
										<td class="amount"><strong>{formatAmount(balanceSheet.total_assets)}</strong></td>
									</tr>
								</tfoot>
							</table>
						{/if}
					</div>

					<div class="bs-section">
						<h3>{m.accounts_liabilities()}</h3>
						{#if balanceSheet.liabilities.length === 0}
							<p class="empty-message">{m.reports_noLiabilityBalances()}</p>
						{:else}
							<table class="report-table">
								<thead>
									<tr>
										<th>{m.reports_account()}</th>
										<th class="amount">{m.reports_balance()}</th>
									</tr>
								</thead>
								<tbody>
									{#each balanceSheet.liabilities as account}
										<tr>
											<td>{account.account_code} - {account.account_name}</td>
											<td class="amount">{formatAmount(toDecimal(account.net_balance).abs())}</td>
										</tr>
									{/each}
								</tbody>
								<tfoot>
									<tr class="totals">
										<td><strong>{m.reports_totalLiabilities()}</strong></td>
										<td class="amount"><strong>{formatAmount(balanceSheet.total_liabilities)}</strong></td>
									</tr>
								</tfoot>
							</table>
						{/if}

						<h3 class="mt-2">{m.accounts_equities()}</h3>
						{#if balanceSheet.equity.length === 0 && toDecimal(balanceSheet.retained_earnings).isZero()}
							<p class="empty-message">{m.reports_noEquityBalances()}</p>
						{:else}
							<table class="report-table">
								<thead>
									<tr>
										<th>{m.reports_account()}</th>
										<th class="amount">{m.reports_balance()}</th>
									</tr>
								</thead>
								<tbody>
									{#each balanceSheet.equity as account}
										<tr>
											<td>{account.account_code} - {account.account_name}</td>
											<td class="amount">{formatAmount(toDecimal(account.net_balance).abs())}</td>
										</tr>
									{/each}
									{#if !toDecimal(balanceSheet.retained_earnings).isZero()}
										<tr>
											<td><em>{m.reports_retainedEarnings()}</em></td>
											<td class="amount">{formatAmount(balanceSheet.retained_earnings)}</td>
										</tr>
									{/if}
								</tbody>
								<tfoot>
									<tr class="totals">
										<td><strong>{m.reports_totalEquity()}</strong></td>
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
								<td><strong>{m.reports_totalAssets()}</strong></td>
								<td class="amount"><strong>{formatAmount(balanceSheet.total_assets)}</strong></td>
							</tr>
							<tr>
								<td><strong>{m.reports_totalLiabilitiesEquity()}</strong></td>
								<td class="amount"><strong>{formatAmount(toDecimal(balanceSheet.total_liabilities).plus(toDecimal(balanceSheet.total_equity)))}</strong></td>
							</tr>
							<tr class="balance-status">
								<td colspan="2" class:balanced={balanceSheet.is_balanced} class:unbalanced={!balanceSheet.is_balanced}>
									{#if balanceSheet.is_balanced}
										{m.reports_bsBalanced()}
									{:else}
										{m.reports_bsNotBalanced()}
									{/if}
								</td>
							</tr>
						</tbody>
					</table>
				</div>

				<p class="report-footer">
					{m.reports_generatedOn()} {new Date(balanceSheet.generated_at).toLocaleString()}
				</p>
			</div>
		{/if}

		<!-- Income Statement Report -->
		{#if incomeStatement}
			<div class="report card">
				<div class="report-header">
					<h2>{m.reports_incomeStatement()}</h2>
					<p class="report-date">{incomeStatement.start_date} - {incomeStatement.end_date}</p>
				</div>

				<div class="is-section">
					<h3>{m.accounts_revenues()}</h3>
					{#if incomeStatement.revenue.length === 0}
						<p class="empty-message">{m.reports_noRevenue()}</p>
					{:else}
						<table class="report-table">
							<thead>
								<tr>
									<th>{m.reports_account()}</th>
									<th class="amount">{m.reports_amount()}</th>
								</tr>
							</thead>
							<tbody>
								{#each incomeStatement.revenue as account}
									<tr>
										<td>{account.account_code} - {account.account_name}</td>
										<td class="amount">{formatAmount(toDecimal(account.net_balance).abs())}</td>
									</tr>
								{/each}
							</tbody>
							<tfoot>
								<tr class="totals">
									<td><strong>{m.reports_totalRevenue()}</strong></td>
									<td class="amount"><strong>{formatAmount(incomeStatement.total_revenue)}</strong></td>
								</tr>
							</tfoot>
						</table>
					{/if}
				</div>

				<div class="is-section">
					<h3>{m.accounts_expenses()}</h3>
					{#if incomeStatement.expenses.length === 0}
						<p class="empty-message">{m.reports_noExpenses()}</p>
					{:else}
						<table class="report-table">
							<thead>
								<tr>
									<th>{m.reports_account()}</th>
									<th class="amount">{m.reports_amount()}</th>
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
									<td><strong>{m.reports_totalExpenses()}</strong></td>
									<td class="amount"><strong>{formatAmount(incomeStatement.total_expenses)}</strong></td>
								</tr>
							</tfoot>
						</table>
					{/if}
				</div>

				<div class="is-summary">
					<table class="report-table summary-table">
						<tbody>
							<tr class="net-income" class:profit={toDecimal(incomeStatement.net_income).greaterThanOrEqualTo(0)} class:loss={toDecimal(incomeStatement.net_income).lessThan(0)}>
								<td><strong>{m.reports_netIncome()}</strong></td>
								<td class="amount"><strong>{formatAmount(incomeStatement.net_income)}</strong></td>
							</tr>
						</tbody>
					</table>
				</div>

				<p class="report-footer">
					{m.reports_generatedOn()} {new Date(incomeStatement.generated_at).toLocaleString()}
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

	.header-actions {
		display: flex;
		gap: 0.5rem;
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
		h1 {
			font-size: 1.25rem;
		}

		.header {
			flex-direction: column;
			align-items: flex-start;
			gap: 1rem;
		}

		.header .btn {
			width: 100%;
			min-height: 44px;
			justify-content: center;
		}

		.control-row {
			flex-direction: column;
			gap: 0.75rem;
		}

		.control-row .form-group {
			max-width: none;
			width: 100%;
		}

		.control-row .form-group .input {
			min-height: 44px;
		}

		.control-row .btn {
			width: 100%;
			min-height: 44px;
			justify-content: center;
		}

		.report {
			padding: 1rem;
		}

		.report-header h2 {
			font-size: 1.25rem;
		}

		.report-table th,
		.report-table td {
			padding: 0.5rem;
			font-size: 0.875rem;
		}

		.report-table .amount {
			width: auto;
		}

		.account-code {
			width: auto;
		}

		.balance-sheet-layout {
			grid-template-columns: 1fr;
		}

		.empty-state {
			padding: 2rem 1rem;
		}

		.summary-table {
			max-width: 100%;
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
