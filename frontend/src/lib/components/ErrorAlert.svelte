<script lang="ts">
	import * as m from '$lib/paraglide/messages.js';

	interface Props {
		message: string;
		type?: 'error' | 'warning' | 'info' | 'success';
		dismissible?: boolean;
		onDismiss?: () => void;
		action?: {
			label: string;
			onClick: () => void;
		};
	}

	let {
		message,
		type = 'error',
		dismissible = true,
		onDismiss,
		action
	}: Props = $props();

	function handleDismiss() {
		onDismiss?.();
	}
</script>

{#if message}
	<div class="alert alert-{type}" role="alert" aria-live="polite">
		<div class="alert-content">
			<span class="alert-icon">
				{#if type === 'error'}
					<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" width="20" height="20">
						<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.28 7.22a.75.75 0 00-1.06 1.06L8.94 10l-1.72 1.72a.75.75 0 101.06 1.06L10 11.06l1.72 1.72a.75.75 0 101.06-1.06L11.06 10l1.72-1.72a.75.75 0 00-1.06-1.06L10 8.94 8.28 7.22z" clip-rule="evenodd" />
					</svg>
				{:else if type === 'warning'}
					<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" width="20" height="20">
						<path fill-rule="evenodd" d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 5a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 5zm0 9a1 1 0 100-2 1 1 0 000 2z" clip-rule="evenodd" />
					</svg>
				{:else if type === 'success'}
					<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" width="20" height="20">
						<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.483 4.79-1.88-1.88a.75.75 0 10-1.06 1.061l2.5 2.5a.75.75 0 001.137-.089l4-5.5z" clip-rule="evenodd" />
					</svg>
				{:else}
					<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" width="20" height="20">
						<path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a.75.75 0 000 1.5h.253a.25.25 0 01.244.304l-.459 2.066A1.75 1.75 0 0010.747 15H11a.75.75 0 000-1.5h-.253a.25.25 0 01-.244-.304l.459-2.066A1.75 1.75 0 009.253 9H9z" clip-rule="evenodd" />
					</svg>
				{/if}
			</span>
			<span class="alert-message">{message}</span>
		</div>
		<div class="alert-actions">
			{#if action}
				<button class="alert-action-btn" onclick={action.onClick}>
					{action.label}
				</button>
			{/if}
			{#if dismissible}
				<button
					class="alert-dismiss"
					onclick={handleDismiss}
					aria-label={m.common_close()}
				>
					<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" width="16" height="16">
						<path d="M6.28 5.22a.75.75 0 00-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 101.06 1.06L10 11.06l3.72 3.72a.75.75 0 101.06-1.06L11.06 10l3.72-3.72a.75.75 0 00-1.06-1.06L10 8.94 6.28 5.22z" />
					</svg>
				</button>
			{/if}
		</div>
	</div>
{/if}

<style>
	.alert {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 0.75rem;
		padding: 0.75rem 1rem;
		border-radius: 0.5rem;
		margin-bottom: 1rem;
		font-size: 0.875rem;
	}

	.alert-error {
		background-color: #fef2f2;
		border: 1px solid #fecaca;
		color: #dc2626;
	}

	.alert-warning {
		background-color: #fffbeb;
		border: 1px solid #fde68a;
		color: #d97706;
	}

	.alert-success {
		background-color: #f0fdf4;
		border: 1px solid #bbf7d0;
		color: #16a34a;
	}

	.alert-info {
		background-color: #eff6ff;
		border: 1px solid #bfdbfe;
		color: #2563eb;
	}

	.alert-content {
		display: flex;
		align-items: flex-start;
		gap: 0.5rem;
		flex: 1;
	}

	.alert-icon {
		flex-shrink: 0;
		display: flex;
		align-items: center;
	}

	.alert-message {
		flex: 1;
		line-height: 1.4;
	}

	.alert-actions {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		flex-shrink: 0;
	}

	.alert-action-btn {
		padding: 0.25rem 0.75rem;
		border-radius: 0.375rem;
		font-size: 0.75rem;
		font-weight: 500;
		cursor: pointer;
		border: 1px solid currentColor;
		background: transparent;
		color: inherit;
		transition: background-color 0.2s;
	}

	.alert-action-btn:hover {
		background-color: rgba(0, 0, 0, 0.05);
	}

	.alert-dismiss {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 0.25rem;
		border: none;
		background: transparent;
		cursor: pointer;
		color: inherit;
		opacity: 0.7;
		border-radius: 0.25rem;
		transition: opacity 0.2s, background-color 0.2s;
	}

	.alert-dismiss:hover {
		opacity: 1;
		background-color: rgba(0, 0, 0, 0.1);
	}

	/* Dark mode support */
	@media (prefers-color-scheme: dark) {
		.alert-error {
			background-color: rgba(220, 38, 38, 0.1);
			border-color: rgba(220, 38, 38, 0.3);
		}

		.alert-warning {
			background-color: rgba(217, 119, 6, 0.1);
			border-color: rgba(217, 119, 6, 0.3);
		}

		.alert-success {
			background-color: rgba(22, 163, 74, 0.1);
			border-color: rgba(22, 163, 74, 0.3);
		}

		.alert-info {
			background-color: rgba(37, 99, 235, 0.1);
			border-color: rgba(37, 99, 235, 0.3);
		}
	}
</style>
