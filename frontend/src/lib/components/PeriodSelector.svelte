<script lang="ts">
	/**
	 * Period selector component for choosing fiscal periods.
	 * Used in reports and dashboards for filtering by month, quarter, or year.
	 * Automatically calculates date ranges for standard periods.
	 *
	 * @example
	 * ```svelte
	 * <PeriodSelector
	 *   bind:value={period}
	 *   bind:startDate
	 *   bind:endDate
	 *   onchange={(p, s, e) => loadReport(s, e)}
	 * />
	 * ```
	 */
	import * as m from '$lib/paraglide/messages.js';
	import { calculateDateRange } from '$lib/utils/dates';

	/** Available period types */
	type Period = 'THIS_MONTH' | 'LAST_MONTH' | 'THIS_QUARTER' | 'THIS_YEAR' | 'CUSTOM';

	/**
	 * Props for PeriodSelector component
	 */
	interface Props {
		/** Selected period type, bindable */
		value?: Period;
		/** Start date in ISO format (YYYY-MM-DD), bindable */
		startDate?: string;
		/** End date in ISO format (YYYY-MM-DD), bindable */
		endDate?: string;
		/** Callback when period or dates change */
		onchange?: (period: Period, start: string, end: string) => void;
	}

	let {
		value = $bindable('THIS_MONTH'),
		startDate = $bindable(''),
		endDate = $bindable(''),
		onchange
	}: Props = $props();

	let showCustom = $derived(value === 'CUSTOM');

	function calculateDates(period: Period): { start: string; end: string } {
		if (period === 'CUSTOM') {
			return { start: startDate, end: endDate };
		}

		const range = calculateDateRange(period);
		return { start: range.from, end: range.to };
	}

	function handlePeriodChange(newPeriod: Period) {
		value = newPeriod;
		if (newPeriod !== 'CUSTOM') {
			const dates = calculateDates(newPeriod);
			startDate = dates.start;
			endDate = dates.end;
		}
		onchange?.(value, startDate, endDate);
	}

	function handleDateChange(field: 'start' | 'end', dateValue: string) {
		if (field === 'start') {
			startDate = dateValue;
		} else {
			endDate = dateValue;
		}
		onchange?.(value, startDate, endDate);
	}

	// Initialize dates on mount
	$effect(() => {
		if (!startDate || !endDate) {
			const dates = calculateDates(value);
			startDate = dates.start;
			endDate = dates.end;
		}
	});
</script>

<div class="period-selector" data-testid="period-selector">
	<select
		bind:value
		onchange={(e) => handlePeriodChange(e.currentTarget.value as Period)}
		class="period-select"
		data-testid="period-select"
	>
		<option value="THIS_MONTH">{m.dashboard_thisMonth()}</option>
		<option value="LAST_MONTH">{m.dashboard_lastMonth()}</option>
		<option value="THIS_QUARTER">{m.dashboard_thisQuarter()}</option>
		<option value="THIS_YEAR">{m.dashboard_thisYear()}</option>
		<option value="CUSTOM">{m.dashboard_custom()}</option>
	</select>

	{#if showCustom}
		<div class="custom-dates" data-testid="custom-dates">
			<input
				type="date"
				bind:value={startDate}
				onchange={(e) => handleDateChange('start', e.currentTarget.value)}
				class="date-input"
				data-testid="date-start"
			/>
			<span class="date-separator">—</span>
			<input
				type="date"
				bind:value={endDate}
				onchange={(e) => handleDateChange('end', e.currentTarget.value)}
				class="date-input"
				data-testid="date-end"
			/>
		</div>
	{/if}
</div>

<style>
	.period-selector {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		flex-wrap: wrap;
	}

	.period-select {
		padding: 0.5rem 1rem;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		background: var(--color-card);
		font-size: 0.875rem;
		min-height: 38px;
		cursor: pointer;
	}

	.period-select:focus {
		outline: 2px solid var(--color-primary);
		outline-offset: 2px;
	}

	.custom-dates {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.date-input {
		padding: 0.5rem;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		font-size: 0.875rem;
		min-height: 38px;
		background: var(--color-card);
	}

	.date-input:focus {
		outline: 2px solid var(--color-primary);
		outline-offset: 2px;
	}

	.date-separator {
		color: var(--color-text-muted);
	}

	@media (max-width: 480px) {
		.period-selector {
			flex-direction: column;
			align-items: stretch;
		}

		.period-select {
			width: 100%;
		}

		.custom-dates {
			flex-direction: column;
			gap: 0.5rem;
		}

		.date-input {
			width: 100%;
		}

		.date-separator {
			display: none;
		}
	}
</style>
