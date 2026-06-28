<script lang="ts">
	import type { DashboardSummary } from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';

	let { summary }: { summary: DashboardSummary | null } = $props();

	type DecimalLike = {
		toNumber: () => number;
	};

	function isDecimalLike(value: unknown): value is DecimalLike {
		return (
			typeof value === 'object' &&
			value !== null &&
			'toNumber' in value &&
			typeof (value as DecimalLike).toNumber === 'function'
		);
	}

	function formatCurrency(value: DecimalLike | number | string): string {
		const num = isDecimalLike(value) ? value.toNumber() : Number(value);
		return new Intl.NumberFormat('et-EE', {
			style: 'currency',
			currency: 'EUR',
			maximumFractionDigits: 0
		}).format(num);
	}

	function formatted(value: DecimalLike | number | string | undefined): string {
		return summary && value !== undefined ? formatCurrency(value) : '--';
	}

	function count(value: number | undefined): string {
		return summary && value !== undefined ? String(value) : '--';
	}
</script>

<div class="workspace-signal-grid" aria-label={m.dashboard_financialSignals()}>
	<div class="workspace-signal">
		<span>{m.dashboard_receivables()}</span>
		<strong>{formatted(summary?.total_receivables)}</strong>
	</div>
	<div class="workspace-signal">
		<span>{m.dashboard_payables()}</span>
		<strong>{formatted(summary?.total_payables)}</strong>
	</div>
	<div class="workspace-signal">
		<span>{m.dashboard_overdueReceivables()}</span>
		<strong>{formatted(summary?.overdue_receivables)}</strong>
	</div>
	<div class="workspace-signal">
		<span>{m.dashboard_overduePayables()}</span>
		<strong>{formatted(summary?.overdue_payables)}</strong>
	</div>
	<div class="workspace-signal">
		<span>{m.dashboard_netIncome()}</span>
		<strong>{formatted(summary?.net_income)}</strong>
	</div>
	<div class="workspace-signal">
		<span>{m.dashboard_pending()}</span>
		<strong>{count(summary?.pending_invoices)}</strong>
	</div>
</div>

<style>
	.workspace-signal-grid {
		display: grid;
		min-width: 0;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.85rem;
	}

	.workspace-signal {
		min-width: 0;
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
		display: block;
		min-width: 0;
		font-size: 1.05rem;
		line-height: 1.1;
		overflow-wrap: anywhere;
	}

	@media (max-width: 480px) {
		.workspace-signal-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
