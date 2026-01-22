<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { api, type KMDDeclaration } from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';
	import StatusBadge, { type StatusConfig } from '$lib/components/StatusBadge.svelte';

	type VATStatus = 'DRAFT' | 'SUBMITTED' | 'ACCEPTED';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let isLoading = $state(false);
	let isGenerating = $state(false);
	let error = $state('');

	let declarations = $state<KMDDeclaration[]>([]);
	let selectedDeclaration = $state<KMDDeclaration | null>(null);

	let selectedYear = $state(new Date().getFullYear());
	let selectedMonth = $state(new Date().getMonth() + 1);

	const months = [
		{ value: 1, label: 'January / Jaanuar' },
		{ value: 2, label: 'February / Veebruar' },
		{ value: 3, label: 'March / Märts' },
		{ value: 4, label: 'April / Aprill' },
		{ value: 5, label: 'May / Mai' },
		{ value: 6, label: 'June / Juuni' },
		{ value: 7, label: 'July / Juuli' },
		{ value: 8, label: 'August / August' },
		{ value: 9, label: 'September / September' },
		{ value: 10, label: 'October / Oktoober' },
		{ value: 11, label: 'November / November' },
		{ value: 12, label: 'December / Detsember' }
	];

	onMount(() => {
		if (tenantId) {
			loadDeclarations();
		}
	});

	async function loadDeclarations() {
		isLoading = true;
		error = '';
		try {
			declarations = await api.listKMD(tenantId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load declarations';
		} finally {
			isLoading = false;
		}
	}

	async function generateDeclaration() {
		isGenerating = true;
		error = '';
		try {
			const newDecl = await api.generateKMD(tenantId, {
				year: selectedYear,
				month: selectedMonth
			});
			declarations = [newDecl, ...declarations];
			selectedDeclaration = newDecl;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to generate declaration';
		} finally {
			isGenerating = false;
		}
	}

	function formatAmount(amount: Decimal | string | number | undefined): string {
		if (!amount) return '0.00';
		if (amount instanceof Decimal) return amount.toFixed(2);
		return new Decimal(amount).toFixed(2);
	}

	const statusConfig: Record<VATStatus, StatusConfig> = {
		DRAFT: { class: 'badge-draft', label: m.vat_status_draft() },
		SUBMITTED: { class: 'badge-submitted', label: m.vat_status_submitted() },
		ACCEPTED: { class: 'badge-accepted', label: m.vat_status_accepted() }
	};

	function calculatePayable(decl: KMDDeclaration): Decimal {
		const output = decl.total_output_vat instanceof Decimal
			? decl.total_output_vat
			: new Decimal(decl.total_output_vat || 0);
		const input = decl.total_input_vat instanceof Decimal
			? decl.total_input_vat
			: new Decimal(decl.total_input_vat || 0);
		return output.minus(input);
	}

	async function exportXML() {
		if (!selectedDeclaration) return;
		try {
			await api.downloadKMDXml(tenantId, selectedDeclaration.year, selectedDeclaration.month);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to export XML';
		}
	}
</script>

<svelte:head>
	<title>{m.vat_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>{m.vat_title()}</h1>
	</div>

	{#if !tenantId}
		<div class="card empty-state">
			<p>Select a tenant from <a href="/dashboard">{m.nav_dashboard()}</a>.</p>
		</div>
	{:else}
		<div class="card generate-section">
			<h2>{m.vat_generate()}</h2>
			<div class="generate-form">
				<div class="form-group">
					<label class="label" for="year">{m.vat_year()}</label>
					<select class="input" id="year" bind:value={selectedYear}>
						{#each [2024, 2025, 2026] as year}
							<option value={year}>{year}</option>
						{/each}
					</select>
				</div>
				<div class="form-group">
					<label class="label" for="month">{m.vat_month()}</label>
					<select class="input" id="month" bind:value={selectedMonth}>
						{#each months as month}
							<option value={month.value}>{month.label}</option>
						{/each}
					</select>
				</div>
				<button class="btn btn-primary" onclick={generateDeclaration} disabled={isGenerating}>
					{isGenerating ? m.vat_generating() : m.vat_generate()}
				</button>
			</div>
		</div>

		{#if error}
			<div class="alert alert-error">{error}</div>
		{/if}

		<div class="declarations-grid">
			<div class="card declarations-list">
				<h2>{m.vat_declarations()}</h2>
				{#if isLoading}
					<p>{m.common_loading()}</p>
				{:else if declarations.length === 0}
					<p class="empty-message">{m.vat_no_declarations()}</p>
				{:else}
					<table class="table">
						<thead>
							<tr>
								<th>{m.vat_period()}</th>
								<th>{m.vat_status()}</th>
								<th class="amount">{m.vat_payable()}</th>
							</tr>
						</thead>
						<tbody>
							{#each declarations as decl}
								<tr
									class:selected={selectedDeclaration?.id === decl.id}
									onclick={() => (selectedDeclaration = decl)}
								>
									<td>{decl.year}-{String(decl.month).padStart(2, '0')}</td>
									<td>
										<StatusBadge status={decl.status} config={statusConfig} />
									</td
									>
									<td class="amount">{formatAmount(calculatePayable(decl))} EUR</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
			</div>

			{#if selectedDeclaration}
				<div class="card declaration-detail">
					<div class="detail-header">
						<h2>
							KMD {selectedDeclaration.year}-{String(selectedDeclaration.month).padStart(2, '0')}
						</h2>
						<div class="detail-actions">
							<button class="btn btn-secondary" onclick={exportXML}>{m.vat_export_xml()}</button>
						</div>
					</div>

					<div class="summary-cards">
						<div class="summary-card">
							<span class="summary-label">{m.vat_output_vat()}</span>
							<span class="summary-value"
								>{formatAmount(selectedDeclaration.total_output_vat)} EUR</span
							>
						</div>
						<div class="summary-card">
							<span class="summary-label">{m.vat_input_vat()}</span>
							<span class="summary-value"
								>{formatAmount(selectedDeclaration.total_input_vat)} EUR</span
							>
						</div>
						<div class="summary-card highlight">
							<span class="summary-label"
								>{calculatePayable(selectedDeclaration).greaterThanOrEqualTo(0)
									? m.vat_payable()
									: m.vat_refundable()}</span
							>
							<span class="summary-value"
								>{formatAmount(calculatePayable(selectedDeclaration).abs())} EUR</span
							>
						</div>
					</div>

					<h3>KMD Rows</h3>
					<table class="table">
						<thead>
							<tr>
								<th>{m.vat_row_code()}</th>
								<th>{m.vat_row_description()}</th>
								<th class="amount">{m.vat_row_tax_base()}</th>
								<th class="amount">{m.vat_row_tax_amount()}</th>
							</tr>
						</thead>
						<tbody>
							{#each selectedDeclaration.rows as row}
								<tr>
									<td>{row.code}</td>
									<td>{row.description}</td>
									<td class="amount">{formatAmount(row.tax_base)}</td>
									<td class="amount">{formatAmount(row.tax_amount)}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.header {
		margin-bottom: 1.5rem;
	}

	h1 {
		font-size: 1.75rem;
	}

	.generate-section {
		margin-bottom: 1.5rem;
	}

	.generate-section h2 {
		font-size: 1.1rem;
		margin-bottom: 1rem;
	}

	.generate-form {
		display: flex;
		gap: 1rem;
		align-items: flex-end;
		flex-wrap: wrap;
	}

	.generate-form .form-group {
		flex: 1;
		min-width: 150px;
		max-width: 200px;
	}

	.declarations-grid {
		display: grid;
		grid-template-columns: 1fr 2fr;
		gap: 1.5rem;
	}

	.declarations-list h2,
	.declaration-detail h2 {
		font-size: 1.1rem;
		margin-bottom: 1rem;
	}

	.declaration-detail h3 {
		font-size: 1rem;
		margin: 1.5rem 0 1rem;
	}

	.detail-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1rem;
	}

	.detail-actions {
		display: flex;
		gap: 0.5rem;
	}

	.summary-cards {
		display: flex;
		gap: 1rem;
		margin-bottom: 1.5rem;
	}

	.summary-card {
		flex: 1;
		padding: 1rem;
		background: var(--color-bg);
		border-radius: 8px;
		text-align: center;
	}

	.summary-card.highlight {
		background: var(--color-primary);
		color: white;
	}

	.summary-label {
		display: block;
		font-size: 0.75rem;
		text-transform: uppercase;
		margin-bottom: 0.5rem;
		opacity: 0.8;
	}

	.summary-value {
		display: block;
		font-size: 1.25rem;
		font-weight: 600;
	}

	.table {
		width: 100%;
		border-collapse: collapse;
	}

	.table th,
	.table td {
		padding: 0.75rem;
		text-align: left;
		border-bottom: 1px solid var(--color-border);
	}

	.table th {
		font-weight: 600;
		font-size: 0.75rem;
		text-transform: uppercase;
		color: var(--color-text-muted);
	}

	.table .amount {
		text-align: right;
		font-family: monospace;
	}

	.table tbody tr {
		cursor: pointer;
	}

	.table tbody tr:hover {
		background: var(--color-bg);
	}

	.table tbody tr.selected {
		background: var(--color-primary-light, #e0f2fe);
	}

	.empty-state,
	.empty-message {
		text-align: center;
		padding: 2rem;
		color: var(--color-text-muted);
	}

	@media (max-width: 768px) {
		.declarations-grid {
			grid-template-columns: 1fr;
		}

		.generate-form {
			flex-direction: column;
		}

		.generate-form .form-group {
			max-width: none;
			width: 100%;
		}

		.summary-cards {
			flex-direction: column;
		}
	}
</style>
