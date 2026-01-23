<script lang="ts">
	/**
	 * Generic status badge component with configurable styling.
	 * Used across invoices, quotes, orders and other status displays.
	 */

	export interface StatusConfig {
		/** CSS class to apply for this status */
		class: string;
		/** Display label for this status */
		label: string;
	}

	interface Props {
		/** Current status value */
		status: string;
		/** Configuration mapping status values to styles and labels */
		config: Record<string, StatusConfig>;
		/** Badge size */
		size?: 'sm' | 'md' | 'lg';
	}

	let { status, config, size = 'md' }: Props = $props();

	// Get the config for the current status, or a default if not found
	let statusConfig = $derived(config[status] || { class: 'badge-default', label: status });
</script>

<span class="badge badge-{size} {statusConfig.class}">
	{statusConfig.label}
</span>

<style>
	.badge {
		display: inline-flex;
		align-items: center;
		font-weight: 500;
		border-radius: 9999px;
		white-space: nowrap;
	}

	/* Sizes */
	.badge-sm {
		padding: 0.125rem 0.5rem;
		font-size: 0.625rem;
	}

	.badge-md {
		padding: 0.25rem 0.75rem;
		font-size: 0.75rem;
	}

	.badge-lg {
		padding: 0.375rem 1rem;
		font-size: 0.875rem;
	}

	/* Default fallback */
	.badge-default {
		background: #e5e7eb;
		color: #374151;
	}

	/* Invoice statuses */
	:global(.badge-draft) {
		background: #f3f4f6;
		color: #6b7280;
	}

	:global(.badge-sent) {
		background: #dbeafe;
		color: #1d4ed8;
	}

	:global(.badge-partial) {
		background: #fef3c7;
		color: #92400e;
	}

	:global(.badge-paid) {
		background: #dcfce7;
		color: #166534;
	}

	:global(.badge-overdue) {
		background: #fee2e2;
		color: #dc2626;
	}

	:global(.badge-voided) {
		background: #f3f4f6;
		color: #9ca3af;
	}

	/* Quote statuses */
	:global(.badge-accepted) {
		background: #dcfce7;
		color: #166534;
	}

	:global(.badge-rejected) {
		background: #fee2e2;
		color: #dc2626;
	}

	:global(.badge-expired) {
		background: #fef3c7;
		color: #b45309;
	}

	:global(.badge-converted) {
		background: #f3e8ff;
		color: #7c3aed;
	}

	/* Order statuses */
	:global(.badge-pending) {
		background: #fef3c7;
		color: #b45309;
	}

	:global(.badge-confirmed) {
		background: #dbeafe;
		color: #1d4ed8;
	}

	:global(.badge-processing) {
		background: #f3e8ff;
		color: #7c3aed;
	}

	:global(.badge-shipped) {
		background: #e0e7ff;
		color: #4338ca;
	}

	:global(.badge-delivered) {
		background: #dcfce7;
		color: #166534;
	}

	:global(.badge-cancelled) {
		background: #fee2e2;
		color: #dc2626;
	}

	/* Submission statuses */
	:global(.badge-submitted) {
		background: #dbeafe;
		color: #1e40af;
	}

	/* Banking statuses */
	:global(.badge-unmatched) {
		background: #fef3c7;
		color: #92400e;
	}

	:global(.badge-matched) {
		background: #dcfce7;
		color: #166534;
	}

	:global(.badge-reconciled) {
		background: #dbeafe;
		color: #1e40af;
	}

	/* Employment types */
	:global(.badge-fulltime) {
		background: #dcfce7;
		color: #166534;
	}

	:global(.badge-parttime) {
		background: #fef3c7;
		color: #92400e;
	}

	:global(.badge-contract) {
		background: #e0e7ff;
		color: #3730a3;
	}
</style>
