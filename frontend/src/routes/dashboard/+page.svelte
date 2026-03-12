<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import {
		api,
		type TenantMembership,
		type Tenant,
		type DashboardSummary,
		type RevenueExpenseChart,
		type CashFlowChart,
		type ActivityItem
	} from '$lib/api';
	import { Chart, registerables } from 'chart.js';
	import Decimal from 'decimal.js';
	import OnboardingWizard from '$lib/components/OnboardingWizard.svelte';
	import PeriodSelector from '$lib/components/PeriodSelector.svelte';
	import ActivityFeed from '$lib/components/ActivityFeed.svelte';
	import SetupCenter from '$lib/components/SetupCenter.svelte';
	import * as m from '$lib/paraglide/messages.js';

	Chart.register(...registerables);

	type Period = 'THIS_MONTH' | 'LAST_MONTH' | 'THIS_QUARTER' | 'THIS_YEAR' | 'CUSTOM';

	let tenants = $state<TenantMembership[]>([]);
	let selectedTenant = $state<Tenant | null>(null);
	let showOnboarding = $state(false);
	let summary = $state<DashboardSummary | null>(null);
	let chartData = $state<RevenueExpenseChart | null>(null);
	let cashFlowData = $state<CashFlowChart | null>(null);
	let activityItems = $state<ActivityItem[]>([]);
	let isLoading = $state(true);
	let isLoadingAnalytics = $state(false);
	let isLoadingActivity = $state(false);
	let error = $state('');
	let showCreateTenant = $state(false);
	let newTenantName = $state('');
	let newTenantSlug = $state('');
	let chartCanvas = $state<HTMLCanvasElement | undefined>(undefined);
	let cashFlowCanvas = $state<HTMLCanvasElement | undefined>(undefined);
	let chartInstance: Chart | null = null;
	let cashFlowChartInstance: Chart | null = null;

	// Period selector state
	let selectedPeriod = $state<Period>('THIS_MONTH');
	let startDate = $state('');
	let endDate = $state('');

	onMount(async () => {
		try {
			tenants = await api.getMyTenants();
			if (tenants.length > 0) {
				// Check for tenant ID in URL parameter first
				const urlTenantId = $page.url.searchParams.get('tenant');
				if (urlTenantId) {
					const urlTenant = tenants.find((t) => t.tenant.id === urlTenantId)?.tenant;
					if (urlTenant) {
						selectedTenant = urlTenant;
					} else {
						// URL tenant not found in user's tenants, fall back to default
						selectedTenant = tenants.find((t) => t.is_default)?.tenant || tenants[0].tenant;
					}
				} else {
					selectedTenant = tenants.find((t) => t.is_default)?.tenant || tenants[0].tenant;
				}
				// Show onboarding wizard if tenant hasn't completed onboarding
				if (selectedTenant && !selectedTenant.onboarding_completed) {
					showOnboarding = true;
				}
				// Update URL with tenant ID if not already present
				if (selectedTenant && !urlTenantId) {
					const url = new URL($page.url);
					url.searchParams.set('tenant', selectedTenant.id);
					goto(url.toString(), { replaceState: true, noScroll: true });
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

	function openOnboarding() {
		showOnboarding = true;
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

	$effect(() => {
		if (cashFlowData && cashFlowCanvas) {
			renderCashFlowChart();
		}
	});

	async function loadAnalytics(tenantId: string) {
		isLoadingAnalytics = true;
		isLoadingActivity = true;
		try {
			// Load summary and chart data (required)
			const [summaryData, chartResponse] = await Promise.all([
				api.getDashboardSummary(tenantId),
				api.getRevenueExpenseChart(tenantId, 6)
			]);
			summary = summaryData;
			chartData = chartResponse;

			// Load activity separately (optional, endpoint may not exist)
			try {
				activityItems = await api.getRecentActivity(tenantId, 10);
			} catch {
				// Activity endpoint not available, use empty array
				activityItems = [];
			}

			// Load cash flow with current period
			await loadCashFlow(tenantId);
		} catch (err) {
			console.error('Failed to load analytics:', err);
		} finally {
			isLoadingAnalytics = false;
			isLoadingActivity = false;
		}
	}

	async function loadCashFlow(tenantId: string) {
		try {
			// Calculate dates based on selected period
			const now = new Date();
			let start: Date;
			let end: Date = new Date(now.getFullYear(), now.getMonth() + 1, 0); // End of current month

			switch (selectedPeriod) {
				case 'THIS_MONTH':
					start = new Date(now.getFullYear(), now.getMonth(), 1);
					break;
				case 'LAST_MONTH':
					start = new Date(now.getFullYear(), now.getMonth() - 1, 1);
					end = new Date(now.getFullYear(), now.getMonth(), 0);
					break;
				case 'THIS_QUARTER':
					const quarter = Math.floor(now.getMonth() / 3);
					start = new Date(now.getFullYear(), quarter * 3, 1);
					break;
				case 'THIS_YEAR':
					start = new Date(now.getFullYear(), 0, 1);
					break;
				case 'CUSTOM':
					if (startDate && endDate) {
						start = new Date(startDate);
						end = new Date(endDate);
					} else {
						start = new Date(now.getFullYear(), now.getMonth(), 1);
					}
					break;
				default:
					start = new Date(now.getFullYear(), now.getMonth(), 1);
			}

			const startStr = start.toISOString().split('T')[0];
			const endStr = end.toISOString().split('T')[0];

			cashFlowData = await api.getCashFlowAnalytics(tenantId, startStr, endStr);
		} catch (err) {
			console.error('Failed to load cash flow:', err);
		}
	}

	function handlePeriodChange(period: Period, start: string, end: string) {
		selectedPeriod = period;
		startDate = start;
		endDate = end;
		if (selectedTenant) {
			loadCashFlow(selectedTenant.id);
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
						label: m.dashboard_revenue(),
						data: chartData.revenue.map((d) => (d instanceof Decimal ? d.toNumber() : Number(d))),
						backgroundColor: 'rgba(34, 197, 94, 0.7)',
						borderColor: 'rgb(34, 197, 94)',
						borderWidth: 1
					},
					{
						label: m.dashboard_expenses(),
						data: chartData.expenses.map((d) => (d instanceof Decimal ? d.toNumber() : Number(d))),
						backgroundColor: 'rgba(239, 68, 68, 0.7)',
						borderColor: 'rgb(239, 68, 68)',
						borderWidth: 1
					},
					{
						type: 'line',
						label: m.dashboard_profit_loss(),
						data: chartData.profit.map((d) => (d instanceof Decimal ? d.toNumber() : Number(d))),
						borderColor: 'rgb(59, 130, 246)',
						backgroundColor: 'rgba(59, 130, 246, 0.1)',
						borderWidth: 2,
						fill: false,
						tension: 0.1
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

	function renderCashFlowChart() {
		if (!cashFlowData || !cashFlowCanvas) return;

		if (cashFlowChartInstance) {
			cashFlowChartInstance.destroy();
		}

		const ctx = cashFlowCanvas.getContext('2d');
		if (!ctx) return;

		cashFlowChartInstance = new Chart(ctx, {
			type: 'bar',
			data: {
				labels: cashFlowData.labels,
				datasets: [
					{
						label: m.dashboard_inflow(),
						data: cashFlowData.inflows.map((d) => (d instanceof Decimal ? d.toNumber() : Number(d))),
						backgroundColor: 'rgba(34, 197, 94, 0.7)',
						borderColor: 'rgb(34, 197, 94)',
						borderWidth: 1
					},
					{
						label: m.dashboard_outflow(),
						data: cashFlowData.outflows.map((d) => (d instanceof Decimal ? d.toNumber() : Number(d))),
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
					// Update URL with new tenant ID
					if (selectedTenant) {
						const url = new URL($page.url);
						url.searchParams.set('tenant', selectedTenant.id);
						goto(url.toString(), { replaceState: true, noScroll: true });
					}
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
			<div class="workspace-hero card">
				<div class="workspace-copy">
					<div class="workspace-meta">
						<div class="workspace-badge" data-state={selectedTenant.onboarding_completed ? 'ready' : 'setup'}>
							{selectedTenant.onboarding_completed ? m.dashboard_workspaceReady() : m.dashboard_setupInProgress()}
						</div>
						<span class="workspace-slug">/{selectedTenant.slug}</span>
					</div>
					<h2>{selectedTenant.name}</h2>
					<p>
						{selectedTenant.onboarding_completed
							? m.dashboard_workspaceReadyDesc()
							: m.dashboard_workspaceSetupDesc()}
					</p>
					<div class="workspace-rail">
						<span>{m.nav_invoices()}</span>
						<span>{m.nav_banking()}</span>
						<span>{m.nav_payroll()}</span>
						<span>{m.nav_reports()}</span>
					</div>

					<div class="workspace-actions">
						{#if selectedTenant.onboarding_completed}
							<a href="/invoices?tenant={selectedTenant.id}" class="btn btn-primary">
								{m.invoices_newInvoice()}
							</a>
							<a href="/reports?tenant={selectedTenant.id}" class="btn btn-secondary">
								{m.dashboard_openReports()}
							</a>
						{:else}
							<button class="btn btn-primary" type="button" onclick={openOnboarding}>
								{m.dashboard_continueGuidedSetup()}
							</button>
							<a href="/settings/company?tenant={selectedTenant.id}" class="btn btn-secondary">
								{m.dashboard_setupTaskCompanyAction()}
							</a>
						{/if}
					</div>
				</div>

				<div class="workspace-side">
					<section class="workspace-panel workspace-panel-primary">
						<div class="workspace-panel-kicker">{m.dashboard_invoiceStatus()}</div>
						<div class="workspace-signal-grid">
							<div class="workspace-signal">
								<span>{m.dashboard_receivables()}</span>
								<strong>{summary ? formatCurrency(summary.total_receivables) : '--'}</strong>
							</div>
							<div class="workspace-signal">
								<span>{m.dashboard_netIncome()}</span>
								<strong>{summary ? formatCurrency(summary.net_income) : '--'}</strong>
							</div>
							<div class="workspace-signal">
								<span>{m.invoices_overdue()}</span>
								<strong>{summary ? summary.overdue_invoices : '--'}</strong>
							</div>
							<div class="workspace-signal">
								<span>{m.dashboard_pending()}</span>
								<strong>{summary ? summary.pending_invoices : '--'}</strong>
							</div>
						</div>
					</section>

					<section class="workspace-panel">
						<div class="workspace-panel-kicker">{m.dashboard_quickActions()}</div>
						<div class="workspace-chip-row">
							<a href="/contacts?tenant={selectedTenant.id}" class="workspace-chip">{m.nav_contacts()}</a>
							<a href="/journal?tenant={selectedTenant.id}" class="workspace-chip">{m.nav_journal()}</a>
							<a href="/banking?tenant={selectedTenant.id}" class="workspace-chip">{m.nav_banking()}</a>
							<a href="/tax?tenant={selectedTenant.id}" class="workspace-chip">{m.nav_tax()}</a>
						</div>
					</section>
				</div>
			</div>

			<SetupCenter tenant={selectedTenant} {summary} onopenwalkthrough={openOnboarding} />

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

			<!-- Period Selector and Cash Flow Chart -->
			<div class="analytics-section">
				<div class="card chart-card cash-flow-card">
					<div class="chart-header">
						<h3>{m.dashboard_cashFlow()}</h3>
						<PeriodSelector
							bind:value={selectedPeriod}
							bind:startDate={startDate}
							bind:endDate={endDate}
							onchange={handlePeriodChange}
						/>
					</div>
					<div class="chart-container">
						<canvas bind:this={cashFlowCanvas}></canvas>
					</div>
					{#if cashFlowData}
						<div class="cash-flow-summary">
							<div class="cf-stat positive">
								<span class="cf-label">{m.dashboard_inflow()}</span>
								<span class="cf-value">{formatCurrency(cashFlowData.inflows.reduce((a, b) => Number(a) + Number(b), 0))}</span>
							</div>
							<div class="cf-stat negative">
								<span class="cf-label">{m.dashboard_outflow()}</span>
								<span class="cf-value">{formatCurrency(cashFlowData.outflows.reduce((a, b) => Number(a) + Number(b), 0))}</span>
							</div>
							<div class="cf-stat" class:positive={cashFlowData.inflows.reduce((a, b) => Number(a) + Number(b), 0) - cashFlowData.outflows.reduce((a, b) => Number(a) + Number(b), 0) >= 0} class:negative={cashFlowData.inflows.reduce((a, b) => Number(a) + Number(b), 0) - cashFlowData.outflows.reduce((a, b) => Number(a) + Number(b), 0) < 0}>
								<span class="cf-label">{m.dashboard_netChange()}</span>
								<span class="cf-value">{formatCurrency(cashFlowData.inflows.reduce((a, b) => Number(a) + Number(b), 0) - cashFlowData.outflows.reduce((a, b) => Number(a) + Number(b), 0))}</span>
							</div>
						</div>
					{/if}
				</div>

				<ActivityFeed items={activityItems} loading={isLoadingActivity} />
			</div>

			<!-- Revenue vs Expenses Chart -->
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

	.workspace-hero {
		display: grid;
		grid-template-columns: minmax(0, 1.5fr) minmax(280px, 0.95fr);
		gap: 1.5rem;
		margin-bottom: 1.75rem;
		padding: 1.75rem;
		position: relative;
		overflow: hidden;
		background:
			radial-gradient(circle at top right, rgba(37, 99, 235, 0.18), transparent 28%),
			radial-gradient(circle at bottom left, rgba(15, 23, 42, 0.08), transparent 38%),
			linear-gradient(135deg, rgba(255, 255, 255, 0.84), rgba(255, 250, 242, 0.96));
		box-shadow: var(--shadow-soft);
	}

	.workspace-copy {
		position: relative;
		z-index: 1;
		max-width: 46rem;
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 1rem;
	}

	.workspace-meta {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		flex-wrap: wrap;
	}

	.workspace-badge {
		display: inline-flex;
		align-items: center;
		padding: 0.25rem 0.65rem;
		border-radius: 999px;
		font-size: 0.75rem;
		font-weight: 700;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		margin-bottom: 0.75rem;
	}

	.workspace-badge[data-state='ready'] {
		background: rgba(34, 197, 94, 0.12);
		color: #15803d;
	}

	.workspace-badge[data-state='setup'] {
		background: rgba(37, 99, 235, 0.12);
		color: var(--color-primary);
	}

	.workspace-slug {
		font-family: var(--font-mono);
		font-size: 0.78rem;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--color-text-muted);
	}

	.workspace-hero h2 {
		font-family: var(--font-display);
		font-size: clamp(2.8rem, 6vw, 5.1rem);
		line-height: 0.88;
		margin-bottom: 0;
		letter-spacing: -0.04em;
		font-weight: 600;
	}

	.workspace-hero p {
		color: var(--color-text-muted);
		max-width: 42rem;
		font-size: 1rem;
	}

	.workspace-rail {
		display: flex;
		flex-wrap: wrap;
		gap: 0.55rem;
	}

	.workspace-rail span {
		padding: 0.45rem 0.8rem;
		border-radius: 999px;
		background: rgba(255, 255, 255, 0.65);
		border: 1px solid rgba(30, 41, 59, 0.08);
		font-size: 0.78rem;
		font-weight: 600;
		color: var(--color-text-muted);
	}

	.workspace-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.75rem;
		justify-content: flex-start;
	}

	.workspace-side {
		position: relative;
		z-index: 1;
		display: grid;
		gap: 1rem;
		align-content: start;
	}

	.workspace-panel {
		padding: 1.1rem;
		border-radius: 1.35rem;
		border: 1px solid rgba(30, 41, 59, 0.08);
		background: rgba(255, 255, 255, 0.66);
		backdrop-filter: blur(16px);
	}

	.workspace-panel-primary {
		background: linear-gradient(180deg, rgba(19, 30, 56, 0.98), rgba(33, 52, 87, 0.96));
		color: #f8f6f1;
		border-color: rgba(255, 255, 255, 0.08);
		box-shadow: 0 24px 48px rgba(15, 23, 42, 0.16);
	}

	.workspace-panel-kicker {
		font-size: 0.75rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--color-text-muted);
		margin-bottom: 0.85rem;
	}

	.workspace-panel-primary .workspace-panel-kicker {
		color: rgba(248, 246, 241, 0.72);
	}

	.workspace-signal-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.85rem;
	}

	.workspace-signal {
		padding: 0.85rem;
		border-radius: 1rem;
		background: rgba(255, 255, 255, 0.08);
	}

	.workspace-signal span {
		display: block;
		font-size: 0.72rem;
		text-transform: uppercase;
		letter-spacing: 0.07em;
		color: rgba(248, 246, 241, 0.68);
		margin-bottom: 0.3rem;
	}

	.workspace-signal strong {
		font-size: 1.05rem;
		line-height: 1.1;
	}

	.workspace-chip-row {
		display: flex;
		flex-wrap: wrap;
		gap: 0.55rem;
	}

	.workspace-chip {
		display: inline-flex;
		align-items: center;
		padding: 0.55rem 0.85rem;
		border-radius: 999px;
		background: rgba(37, 99, 235, 0.08);
		border: 1px solid rgba(37, 99, 235, 0.12);
		color: var(--color-primary);
		font-weight: 600;
		text-decoration: none;
	}

	.workspace-chip:hover {
		text-decoration: none;
		background: rgba(37, 99, 235, 0.12);
	}

	/* Mobile responsive styles */
	@media (max-width: 768px) {
		.header {
			flex-direction: column;
			align-items: stretch;
		}

		.workspace-hero {
			grid-template-columns: 1fr;
			padding: 1.35rem;
		}

		.workspace-actions {
			width: 100%;
		}

		.workspace-actions .btn {
			width: 100%;
			justify-content: center;
			min-height: 44px;
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
		margin: -0.4rem 0 1.5rem;
		position: relative;
		z-index: 1;
	}

	.summary-card {
		background: rgba(255, 255, 255, 0.74);
		border-radius: 1.15rem;
		padding: 1.25rem;
		border: 1px solid rgba(30, 41, 59, 0.08);
		box-shadow: 0 14px 30px rgba(15, 23, 42, 0.05);
		backdrop-filter: blur(14px);
	}

	.summary-label {
		font-size: 0.72rem;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--color-text-muted);
		margin-bottom: 0.65rem;
	}

	.summary-value {
		font-size: 1.7rem;
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
		background: rgba(255, 255, 255, 0.72);
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

	.analytics-section {
		display: grid;
		grid-template-columns: 2fr 1fr;
		gap: 1.5rem;
		margin-bottom: 1.5rem;
	}

	@media (max-width: 1024px) {
		.analytics-section {
			grid-template-columns: 1fr;
		}
	}

	.chart-card {
		margin-bottom: 1.5rem;
		background: linear-gradient(180deg, rgba(255, 255, 255, 0.8), rgba(255, 251, 244, 0.92));
	}

	.cash-flow-card {
		margin-bottom: 0;
	}

	.chart-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1rem;
		gap: 1rem;
		flex-wrap: wrap;
	}

	.chart-header h3 {
		margin: 0;
		font-size: 1rem;
	}

	.chart-card h3 {
		margin-bottom: 1rem;
		font-size: 1rem;
	}

	.chart-container {
		height: 300px;
	}

	.cash-flow-summary {
		display: flex;
		justify-content: space-around;
		margin-top: 1rem;
		padding-top: 1rem;
		border-top: 1px solid var(--color-border);
	}

	.cf-stat {
		text-align: center;
	}

	.cf-label {
		display: block;
		font-size: 0.75rem;
		color: var(--color-text-muted);
		margin-bottom: 0.25rem;
	}

	.cf-value {
		font-size: 1rem;
		font-weight: 600;
		font-family: var(--font-mono);
	}

	.cf-stat.positive .cf-value {
		color: #22c55e;
	}

	.cf-stat.negative .cf-value {
		color: #ef4444;
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

	.quick-links {
		background: linear-gradient(135deg, rgba(255, 255, 255, 0.82), rgba(247, 250, 255, 0.94));
	}

	.quick-links :global(.btn-secondary) {
		background: rgba(255, 255, 255, 0.8);
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
