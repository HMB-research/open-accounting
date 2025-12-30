<script lang="ts">
	import { onMount } from 'svelte';
	import {
		api,
		type TenantMembership,
		type Tenant,
		type DashboardSummary,
		type RevenueExpenseChart
	} from '$lib/api';
	import { Chart, registerables } from 'chart.js';
	import Decimal from 'decimal.js';
	import OnboardingWizard from '$lib/components/OnboardingWizard.svelte';
	import * as m from '$lib/paraglide/messages.js';

	Chart.register(...registerables);

	let tenants = $state<TenantMembership[]>([]);
	let selectedTenant = $state<Tenant | null>(null);
	let showOnboarding = $state(false);
	let summary = $state<DashboardSummary | null>(null);
	let chartData = $state<RevenueExpenseChart | null>(null);
	let isLoading = $state(true);
	let isLoadingAnalytics = $state(false);
	let error = $state('');
	let showCreateTenant = $state(false);
	let newTenantName = $state('');
	let newTenantSlug = $state('');
	let chartCanvas: HTMLCanvasElement;
	let chartInstance: Chart | null = null;

	onMount(async () => {
		try {
			tenants = await api.getMyTenants();
			if (tenants.length > 0) {
				selectedTenant = tenants.find((t) => t.is_default)?.tenant || tenants[0].tenant;
				// Show onboarding wizard if tenant hasn't completed onboarding
				if (selectedTenant && !selectedTenant.onboarding_completed) {
					showOnboarding = true;
				}
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load tenants';
		} finally {
			isLoading = false;
		}
	});

	async function handleOnboardingComplete() {
		showOnboarding = false;
		// Reload the tenant to get updated data
		if (selectedTenant) {
			try {
				const updated = await api.getTenant(selectedTenant.id);
				selectedTenant = updated;
				// Update in tenants array too
				const idx = tenants.findIndex((t) => t.tenant.id === updated.id);
				if (idx >= 0) {
					tenants[idx].tenant = updated;
				}
			} catch {
				// Ignore error, we'll just proceed with existing data
			}
		}
	}

	$effect(() => {
		if (selectedTenant) {
			loadAnalytics(selectedTenant.id);
		}
	});

	$effect(() => {
		if (chartData && chartCanvas) {
			renderChart();
		}
	});

	async function loadAnalytics(tenantId: string) {
		isLoadingAnalytics = true;
		try {
			const [summaryData, chartResponse] = await Promise.all([
				api.getDashboardSummary(tenantId),
				api.getRevenueExpenseChart(tenantId, 6)
			]);
			summary = summaryData;
			chartData = chartResponse;
		} catch (err) {
			console.error('Failed to load analytics:', err);
		} finally {
			isLoadingAnalytics = false;
		}
	}

	function renderChart() {
		if (!chartData || !chartCanvas) return;

		if (chartInstance) {
			chartInstance.destroy();
		}

		const ctx = chartCanvas.getContext('2d');
		if (!ctx) return;

		chartInstance = new Chart(ctx, {
			type: 'bar',
			data: {
				labels: chartData.labels,
				datasets: [
					{
						label: 'Revenue',
						data: chartData.revenue.map((d) => (d instanceof Decimal ? d.toNumber() : Number(d))),
						backgroundColor: 'rgba(34, 197, 94, 0.7)',
						borderColor: 'rgb(34, 197, 94)',
						borderWidth: 1
					},
					{
						label: 'Expenses',
						data: chartData.expenses.map((d) => (d instanceof Decimal ? d.toNumber() : Number(d))),
						backgroundColor: 'rgba(239, 68, 68, 0.7)',
						borderColor: 'rgb(239, 68, 68)',
						borderWidth: 1
					}
				]
			},
			options: {
				responsive: true,
				maintainAspectRatio: false,
				plugins: {
					legend: {
						position: 'bottom'
					}
				},
				scales: {
					y: {
						beginAtZero: true
					}
				}
			}
		});
	}

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

	function formatCurrency(value: Decimal | number | string): string {
		const num = typeof value === 'object' && 'toFixed' in value ? value.toNumber() : Number(value);
		return new Intl.NumberFormat('et-EE', {
			style: 'currency',
			currency: 'EUR',
			maximumFractionDigits: 0
		}).format(num);
	}

	function formatPercent(value: Decimal | number | string): string {
		const num = typeof value === 'object' && 'toFixed' in value ? value.toNumber() : Number(value);
		const sign = num >= 0 ? '+' : '';
		return `${sign}${num.toFixed(1)}%`;
	}
</script>

<svelte:head>
	<title>{m.dashboard_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>{m.dashboard_title()}</h1>
		<button class="btn btn-primary" onclick={() => (showCreateTenant = true)}>
			+ {m.dashboard_newOrganization()}
		</button>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<p>{m.common_loading()}</p>
	{:else if tenants.length === 0}
		<div class="card empty-state">
			<h2>{m.dashboard_welcome()}</h2>
			<p>{m.dashboard_createFirst()}</p>
			<button class="btn btn-primary" onclick={() => (showCreateTenant = true)}>
				{m.dashboard_createOrganization()}
			</button>
		</div>
	{:else}
		<!-- Tenant Selector -->
		<div class="tenant-selector">
			<select
				class="input"
				onchange={(e) => {
					const id = (e.target as HTMLSelectElement).value;
					selectedTenant = tenants.find((t) => t.tenant.id === id)?.tenant || null;
				}}
			>
				{#each tenants as membership}
					<option value={membership.tenant.id} selected={selectedTenant?.id === membership.tenant.id}>
						{membership.tenant.name}
					</option>
				{/each}
			</select>
		</div>

		{#if selectedTenant}
			<!-- Summary Cards -->
			{#if summary}
				<div class="summary-grid">
					<div class="summary-card">
						<div class="summary-label">{m.dashboard_revenue()}</div>
						<div class="summary-value positive">{formatCurrency(summary.total_revenue)}</div>
						<div class="summary-change" class:positive={Number(summary.revenue_change) >= 0} class:negative={Number(summary.revenue_change) < 0}>
							{formatPercent(summary.revenue_change)} {m.dashboard_vsLastMonth()}
						</div>
					</div>
					<div class="summary-card">
						<div class="summary-label">{m.dashboard_expenses()}</div>
						<div class="summary-value negative">{formatCurrency(summary.total_expenses)}</div>
						<div class="summary-change" class:positive={Number(summary.expenses_change) < 0} class:negative={Number(summary.expenses_change) >= 0}>
							{formatPercent(summary.expenses_change)} {m.dashboard_vsLastMonth()}
						</div>
					</div>
					<div class="summary-card">
						<div class="summary-label">{m.dashboard_netIncome()}</div>
						<div class="summary-value" class:positive={Number(summary.net_income) >= 0} class:negative={Number(summary.net_income) < 0}>
							{formatCurrency(summary.net_income)}
						</div>
					</div>
					<div class="summary-card">
						<div class="summary-label">{m.dashboard_receivables()}</div>
						<div class="summary-value">{formatCurrency(summary.total_receivables)}</div>
						{#if Number(summary.overdue_receivables) > 0}
							<div class="summary-change negative">
								{formatCurrency(summary.overdue_receivables)} {m.dashboard_overdue()}
							</div>
						{/if}
					</div>
				</div>

				<!-- Invoice Status -->
				<div class="invoice-status card">
					<h3>{m.dashboard_invoiceStatus()}</h3>
					<div class="status-row">
						<div class="status-item">
							<span class="status-count">{summary.draft_invoices}</span>
							<span class="status-label">{m.dashboard_draft()}</span>
						</div>
						<div class="status-item">
							<span class="status-count">{summary.pending_invoices}</span>
							<span class="status-label">{m.dashboard_pending()}</span>
						</div>
						<div class="status-item warning">
							<span class="status-count">{summary.overdue_invoices}</span>
							<span class="status-label">{m.invoices_overdue()}</span>
						</div>
					</div>
				</div>
			{:else if isLoadingAnalytics}
				<div class="summary-loading">{m.common_loading()}</div>
			{/if}

			<!-- Chart -->
			<div class="card chart-card">
				<h3>{m.dashboard_revenueVsExpenses()}</h3>
				<div class="chart-container">
					<canvas bind:this={chartCanvas}></canvas>
				</div>
			</div>

			<!-- Quick Links -->
			<div class="quick-links card">
				<h3>{m.dashboard_quickActions()}</h3>
				<div class="links-row">
					<a href="/invoices?tenant={selectedTenant.id}" class="btn btn-secondary">{m.nav_invoices()}</a>
					<a href="/recurring?tenant={selectedTenant.id}" class="btn btn-secondary">{m.nav_recurring()}</a>
					<a href="/payments?tenant={selectedTenant.id}" class="btn btn-secondary">{m.nav_payments()}</a>
					<a href="/contacts?tenant={selectedTenant.id}" class="btn btn-secondary">{m.nav_contacts()}</a>
					<a href="/accounts?tenant={selectedTenant.id}" class="btn btn-secondary">{m.nav_accounts()}</a>
					<a href="/journal?tenant={selectedTenant.id}" class="btn btn-secondary">{m.nav_journal()}</a>
					<a href="/reports?tenant={selectedTenant.id}" class="btn btn-secondary">{m.nav_reports()}</a>
					<a href="/banking?tenant={selectedTenant.id}" class="btn btn-secondary">{m.nav_banking()}</a>
					<a href="/tax?tenant={selectedTenant.id}" class="btn btn-secondary">{m.nav_tax()}</a>
					<a href="/settings/email?tenant={selectedTenant.id}" class="btn btn-secondary">{m.settings_emailSettings()}</a>
				</div>
			</div>
		{/if}
	{/if}
</div>

{#if showCreateTenant}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showCreateTenant = false)} role="presentation">
		<div class="modal card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="create-org-title" tabindex="-1">
			<h2 id="create-org-title">{m.modal_createOrganization()}</h2>
			<form onsubmit={createTenant}>
				<div class="form-group">
					<label class="label" for="name">{m.modal_organizationName()}</label>
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
					<label class="label" for="slug">{m.modal_urlIdentifier()}</label>
					<input
						class="input"
						type="text"
						id="slug"
						bind:value={newTenantSlug}
						required
						pattern="[a-z0-9][a-z0-9-]*[a-z0-9]"
						placeholder="my-company"
					/>
					<small>{m.modal_urlIdentifierHint()}</small>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={() => (showCreateTenant = false)}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary">{m.common_create()}</button>
				</div>
			</form>
		</div>
	</div>
{/if}

{#if showOnboarding && selectedTenant}
	<OnboardingWizard tenant={selectedTenant} oncomplete={handleOnboardingComplete} />
{/if}

<style>
	.header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1.5rem;
		gap: 1rem;
		flex-wrap: wrap;
	}

	h1 {
		font-size: 1.75rem;
	}

	.tenant-selector {
		margin-bottom: 1.5rem;
		max-width: 300px;
	}

	/* Mobile responsive styles */
	@media (max-width: 768px) {
		.header {
			flex-direction: column;
			align-items: stretch;
		}

		.header .btn {
			width: 100%;
			justify-content: center;
			min-height: 44px; /* Touch target */
		}

		h1 {
			font-size: 1.5rem;
		}

		.tenant-selector {
			max-width: 100%;
		}

		.summary-grid {
			grid-template-columns: 1fr;
		}

		.status-row {
			flex-wrap: wrap;
			gap: 1rem;
		}

		.status-item {
			flex: 1;
			min-width: 80px;
		}

		.chart-container {
			height: 250px;
		}

		.links-row .btn {
			flex: 1 1 calc(50% - 0.25rem);
			min-width: 120px;
			min-height: 44px;
			justify-content: center;
		}

		.modal {
			margin: 0;
			max-width: 100%;
			border-radius: 1rem 1rem 0 0;
			max-height: 90vh;
		}

		.modal-backdrop {
			align-items: flex-end;
			padding: 0;
		}

		.modal-actions {
			flex-direction: column;
		}

		.modal-actions .btn {
			width: 100%;
			min-height: 44px;
			justify-content: center;
		}
	}

	@media (max-width: 480px) {
		h1 {
			font-size: 1.25rem;
		}

		.summary-value {
			font-size: 1.25rem;
		}

		.links-row .btn {
			flex: 1 1 100%;
		}
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

	.summary-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
		gap: 1rem;
		margin-bottom: 1.5rem;
	}

	.summary-card {
		background: var(--color-card);
		border-radius: var(--radius-md);
		padding: 1.25rem;
		border: 1px solid var(--color-border);
	}

	.summary-label {
		font-size: 0.875rem;
		color: var(--color-text-muted);
		margin-bottom: 0.5rem;
	}

	.summary-value {
		font-size: 1.5rem;
		font-weight: 600;
		font-family: var(--font-mono);
	}

	.summary-value.positive {
		color: #22c55e;
	}

	.summary-value.negative {
		color: #ef4444;
	}

	.summary-change {
		font-size: 0.75rem;
		margin-top: 0.25rem;
	}

	.summary-change.positive {
		color: #22c55e;
	}

	.summary-change.negative {
		color: #ef4444;
	}

	.summary-loading {
		text-align: center;
		padding: 2rem;
		color: var(--color-text-muted);
	}

	.invoice-status {
		margin-bottom: 1.5rem;
	}

	.invoice-status h3 {
		margin-bottom: 1rem;
		font-size: 1rem;
	}

	.status-row {
		display: flex;
		gap: 2rem;
	}

	.status-item {
		display: flex;
		flex-direction: column;
		align-items: center;
	}

	.status-count {
		font-size: 1.5rem;
		font-weight: 600;
	}

	.status-label {
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}

	.status-item.warning .status-count {
		color: #ef4444;
	}

	.chart-card {
		margin-bottom: 1.5rem;
	}

	.chart-card h3 {
		margin-bottom: 1rem;
		font-size: 1rem;
	}

	.chart-container {
		height: 300px;
	}

	.quick-links h3 {
		margin-bottom: 1rem;
		font-size: 1rem;
	}

	.links-row {
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
