<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { api, type InterestSettings, type InterestCalculationResult } from '$lib/api';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let isLoading = $state(false);
	let isSaving = $state(false);
	let error = $state('');
	let successMessage = $state('');

	let settings = $state<InterestSettings | null>(null);
	let overdueInvoices = $state<InterestCalculationResult[]>([]);

	// Form state - daily rate as percentage (e.g., 0.05 for 0.05%)
	let formRatePercent = $state(0.05);

	onMount(() => {
		if (tenantId) {
			loadSettings();
			loadOverdueInvoices();
		}
	});

	async function loadSettings() {
		isLoading = true;
		error = '';
		try {
			settings = await api.getInterestSettings(tenantId);
			formRatePercent = settings.rate * 100; // Convert to percentage
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load settings';
		} finally {
			isLoading = false;
		}
	}

	async function loadOverdueInvoices() {
		try {
			overdueInvoices = await api.getOverdueInvoicesWithInterest(tenantId);
		} catch (err) {
			console.error('Failed to load overdue invoices:', err);
		}
	}

	async function saveSettings() {
		isSaving = true;
		error = '';
		successMessage = '';

		try {
			const rate = formRatePercent / 100; // Convert from percentage to decimal
			settings = await api.updateInterestSettings(tenantId, { rate });
			successMessage = 'Interest settings saved successfully';
			await loadOverdueInvoices(); // Refresh with new rate
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to save settings';
		} finally {
			isSaving = false;
		}
	}

	function formatCurrency(amount: string, currency: string): string {
		const num = parseFloat(amount);
		return new Intl.NumberFormat('en-US', { style: 'currency', currency }).format(num);
	}

	function formatDate(date: string): string {
		return new Date(date).toLocaleDateString();
	}

	let annualRateDisplay = $derived((formRatePercent * 365).toFixed(1));
</script>

<svelte:head>
	<title>Interest Settings - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>Late Payment Interest</h1>
	</div>

	{#if !tenantId}
		<div class="card empty-state">
			<p>Select a tenant from <a href="/dashboard">Dashboard</a>.</p>
		</div>
	{:else}
		{#if error}
			<div class="alert alert-error">{error}</div>
		{/if}

		{#if successMessage}
			<div class="alert alert-success">{successMessage}</div>
		{/if}

		<div class="card">
			<h2>Interest Rate Configuration</h2>
			<p class="description">
				Configure the daily interest rate applied to overdue invoices. Interest is calculated as:
				<code>outstanding amount × daily rate × days overdue</code>
			</p>

			{#if isLoading}
				<p>Loading...</p>
			{:else}
				<form onsubmit={(e) => { e.preventDefault(); saveSettings(); }}>
					<div class="form-group">
						<label for="rate">Daily Interest Rate (%)</label>
						<input
							type="number"
							id="rate"
							bind:value={formRatePercent}
							step="0.001"
							min="0"
							max="1"
							placeholder="0.05"
						/>
						<small>
							{formRatePercent > 0
								? `${formRatePercent}% daily = ${annualRateDisplay}% annually`
								: 'Set to 0 to disable interest calculation'}
						</small>
					</div>

					<div class="form-actions">
						<button type="submit" class="btn btn-primary" disabled={isSaving}>
							{isSaving ? 'Saving...' : 'Save Settings'}
						</button>
					</div>
				</form>

				{#if settings}
					<div class="current-settings">
						<h4>Current Settings</h4>
						<p><strong>Status:</strong> {settings.is_enabled ? 'Enabled' : 'Disabled'}</p>
						<p><strong>Rate:</strong> {settings.description}</p>
					</div>
				{/if}
			{/if}
		</div>

		{#if overdueInvoices.length > 0}
			<div class="card">
				<h2>Overdue Invoices with Interest</h2>
				<p class="description">
					Current interest calculations for all overdue invoices.
				</p>

				<table class="table">
					<thead>
						<tr>
							<th>Invoice</th>
							<th>Due Date</th>
							<th>Days Overdue</th>
							<th class="text-right">Outstanding</th>
							<th class="text-right">Interest</th>
							<th class="text-right">Total Due</th>
						</tr>
					</thead>
					<tbody>
						{#each overdueInvoices as invoice}
							<tr>
								<td>
									<a href="/invoices/{invoice.invoice_id}?tenant={tenantId}">
										{invoice.invoice_number}
									</a>
								</td>
								<td>{formatDate(invoice.due_date)}</td>
								<td>{invoice.days_overdue}</td>
								<td class="text-right">{formatCurrency(invoice.outstanding_amount, invoice.currency)}</td>
								<td class="text-right text-warning">{formatCurrency(invoice.total_interest, invoice.currency)}</td>
								<td class="text-right text-bold">{formatCurrency(invoice.total_with_interest, invoice.currency)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{:else if !isLoading}
			<div class="card">
				<h2>Overdue Invoices</h2>
				<p class="empty-state">No overdue invoices with outstanding balances.</p>
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

	h2 {
		margin-top: 0;
		margin-bottom: 0.5rem;
	}

	.description {
		color: var(--color-text-muted);
		margin-bottom: 1.5rem;
	}

	.description code {
		background: var(--color-bg);
		padding: 0.125rem 0.375rem;
		border-radius: 0.25rem;
		font-size: 0.875rem;
	}

	.form-group {
		margin-bottom: 1.5rem;
		max-width: 300px;
	}

	.form-group label {
		display: block;
		margin-bottom: 0.25rem;
		font-weight: 500;
	}

	.form-group input {
		width: 100%;
		padding: 0.5rem;
		border: 1px solid var(--color-border);
		border-radius: 0.25rem;
	}

	.form-group small {
		display: block;
		margin-top: 0.25rem;
		color: var(--color-text-muted);
		font-size: 0.75rem;
	}

	.form-actions {
		margin-top: 1.5rem;
	}

	.current-settings {
		margin-top: 1.5rem;
		padding-top: 1.5rem;
		border-top: 1px solid var(--color-border);
	}

	.current-settings h4 {
		margin: 0 0 0.5rem;
		font-size: 0.875rem;
		color: var(--color-text-muted);
	}

	.current-settings p {
		margin: 0.25rem 0;
	}

	.table {
		width: 100%;
		border-collapse: collapse;
	}

	.table th,
	.table td {
		padding: 0.75rem;
		border-bottom: 1px solid var(--color-border);
		text-align: left;
	}

	.table th {
		font-weight: 600;
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}

	.text-right {
		text-align: right;
	}

	.text-warning {
		color: var(--color-warning, #f59e0b);
	}

	.text-bold {
		font-weight: 600;
	}

	.empty-state {
		text-align: center;
		padding: 2rem;
		color: var(--color-text-muted);
	}

	.card + .card {
		margin-top: 1rem;
	}

	.alert-success {
		background: rgba(76, 175, 80, 0.1);
		color: #2e7d32;
		padding: 0.75rem 1rem;
		border-radius: 0.25rem;
		margin-bottom: 1rem;
	}

	.alert-error {
		background: rgba(244, 67, 54, 0.1);
		color: #c62828;
		padding: 0.75rem 1rem;
		border-radius: 0.25rem;
		margin-bottom: 1rem;
	}
</style>
