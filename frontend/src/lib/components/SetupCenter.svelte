<script lang="ts">
	import type { DashboardSummary, Tenant } from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';

	interface Props {
		tenant: Tenant;
		summary: DashboardSummary | null;
		onopenwalkthrough?: () => void;
	}

	type SetupTask = {
		id: 'company' | 'branding' | 'opening-balances' | 'contacts' | 'period-lock';
		title: string;
		description: string;
		actionLabel: string;
		href: string;
		completed: boolean;
		status: 'done' | 'next' | 'recommended';
	};

	let { tenant, summary, onopenwalkthrough }: Props = $props();

	function toNumber(value: unknown): number {
		if (value === null || value === undefined) return 0;
		if (typeof value === 'number') return value;
		if (typeof value === 'string') return Number(value);
		if (typeof value === 'object' && value !== null && 'toNumber' in value) {
			return (value as { toNumber(): number }).toNumber();
		}
		return Number(value);
	}

	function hasCompanyProfile(tenantValue: Tenant): boolean {
		const settings = tenantValue.settings ?? {};
		return Boolean(settings.reg_code || settings.vat_number || settings.address || settings.email || settings.phone);
	}

	function hasBranding(tenantValue: Tenant): boolean {
		const settings = tenantValue.settings ?? {};
		return Boolean(
			settings.logo ||
				settings.bank_details ||
				settings.invoice_terms ||
				(settings.pdf_primary_color && settings.pdf_primary_color !== '#4f46e5')
		);
	}

	function hasAccountingActivity(summaryValue: DashboardSummary | null): boolean {
		if (!summaryValue) return false;
		return (
			toNumber(summaryValue.total_revenue) > 0 ||
			toNumber(summaryValue.total_expenses) > 0 ||
			toNumber(summaryValue.total_receivables) > 0 ||
			toNumber(summaryValue.total_payables) > 0
		);
	}

	function hasBillingActivity(summaryValue: DashboardSummary | null): boolean {
		if (!summaryValue) return false;
		return (
			summaryValue.draft_invoices > 0 ||
			summaryValue.pending_invoices > 0 ||
			summaryValue.overdue_invoices > 0 ||
			toNumber(summaryValue.total_receivables) > 0
		);
	}

	let tasks = $derived.by<SetupTask[]>(() => {
		const candidateTasks: SetupTask[] = [
			{
				id: 'company',
				title: m.dashboard_setupTaskCompanyTitle(),
				description: m.dashboard_setupTaskCompanyDesc(),
				actionLabel: m.dashboard_setupTaskCompanyAction(),
				href: `/settings/company?tenant=${tenant.id}`,
				completed: hasCompanyProfile(tenant),
				status: 'recommended'
			},
			{
				id: 'branding',
				title: m.dashboard_setupTaskBrandingTitle(),
				description: m.dashboard_setupTaskBrandingDesc(),
				actionLabel: m.dashboard_setupTaskBrandingAction(),
				href: `/settings/company?tenant=${tenant.id}`,
				completed: hasBranding(tenant),
				status: 'recommended'
			},
			{
				id: 'opening-balances',
				title: m.dashboard_setupTaskOpeningBalancesTitle(),
				description: m.dashboard_setupTaskOpeningBalancesDesc(),
				actionLabel: m.dashboard_setupTaskOpeningBalancesAction(),
				href: `/journal?tenant=${tenant.id}`,
				completed: hasAccountingActivity(summary),
				status: 'recommended'
			},
			{
				id: 'contacts',
				title: m.dashboard_setupTaskContactsTitle(),
				description: m.dashboard_setupTaskContactsDesc(),
				actionLabel: m.dashboard_setupTaskContactsAction(),
				href: `/contacts?tenant=${tenant.id}`,
				completed: hasBillingActivity(summary),
				status: 'recommended'
			},
			{
				id: 'period-lock',
				title: m.dashboard_setupTaskPeriodLockTitle(),
				description: m.dashboard_setupTaskPeriodLockDesc(),
				actionLabel: m.dashboard_setupTaskPeriodLockAction(),
				href: `/settings/company?tenant=${tenant.id}`,
				completed: Boolean(tenant.settings?.period_lock_date),
				status: 'recommended'
			}
		];

		let nextMarked = false;
		return candidateTasks.map((task) => {
			if (task.completed) {
				return { ...task, status: 'done' };
			}
			if (!nextMarked) {
				nextMarked = true;
				return { ...task, status: 'next' };
			}
			return task;
		});
	});

	let completedCount = $derived(tasks.filter((task) => task.completed).length);
	let completionRatio = $derived(tasks.length === 0 ? 0 : (completedCount / tasks.length) * 100);

	function statusLabel(status: SetupTask['status']): string {
		switch (status) {
			case 'done':
				return m.dashboard_setupDone();
			case 'next':
				return m.dashboard_setupNext();
			default:
				return m.dashboard_setupRecommended();
		}
	}
</script>

<section class="setup-center card">
	<div class="setup-center-header">
		<div>
			<div class="setup-eyebrow">{m.dashboard_setupCenter()}</div>
			<h3>{m.dashboard_setupCenterTitle()}</h3>
			<p>{m.dashboard_setupCenterDesc()}</p>
		</div>

		<div class="setup-progress">
			<div class="setup-progress-label">
				<span>{m.dashboard_setupProgress()}</span>
				<strong>{completedCount}/{tasks.length}</strong>
			</div>
			<div class="setup-progress-bar" aria-hidden="true">
				<div class="setup-progress-fill" style={`width: ${completionRatio}%`}></div>
			</div>
			<div class="setup-progress-summary">
				{completedCount} {m.dashboard_setupConfigured()}
			</div>
			{#if onopenwalkthrough && !tenant.onboarding_completed}
				<button type="button" class="btn btn-primary" onclick={onopenwalkthrough}>
					{m.dashboard_continueGuidedSetup()}
				</button>
			{/if}
		</div>
	</div>

	<div class="setup-task-grid">
		{#each tasks as task (task.id)}
			<article class="setup-task-card" class:complete={task.completed} class:next={task.status === 'next'}>
				<div class="task-icon" data-kind={task.id}>
					{#if task.id === 'company'}
						<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<path d="M3 21h18" />
							<path d="M5 21V7l8-4v18" />
							<path d="M19 21V11l-6-4" />
						</svg>
					{:else if task.id === 'branding'}
						<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<circle cx="13.5" cy="6.5" r=".5" />
							<circle cx="17.5" cy="10.5" r=".5" />
							<circle cx="8.5" cy="7.5" r=".5" />
							<circle cx="6.5" cy="12.5" r=".5" />
							<path d="M12 22c4.97 0 9-4.03 9-9 0-4.1-2.75-7.55-6.5-8.62A9 9 0 1 0 12 22Z" />
							<path d="M12 22a4 4 0 0 0 0-8 2 2 0 0 1 0-4 4 4 0 0 0 0-8" />
						</svg>
					{:else if task.id === 'opening-balances'}
						<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<line x1="12" y1="2" x2="12" y2="22" />
							<path d="M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6" />
						</svg>
					{:else if task.id === 'contacts'}
						<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
							<circle cx="9" cy="7" r="4" />
							<path d="M23 21v-2a4 4 0 0 0-3-3.87" />
							<path d="M16 3.13a4 4 0 0 1 0 7.75" />
						</svg>
					{:else}
						<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<circle cx="12" cy="12" r="3" />
							<path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.6 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9c.67 0 1.28.4 1.51 1H21a2 2 0 0 1 0 4h-.09c-.23.6-.84 1-1.51 1Z" />
						</svg>
					{/if}
				</div>

				<div class="task-content">
					<div class="task-header">
						<h4>{task.title}</h4>
						<span class="task-status" data-status={task.status}>{statusLabel(task.status)}</span>
					</div>
					<p>{task.description}</p>
				</div>

				<a class="task-link" href={task.href}>
					{task.actionLabel}
				</a>
			</article>
		{/each}
	</div>
</section>

<style>
	.setup-center {
		margin-bottom: 1.5rem;
	}

	.setup-center-header {
		display: flex;
		justify-content: space-between;
		gap: 1.5rem;
		margin-bottom: 1.5rem;
		align-items: flex-start;
	}

	.setup-eyebrow {
		font-size: 0.75rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--color-primary);
		margin-bottom: 0.35rem;
	}

	.setup-center-header h3 {
		font-size: 1.25rem;
		margin-bottom: 0.35rem;
	}

	.setup-center-header p {
		color: var(--color-text-muted);
		max-width: 42rem;
	}

	.setup-progress {
		min-width: 220px;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.setup-progress-label {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
		font-size: 0.875rem;
		color: var(--color-text-muted);
	}

	.setup-progress-label strong {
		font-size: 1rem;
		color: var(--color-text);
	}

	.setup-progress-bar {
		height: 0.5rem;
		border-radius: 999px;
		background: var(--color-bg);
		overflow: hidden;
	}

	.setup-progress-fill {
		height: 100%;
		border-radius: inherit;
		background: linear-gradient(90deg, var(--color-primary), #38bdf8);
	}

	.setup-progress-summary {
		font-size: 0.8rem;
		color: var(--color-text-muted);
	}

	.setup-task-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
		gap: 1rem;
	}

	.setup-task-card {
		display: flex;
		flex-direction: column;
		gap: 0.85rem;
		padding: 1rem;
		border-radius: 0.75rem;
		border: 1px solid var(--color-border);
		background: linear-gradient(180deg, #fff 0%, #f8fbff 100%);
	}

	.setup-task-card.complete {
		background: linear-gradient(180deg, #ffffff 0%, #f6fcf7 100%);
		border-color: #bbf7d0;
	}

	.setup-task-card.next {
		border-color: rgba(37, 99, 235, 0.35);
		box-shadow: 0 12px 30px rgba(37, 99, 235, 0.08);
	}

	.task-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 2.5rem;
		height: 2.5rem;
		border-radius: 0.75rem;
		background: rgba(37, 99, 235, 0.12);
		color: var(--color-primary);
	}

	.setup-task-card.complete .task-icon {
		background: rgba(34, 197, 94, 0.14);
		color: #15803d;
	}

	.task-header {
		display: flex;
		justify-content: space-between;
		gap: 0.5rem;
		align-items: flex-start;
		margin-bottom: 0.35rem;
	}

	.task-header h4 {
		font-size: 1rem;
	}

	.task-status {
		flex-shrink: 0;
		padding: 0.15rem 0.5rem;
		border-radius: 999px;
		font-size: 0.72rem;
		font-weight: 600;
	}

	.task-status[data-status='done'] {
		background: rgba(34, 197, 94, 0.14);
		color: #15803d;
	}

	.task-status[data-status='next'] {
		background: rgba(37, 99, 235, 0.12);
		color: var(--color-primary);
	}

	.task-status[data-status='recommended'] {
		background: rgba(148, 163, 184, 0.16);
		color: #475569;
	}

	.task-content p {
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}

	.task-link {
		margin-top: auto;
		font-weight: 600;
	}

	@media (max-width: 900px) {
		.setup-center-header {
			flex-direction: column;
		}

		.setup-progress {
			width: 100%;
			min-width: 0;
		}
	}
</style>
