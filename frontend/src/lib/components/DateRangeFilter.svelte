<script lang="ts">
	/**
	 * Date range filter component with preset options and custom date inputs.
	 * Supports common presets like "Last 7 Days", "This Month", "This Year" etc.
	 *
	 * @example
	 * ```svelte
	 * <DateRangeFilter
	 *   bind:fromDate
	 *   bind:toDate
	 *   onchange={(from, to) => loadData(from, to)}
	 * />
	 * ```
	 */
	import * as m from '$lib/paraglide/messages.js';
	import { type DatePreset, calculateDateRange, getTodayISO } from '$lib/utils/dates.js';

	/**
	 * Props for DateRangeFilter component
	 */
	interface Props {
		/** Start date in ISO format (YYYY-MM-DD), bindable */
		fromDate?: string;
		/** End date in ISO format (YYYY-MM-DD), bindable */
		toDate?: string;
		/** Callback when dates change */
		onchange?: (from: string, to: string) => void;
		/** Show preset dropdown (Today, Last 7 Days, etc.) */
		showPresets?: boolean;
		/** Use compact styling for smaller layouts */
		compact?: boolean;
	}

	let {
		fromDate = $bindable(''),
		toDate = $bindable(''),
		onchange,
		showPresets = true,
		compact = false
	}: Props = $props();

	let selectedPreset = $state<DatePreset | 'CUSTOM'>('ALL_TIME');

	const presets: { value: DatePreset | 'CUSTOM'; label: () => string }[] = [
		{ value: 'ALL_TIME', label: () => m.filter_allTime() },
		{ value: 'TODAY', label: () => m.filter_today() },
		{ value: 'LAST_7_DAYS', label: () => m.filter_last7Days() },
		{ value: 'LAST_30_DAYS', label: () => m.filter_last30Days() },
		{ value: 'THIS_MONTH', label: () => m.dashboard_thisMonth() },
		{ value: 'THIS_QUARTER', label: () => m.dashboard_thisQuarter() },
		{ value: 'THIS_YEAR', label: () => m.dashboard_thisYear() },
		{ value: 'CUSTOM', label: () => m.dashboard_custom() }
	];

	function handlePresetChange(preset: DatePreset | 'CUSTOM') {
		selectedPreset = preset;

		if (preset === 'CUSTOM') {
			// Don't change dates, let user input them
			if (!fromDate) fromDate = getTodayISO();
			if (!toDate) toDate = getTodayISO();
		} else if (preset === 'ALL_TIME') {
			fromDate = '';
			toDate = '';
		} else {
			const range = calculateDateRange(preset);
			fromDate = range.from;
			toDate = range.to;
		}

		onchange?.(fromDate, toDate);
	}

	function handleDateChange() {
		selectedPreset = 'CUSTOM';
		onchange?.(fromDate, toDate);
	}

	function clearDates() {
		fromDate = '';
		toDate = '';
		selectedPreset = 'ALL_TIME';
		onchange?.('', '');
	}

	// Determine preset based on current dates
	// Sync preset with empty dates (but don't assign state in effect for other cases)
	$effect(() => {
		if (!fromDate && !toDate && selectedPreset !== 'ALL_TIME') {
			selectedPreset = 'ALL_TIME';
		}
	});
</script>

<div class="date-range-filter" class:compact data-testid="date-range-filter">
	{#if showPresets}
		<select
			bind:value={selectedPreset}
			onchange={(e) => handlePresetChange(e.currentTarget.value as DatePreset | 'CUSTOM')}
			class="preset-select"
			data-testid="preset-select"
		>
			{#each presets as preset (preset.value)}
				<option value={preset.value}>{preset.label()}</option>
			{/each}
		</select>
	{/if}

	<div class="date-inputs" class:show={selectedPreset === 'CUSTOM' || !showPresets}>
		<div class="date-field">
			<label for="from-date">{m.common_from()}</label>
			<input
				id="from-date"
				type="date"
				bind:value={fromDate}
				onchange={handleDateChange}
				class="date-input"
				data-testid="from-date"
			/>
		</div>
		<div class="date-field">
			<label for="to-date">{m.common_to()}</label>
			<input
				id="to-date"
				type="date"
				bind:value={toDate}
				onchange={handleDateChange}
				class="date-input"
				data-testid="to-date"
			/>
		</div>
		{#if fromDate || toDate}
			<button
				type="button"
				class="clear-btn"
				onclick={clearDates}
				title={m.filter_clearDates()}
				aria-label={m.filter_clearDates()}
			>
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="16"
					height="16"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
				>
					<line x1="18" y1="6" x2="6" y2="18"></line>
					<line x1="6" y1="6" x2="18" y2="18"></line>
				</svg>
			</button>
		{/if}
	</div>
</div>

<style>
	.date-range-filter {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		flex-wrap: wrap;
	}

	.date-range-filter.compact {
		gap: 0.5rem;
	}

	.preset-select {
		padding: 0.5rem 0.75rem;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		background: var(--color-card);
		font-size: 0.875rem;
		min-height: 38px;
		cursor: pointer;
	}

	.compact .preset-select {
		padding: 0.375rem 0.5rem;
		font-size: 0.8125rem;
		min-height: 32px;
	}

	.preset-select:focus {
		outline: 2px solid var(--color-primary);
		outline-offset: 2px;
	}

	.date-inputs {
		display: none;
		align-items: center;
		gap: 0.5rem;
	}

	.date-inputs.show {
		display: flex;
	}

	.date-field {
		display: flex;
		align-items: center;
		gap: 0.375rem;
	}

	.date-field label {
		font-size: 0.75rem;
		color: var(--color-text-muted);
		white-space: nowrap;
	}

	.date-input {
		padding: 0.5rem;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		font-size: 0.875rem;
		min-height: 38px;
		background: var(--color-card);
	}

	.compact .date-input {
		padding: 0.375rem;
		font-size: 0.8125rem;
		min-height: 32px;
	}

	.date-input:focus {
		outline: 2px solid var(--color-primary);
		outline-offset: 2px;
	}

	.clear-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 0.5rem;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		background: var(--color-card);
		cursor: pointer;
		color: var(--color-text-muted);
		min-height: 38px;
		min-width: 38px;
	}

	.compact .clear-btn {
		padding: 0.375rem;
		min-height: 32px;
		min-width: 32px;
	}

	.clear-btn:hover {
		background: var(--color-bg-hover);
		color: var(--color-danger);
	}

	@media (max-width: 640px) {
		.date-range-filter {
			flex-direction: column;
			align-items: stretch;
		}

		.preset-select {
			width: 100%;
		}

		.date-inputs {
			flex-direction: column;
			gap: 0.5rem;
		}

		.date-inputs.show {
			display: flex;
		}

		.date-field {
			width: 100%;
		}

		.date-input {
			flex: 1;
		}

		.clear-btn {
			width: 100%;
		}
	}
</style>
