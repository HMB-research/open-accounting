<script lang="ts">
	import type { YearEndCloseStatus } from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';

	interface Props {
		status: YearEndCloseStatus | null;
		periodEndDate: string;
		currency?: string;
		errorMessage?: string;
		successMessage?: string;
		isLoading?: boolean;
		isSubmitting?: boolean;
		onrefresh?: () => void;
		onsubmit?: () => void;
		onperiodenddatechange?: (value: string) => void;
	}

	type ChecklistItem = {
		id: 'year-end' | 'closed' | 'activity' | 'retained';
		label: string;
		description: string;
		status: 'done' | 'pending';
	};

	let {
		status,
		periodEndDate,
		currency = 'EUR',
		errorMessage = '',
		successMessage = '',
		isLoading = false,
		isSubmitting = false,
		onrefresh,
		onsubmit,
		onperiodenddatechange
	}: Props = $props();

	function updatePeriodEndDate(event: Event) {
		onperiodenddatechange?.((event.currentTarget as HTMLInputElement).value);
	}

	function formatDateLabel(value: string | null | undefined): string {
		if (!value) {
			return m.common_notSet();
		}

		const parsed = new Date(`${value}T00:00:00Z`);
		if (Number.isNaN(parsed.getTime())) {
			return value;
		}

		return new Intl.DateTimeFormat(undefined, {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			timeZone: 'UTC'
		}).format(parsed);
	}

	function formatMoney(value: unknown, currencyCode: string): string {
		const amount = toNumber(value);
		return new Intl.NumberFormat(undefined, {
			style: 'currency',
			currency: currencyCode,
			minimumFractionDigits: 2,
			maximumFractionDigits: 2
		}).format(amount);
	}

	function toNumber(value: unknown): number {
		if (value === null || value === undefined) return 0;
		if (typeof value === 'number') return value;
		if (typeof value === 'string') return Number(value);
		if (typeof value === 'object' && value !== null && 'toNumber' in value) {
			return (value as { toNumber(): number }).toNumber();
		}
		return Number(value);
	}

	let checklist = $derived.by<ChecklistItem[]>(() => {
		if (!status) {
			return [];
		}

		return [
			{
				id: 'year-end',
				label: m.settings_yearEndChecklistDate(),
				description: status.is_fiscal_year_end ? m.settings_yearEndChecklistDateDone() : m.settings_yearEndChecklistDatePending(),
				status: status.is_fiscal_year_end ? 'done' : 'pending'
			},
			{
				id: 'closed',
				label: m.settings_yearEndChecklistClose(),
				description: status.period_closed
					? m.settings_yearEndChecklistCloseDone({ date: formatDateLabel(status.locked_through_date || status.period_end_date) })
					: m.settings_yearEndChecklistClosePending(),
				status: status.period_closed ? 'done' : 'pending'
			},
			{
				id: 'activity',
				label: m.settings_yearEndChecklistActivity(),
				description: status.has_profit_and_loss_activity
					? m.settings_yearEndChecklistActivityDone()
					: m.settings_yearEndChecklistActivityPending(),
				status: status.has_profit_and_loss_activity ? 'done' : 'pending'
			},
			{
				id: 'retained',
				label: m.settings_yearEndChecklistRetained(),
				description: status.has_retained_earnings_account
					? m.settings_yearEndChecklistRetainedDone({ code: status.retained_earnings_account?.code || '' })
					: m.settings_yearEndChecklistRetainedPending(),
				status: status.has_retained_earnings_account ? 'done' : 'pending'
			}
		];
	});

	let badgeLabel = $derived.by(() => {
		if (!status) {
			return m.settings_yearEndStatusIdle();
		}
		if (status.existing_carry_forward) {
			return m.settings_yearEndStatusComplete();
		}
		if (status.carry_forward_ready) {
			return m.settings_yearEndStatusReady();
		}
		return m.settings_yearEndStatusAttention();
	});

	let badgeTone = $derived.by<'success' | 'warning' | 'muted'>(() => {
		if (!status) {
			return 'muted';
		}
		if (status.existing_carry_forward || status.carry_forward_ready) {
			return 'success';
		}
		return 'warning';
	});
</script>

<section class="card year-end-panel">
	<div class="section-header">
		<div>
			<div class="summary-label">{m.settings_yearEndTitle()}</div>
			<h3>{m.settings_yearEndHeading()}</h3>
			<p class="section-description">{m.settings_yearEndDesc()}</p>
		</div>
		<span class={`year-end-badge year-end-badge-${badgeTone}`}>{badgeLabel}</span>
	</div>

	<div class="year-end-toolbar">
		<div class="form-group">
			<label class="label" for="yearEndPeriodEndDate">{m.settings_yearEndTargetDate()}</label>
			<input class="input" id="yearEndPeriodEndDate" type="date" value={periodEndDate} onchange={updatePeriodEndDate} />
		</div>
		<div class="year-end-toolbar-actions">
			<button type="button" class="btn btn-secondary" disabled={isLoading || !periodEndDate} onclick={onrefresh}>
				{isLoading ? m.common_loading() : m.settings_yearEndRefresh()}
			</button>
			<button
				type="button"
				class="btn btn-primary"
				disabled={isSubmitting || !status?.carry_forward_ready}
				onclick={onsubmit}
			>
				{isSubmitting ? m.settings_yearEndCarryForwardSubmitting() : m.settings_yearEndCarryForwardAction()}
			</button>
		</div>
	</div>

	{#if errorMessage}
		<div class="alert alert-error">{errorMessage}</div>
	{/if}

	{#if successMessage}
		<div class="alert alert-success">{successMessage}</div>
	{/if}

	{#if isLoading && !status}
		<p>{m.common_loading()}</p>
	{:else if status}
		<div class="year-end-summary-grid">
			<div class="year-end-summary-card">
				<span class="summary-label">{m.settings_yearEndFiscalYear()}</span>
				<strong>{status.fiscal_year_label}</strong>
				<span class="help-text">
					{formatDateLabel(status.fiscal_year_start_date)} - {formatDateLabel(status.fiscal_year_end_date)}
				</span>
			</div>
			<div class="year-end-summary-card">
				<span class="summary-label">{m.settings_yearEndCarryForwardDate()}</span>
				<strong>{formatDateLabel(status.carry_forward_date)}</strong>
				<span class="help-text">{m.settings_yearEndCarryForwardDateHelp()}</span>
			</div>
			<div class="year-end-summary-card">
				<span class="summary-label">{m.settings_yearEndNetIncome()}</span>
				<strong>{formatMoney(status.net_income, currency)}</strong>
				<span class="help-text">{m.settings_yearEndNetIncomeHelp()}</span>
			</div>
			<div class="year-end-summary-card">
				<span class="summary-label">{m.settings_yearEndRetainedEarnings()}</span>
				<strong>
					{#if status.retained_earnings_account}
						{status.retained_earnings_account.code} · {status.retained_earnings_account.name}
					{:else}
						{m.common_notSet()}
					{/if}
				</strong>
				<span class="help-text">{m.settings_yearEndRetainedEarningsHelp()}</span>
			</div>
		</div>

		<div class="year-end-content-grid">
			<div class="year-end-checklist">
				<div class="year-end-subheader">
					<h4>{m.settings_yearEndChecklistTitle()}</h4>
					<p>{m.settings_yearEndChecklistDesc()}</p>
				</div>
				<ul class="year-end-checklist-list">
					{#each checklist as item (item.id)}
						<li class="year-end-checklist-item">
							<span class="check-indicator" data-status={item.status}></span>
							<div>
								<strong>{item.label}</strong>
								<p>{item.description}</p>
							</div>
						</li>
					{/each}
				</ul>
			</div>

			<div class="year-end-callout">
				<div class="year-end-subheader">
					<h4>{m.settings_yearEndNextActionTitle()}</h4>
					<p>{m.settings_yearEndNextActionDesc()}</p>
				</div>

				{#if status.existing_carry_forward}
					<div class="year-end-note success">
						<strong>{m.settings_yearEndExistingEntryTitle()}</strong>
						<p>
							{m.settings_yearEndExistingEntryDesc({
								entryNumber: status.existing_carry_forward.entry_number,
								date: formatDateLabel(status.existing_carry_forward.entry_date)
							})}
						</p>
					</div>
				{:else if status.carry_forward_ready}
					<div class="year-end-note success">
						<strong>{m.settings_yearEndReadyTitle()}</strong>
						<p>{m.settings_yearEndReadyDesc()}</p>
					</div>
				{:else if !status.is_fiscal_year_end}
					<div class="year-end-note warning">
						<strong>{m.settings_yearEndNotYearEndTitle()}</strong>
						<p>{m.settings_yearEndNotYearEndDesc({ date: formatDateLabel(status.fiscal_year_end_date) })}</p>
					</div>
				{:else if !status.period_closed}
					<div class="year-end-note warning">
						<strong>{m.settings_yearEndNeedsCloseTitle()}</strong>
						<p>{m.settings_yearEndNeedsCloseDesc()}</p>
					</div>
				{:else if !status.has_profit_and_loss_activity}
					<div class="year-end-note muted">
						<strong>{m.settings_yearEndNoActivityTitle()}</strong>
						<p>{m.settings_yearEndNoActivityDesc()}</p>
					</div>
				{:else}
					<div class="year-end-note warning">
						<strong>{m.settings_yearEndNeedsRetainedTitle()}</strong>
						<p>{m.settings_yearEndNeedsRetainedDesc()}</p>
					</div>
				{/if}
			</div>
		</div>
	{/if}
</section>

<style>
	.year-end-panel {
		margin-top: 1.5rem;
	}

	.year-end-badge {
		display: inline-flex;
		align-items: center;
		padding: 0.5rem 0.875rem;
		border-radius: 999px;
		font-size: 0.82rem;
		font-weight: 600;
	}

	.year-end-badge-success {
		background: color-mix(in srgb, var(--color-success, #16a34a) 12%, white);
		color: var(--color-success, #166534);
	}

	.year-end-badge-warning {
		background: color-mix(in srgb, var(--color-warning, #d97706) 12%, white);
		color: var(--color-warning, #92400e);
	}

	.year-end-badge-muted {
		background: color-mix(in srgb, var(--color-text-muted, #64748b) 12%, white);
		color: var(--color-text-muted, #475569);
	}

	.year-end-toolbar {
		display: flex;
		justify-content: space-between;
		align-items: end;
		gap: 1rem;
		margin-bottom: 1rem;
	}

	.year-end-toolbar-actions {
		display: flex;
		gap: 0.75rem;
	}

	.year-end-summary-grid {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		gap: 1rem;
		margin-bottom: 1rem;
	}

	.year-end-summary-card {
		padding: 1rem;
		border: 1px solid var(--color-border);
		border-radius: 0.75rem;
		background: color-mix(in srgb, var(--color-surface, white) 94%, var(--color-primary, #1d4ed8) 6%);
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
	}

	.year-end-content-grid {
		display: grid;
		grid-template-columns: 1.35fr 1fr;
		gap: 1rem;
	}

	.year-end-checklist,
	.year-end-callout {
		padding: 1rem;
		border: 1px solid var(--color-border);
		border-radius: 0.75rem;
		background: var(--color-surface, white);
	}

	.year-end-subheader h4 {
		margin: 0 0 0.25rem;
	}

	.year-end-subheader p {
		margin: 0;
		color: var(--color-text-muted);
		font-size: 0.9rem;
	}

	.year-end-checklist-list {
		list-style: none;
		margin: 1rem 0 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.9rem;
	}

	.year-end-checklist-item {
		display: flex;
		gap: 0.75rem;
		align-items: flex-start;
	}

	.year-end-checklist-item p {
		margin: 0.2rem 0 0;
		color: var(--color-text-muted);
		font-size: 0.9rem;
	}

	.check-indicator {
		width: 0.85rem;
		height: 0.85rem;
		border-radius: 999px;
		margin-top: 0.25rem;
		flex: 0 0 auto;
		background: color-mix(in srgb, var(--color-warning, #d97706) 22%, white);
		border: 1px solid color-mix(in srgb, var(--color-warning, #d97706) 45%, white);
	}

	.check-indicator[data-status='done'] {
		background: color-mix(in srgb, var(--color-success, #16a34a) 22%, white);
		border-color: color-mix(in srgb, var(--color-success, #16a34a) 45%, white);
	}

	.year-end-note {
		margin-top: 1rem;
		padding: 1rem;
		border-radius: 0.75rem;
		border: 1px solid var(--color-border);
	}

	.year-end-note strong {
		display: block;
		margin-bottom: 0.35rem;
	}

	.year-end-note p {
		margin: 0;
		color: var(--color-text-muted);
	}

	.year-end-note.success {
		background: color-mix(in srgb, var(--color-success, #16a34a) 8%, white);
	}

	.year-end-note.warning {
		background: color-mix(in srgb, var(--color-warning, #d97706) 8%, white);
	}

	.year-end-note.muted {
		background: color-mix(in srgb, var(--color-text-muted, #64748b) 8%, white);
	}

	@media (max-width: 900px) {
		.year-end-toolbar,
		.year-end-content-grid {
			grid-template-columns: 1fr;
			display: grid;
		}

		.year-end-toolbar-actions,
		.year-end-summary-grid {
			grid-template-columns: 1fr;
			display: grid;
		}

		.year-end-summary-grid {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}
	}

	@media (max-width: 640px) {
		.year-end-summary-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
