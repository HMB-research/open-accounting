<script lang="ts">
	import { page } from '$app/stores';
	import { api, type KMDDeclaration } from '$lib/api';
	import { goto } from '$app/navigation';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';
	import { formatCurrency } from '$lib/utils/formatting';
	import StatusBadge, { type StatusConfig } from '$lib/components/StatusBadge.svelte';

	type KMDStatus = 'DRAFT' | 'SUBMITTED' | 'ACCEPTED';

	const statusConfig: Record<KMDStatus, StatusConfig> = {
		DRAFT: { class: 'badge-draft', label: m.vat_status_draft() },
		SUBMITTED: { class: 'badge-submitted', label: m.vat_status_submitted() },
		ACCEPTED: { class: 'badge-accepted', label: m.vat_status_accepted() }
	};

	let tenantId = $state('');
	let loading = $state(true);
	let generating = $state(false);
	let error = $state<string | null>(null);
	let declarations = $state<KMDDeclaration[]>([]);

	let selectedYear = $state(new Date().getFullYear());
	let selectedMonth = $state(new Date().getMonth() + 1);

	$effect(() => {
		loadData();
	});

	async function loadData() {
		loading = true;
		error = null;
		try {
			// Check URL param for tenant first
			const urlTenantId = $page.url.searchParams.get('tenant');
			if (urlTenantId) {
				tenantId = urlTenantId;
			} else {
				const memberships = await api.getMyTenants();
				if (memberships.length === 0) {
					error = m.tax_noTenantAvailable();
					return;
				}
				tenantId = memberships[0].tenant.id;
			}
			declarations = await api.listKMD(tenantId);
		} catch (e) {
			error = e instanceof Error ? e.message : m.tax_failedToLoad();
		} finally {
			loading = false;
		}
	}

	async function generateKMD() {
		generating = true;
		error = null;
		try {
			const decl = await api.generateKMD(tenantId, {
				year: selectedYear,
				month: selectedMonth
			});
			// Reload to get the updated list
			declarations = await api.listKMD(tenantId);
		} catch (e) {
			error = e instanceof Error ? e.message : m.tax_failedToGenerate();
		} finally {
			generating = false;
		}
	}

	async function downloadXml(decl: KMDDeclaration) {
		try {
			await api.downloadKMDXml(tenantId, decl.year, decl.month);
		} catch (e) {
			error = e instanceof Error ? e.message : m.tax_failedToDownload();
		}
	}

	function getPayable(decl: KMDDeclaration): Decimal {
		const output =
			decl.total_output_vat instanceof Decimal
				? decl.total_output_vat
				: new Decimal(decl.total_output_vat || 0);
		const input =
			decl.total_input_vat instanceof Decimal
				? decl.total_input_vat
				: new Decimal(decl.total_input_vat || 0);
		return output.minus(input);
	}
</script>

<svelte:head>
	<title>VAT Declarations - Open Accounting</title>
</svelte:head>

<div class="page-container">
	<div class="page-header">
		<div class="header-left">
			<button onclick={() => goto('/dashboard')} class="back-btn">
				&larr; {m.common_back()}
			</button>
			<h1>{m.tax_vatDeclarations()}</h1>
		</div>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if loading}
		<div class="loading-spinner">
			<div class="spinner"></div>
		</div>
	{:else}
		<!-- Generate KMD Form -->
		<div class="card generate-form">
			<h2>{m.tax_generateVatDeclaration()}</h2>
			<div class="form-row">
				<div class="form-group">
					<label class="label" for="year">{m.tax_year()}</label>
					<select id="year" class="input" bind:value={selectedYear}>
						{#each [2024, 2025, 2026] as year}
							<option value={year}>{year}</option>
						{/each}
					</select>
				</div>
				<div class="form-group">
					<label class="label" for="month">{m.tax_month()}</label>
					<select id="month" class="input" bind:value={selectedMonth}>
						{#each Array.from({ length: 12 }, (_, i) => i + 1) as month}
							<option value={month}>{String(month).padStart(2, '0')}</option>
						{/each}
					</select>
				</div>
				<button onclick={generateKMD} disabled={generating} class="btn btn-primary">
					{generating ? m.tax_generating() : m.tax_generate()}
				</button>
			</div>
		</div>

		<!-- Declarations List -->
		{#if declarations.length > 0}
			<div class="card declarations-list">
				<div class="list-header">
					<h2>{m.tax_generatedDeclarations()}</h2>
				</div>
				<div class="declarations">
					{#each declarations as decl}
						<div class="declaration-item">
							<div class="declaration-info">
								<div class="declaration-period">{decl.year}-{String(decl.month).padStart(2, '0')}</div>
								<div class="declaration-amounts">
									{m.tax_outputVat()}: {formatCurrency(decl.total_output_vat)} | {m.tax_inputVat()}: {formatCurrency(decl.total_input_vat)} | {m.tax_payable()}: {formatCurrency(getPayable(decl))}
								</div>
							</div>
							<div class="declaration-actions">
								<StatusBadge status={decl.status} config={statusConfig} />
								<button onclick={() => downloadXml(decl)} class="btn btn-secondary btn-sm">
									{m.tax_downloadXml()}
								</button>
							</div>
						</div>
					{/each}
				</div>
			</div>
		{:else}
			<div class="card empty-state">
				<p>{m.tax_noDeclarations()}</p>
			</div>
		{/if}
	{/if}
</div>

<style>
	.page-container {
		max-width: 1200px;
		margin: 0 auto;
		padding: 2rem 1rem;
	}

	.page-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 1.5rem;
	}

	.header-left {
		display: flex;
		align-items: center;
		gap: 1rem;
	}

	.back-btn {
		color: var(--color-text-muted);
		background: none;
		border: none;
		cursor: pointer;
	}

	.back-btn:hover {
		color: var(--color-text);
	}

	h1 {
		font-size: 1.5rem;
		font-weight: 600;
	}

	h2 {
		font-size: 1.125rem;
		font-weight: 600;
		margin-bottom: 1rem;
	}

	.loading-spinner {
		display: flex;
		justify-content: center;
		padding: 3rem;
	}

	.spinner {
		width: 2rem;
		height: 2rem;
		border: 2px solid var(--color-border);
		border-top-color: var(--color-primary);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}

	.generate-form {
		margin-bottom: 1.5rem;
	}

	.form-row {
		display: flex;
		gap: 1rem;
		align-items: flex-end;
		flex-wrap: wrap;
	}

	.form-group {
		flex: 1;
		min-width: 120px;
	}

	.declarations-list {
		padding: 0;
	}

	.list-header {
		padding: 1rem 1.5rem;
		border-bottom: 1px solid var(--color-border);
	}

	.list-header h2 {
		margin-bottom: 0;
	}

	.declarations {
		/* Divider between items handled by border-bottom on each item */
	}

	.declaration-item {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 1rem 1.5rem;
		border-bottom: 1px solid var(--color-border);
	}

	.declaration-item:last-child {
		border-bottom: none;
	}

	.declaration-period {
		font-weight: 500;
	}

	.declaration-amounts {
		font-size: 0.875rem;
		color: var(--color-text-muted);
	}

	.declaration-actions {
		display: flex;
		gap: 0.5rem;
		align-items: center;
	}

	.btn-sm {
		padding: 0.25rem 0.75rem;
		font-size: 0.875rem;
	}

	.empty-state {
		text-align: center;
		padding: 3rem;
		color: var(--color-text-muted);
	}

	/* Mobile responsive */
	@media (max-width: 768px) {
		.page-container {
			padding: 1rem;
		}

		h1 {
			font-size: 1.25rem;
		}

		.header-left {
			flex-direction: column;
			align-items: flex-start;
			gap: 0.5rem;
		}

		.form-row {
			flex-direction: column;
			gap: 0.75rem;
		}

		.form-group {
			width: 100%;
			min-width: unset;
		}

		.form-row .input {
			min-height: 44px;
		}

		.form-row .btn {
			width: 100%;
			min-height: 44px;
			justify-content: center;
		}

		.declaration-item {
			flex-direction: column;
			align-items: flex-start;
			gap: 0.75rem;
			padding: 1rem;
		}

		.declaration-amounts {
			font-size: 0.75rem;
		}

		.declaration-actions {
			width: 100%;
			justify-content: space-between;
		}

		.declaration-actions .btn {
			min-height: 44px;
		}

		.empty-state {
			padding: 2rem 1rem;
		}
	}
</style>
