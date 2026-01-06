<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { api, type CashFlowStatement, type CashFlowItem } from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';
	import { getLocale } from '$lib/paraglide/runtime.js';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let isLoading = $state(false);
	let error = $state('');

	let startDate = $state(new Date(new Date().getFullYear(), 0, 1).toISOString().split('T')[0]);
	let endDate = $state(new Date().toISOString().split('T')[0]);

	let report = $state<CashFlowStatement | null>(null);

	onMount(() => {
		if (tenantId) {
			loadReport();
		}
	});

	async function loadReport() {
		if (!tenantId) return;

		isLoading = true;
		error = '';
		try {
			report = await api.getCashFlowStatement(tenantId, startDate, endDate);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load report';
		} finally {
			isLoading = false;
		}
	}

	function formatAmount(amount: string | undefined): string {
		if (!amount) return '0.00';
		const dec = new Decimal(amount);
		if (dec.isZero()) return '-';
		return dec.toFixed(2);
	}

	function formatSignedAmount(amount: string | undefined): string {
		if (!amount) return '0.00';
		const dec = new Decimal(amount);
		if (dec.isZero()) return '-';
		const sign = dec.isNegative() ? '' : '+';
		return sign + dec.toFixed(2);
	}

	function getDescription(item: CashFlowItem): string {
		const lang = getLocale();
		return lang === 'et' && item.description_et ? item.description_et : item.description;
	}

	function printReport() {
		window.print();
	}
</script>

<svelte:head>
	<title>{m.cashflow_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>{m.cashflow_title()}</h1>
		<div class="header-actions">
			<a href="/reports?tenant={tenantId}" class="btn btn-secondary">{m.common_back()}</a>
		</div>
	</div>

	{#if !tenantId}
		<div class="card empty-state">
			<p>Select a tenant from <a href="/dashboard">{m.nav_dashboard()}</a>.</p>
		</div>
	{:else}
		<div class="card controls-section">
			<div class="controls-form">
				<div class="form-group">
					<label class="label" for="startDate">{m.common_from()}</label>
					<input type="date" class="input" id="startDate" bind:value={startDate} />
				</div>
				<div class="form-group">
					<label class="label" for="endDate">{m.common_to()}</label>
					<input type="date" class="input" id="endDate" bind:value={endDate} />
				</div>
				<button class="btn btn-primary" onclick={loadReport} disabled={isLoading}>
					{isLoading ? m.common_loading() : m.cashflow_generate()}
				</button>
				{#if report}
					<button class="btn btn-secondary" onclick={printReport}>{m.common_print()}</button>
				{/if}
			</div>
		</div>

		{#if error}
			<div class="alert alert-error">{error}</div>
		{/if}

		{#if report}
			<div class="card report-container">
				<div class="report-header">
					<h2>{m.cashflow_title()}</h2>
					<p class="period">{report.start_date} — {report.end_date}</p>
				</div>

				<!-- Operating Activities -->
				<section class="cashflow-section">
					<h3>{m.cashflow_operating()}</h3>
					<table class="table">
						<tbody>
							{#each report.operating_activities as item}
								<tr class:subtotal={item.is_subtotal}>
									<td class="description">{getDescription(item)}</td>
									<td class="amount">{formatAmount(item.amount)}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</section>

				<!-- Investing Activities -->
				<section class="cashflow-section">
					<h3>{m.cashflow_investing()}</h3>
					<table class="table">
						<tbody>
							{#if report.investing_activities.length === 0}
								<tr>
									<td class="description empty">-</td>
									<td class="amount">-</td>
								</tr>
							{:else}
								{#each report.investing_activities as item}
									<tr class:subtotal={item.is_subtotal}>
										<td class="description">{getDescription(item)}</td>
										<td class="amount">{formatAmount(item.amount)}</td>
									</tr>
								{/each}
							{/if}
						</tbody>
					</table>
				</section>

				<!-- Financing Activities -->
				<section class="cashflow-section">
					<h3>{m.cashflow_financing()}</h3>
					<table class="table">
						<tbody>
							{#if report.financing_activities.length === 0}
								<tr>
									<td class="description empty">-</td>
									<td class="amount">-</td>
								</tr>
							{:else}
								{#each report.financing_activities as item}
									<tr class:subtotal={item.is_subtotal}>
										<td class="description">{getDescription(item)}</td>
										<td class="amount">{formatAmount(item.amount)}</td>
									</tr>
								{/each}
							{/if}
						</tbody>
					</table>
				</section>

				<!-- Summary -->
				<section class="cashflow-summary">
					<table class="table summary-table">
						<tbody>
							<tr class="summary-row">
								<td class="description">{m.cashflow_net_change()}</td>
								<td class="amount highlight">{formatSignedAmount(report.net_cash_change)}</td>
							</tr>
							<tr>
								<td class="description">{m.cashflow_opening()}</td>
								<td class="amount">{formatAmount(report.opening_cash)}</td>
							</tr>
							<tr class="total-row">
								<td class="description">{m.cashflow_closing()}</td>
								<td class="amount highlight">{formatAmount(report.closing_cash)}</td>
							</tr>
						</tbody>
					</table>
				</section>
			</div>
		{/if}
	{/if}
</div>

<style>
	.header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1.5rem;
	}

	h1 {
		font-size: 1.75rem;
	}

	.header-actions {
		display: flex;
		gap: 0.5rem;
	}

	.controls-section {
		margin-bottom: 1.5rem;
	}

	.controls-form {
		display: flex;
		gap: 1rem;
		align-items: flex-end;
		flex-wrap: wrap;
	}

	.controls-form .form-group {
		flex: 1;
		min-width: 150px;
		max-width: 200px;
	}

	.report-container {
		max-width: 800px;
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

	.report-header .period {
		color: var(--color-text-muted);
		font-size: 0.9rem;
	}

	.cashflow-section {
		margin-bottom: 1.5rem;
	}

	.cashflow-section h3 {
		font-size: 1rem;
		font-weight: 600;
		color: var(--color-primary);
		margin-bottom: 0.75rem;
		padding-bottom: 0.5rem;
		border-bottom: 1px solid var(--color-border);
	}

	.table {
		width: 100%;
		border-collapse: collapse;
	}

	.table td {
		padding: 0.5rem 0.75rem;
		border-bottom: 1px solid var(--color-border);
	}

	.table .description {
		width: 70%;
	}

	.table .description.empty {
		color: var(--color-text-muted);
		font-style: italic;
	}

	.table .amount {
		width: 30%;
		text-align: right;
		font-family: var(--font-mono);
	}

	.table tr.subtotal {
		font-weight: 600;
		background: var(--color-bg);
	}

	.table tr.subtotal td {
		border-top: 2px solid var(--color-border);
	}

	.cashflow-summary {
		margin-top: 2rem;
		padding-top: 1rem;
		border-top: 2px solid var(--color-border);
	}

	.summary-table .description {
		font-weight: 600;
	}

	.summary-table .summary-row {
		background: var(--color-bg);
	}

	.summary-table .total-row {
		background: var(--color-primary);
		color: white;
	}

	.summary-table .total-row td {
		border-bottom: none;
	}

	.summary-table .amount.highlight {
		font-weight: 700;
		font-size: 1.1em;
	}

	.empty-state {
		text-align: center;
		padding: 2rem;
		color: var(--color-text-muted);
	}

	@media print {
		.header-actions,
		.controls-section {
			display: none;
		}

		.report-container {
			max-width: none;
			box-shadow: none;
			border: none;
		}
	}

	@media (max-width: 768px) {
		.header {
			flex-direction: column;
			align-items: flex-start;
			gap: 1rem;
		}

		.controls-form {
			flex-direction: column;
		}

		.controls-form .form-group {
			max-width: none;
			width: 100%;
		}
	}
</style>
