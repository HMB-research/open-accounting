<script lang="ts">
	import type { Snippet } from 'svelte';

	/**
	 * Reusable modal component with backdrop, accessibility support,
	 * and responsive mobile styles.
	 */

	interface Props {
		/** Whether the modal is open */
		open: boolean;
		/** Modal title displayed in the header */
		title: string;
		/** Modal size variant */
		size?: 'sm' | 'md' | 'lg' | 'xl';
		/** Called when the modal should close (backdrop click or escape) */
		onClose: () => void;
		/** Modal content */
		children: Snippet;
		/** Optional footer slot for action buttons */
		footer?: Snippet;
	}

	let { open, title, size = 'md', onClose, children, footer }: Props = $props();

	// Generate a unique ID for accessibility
	const titleId = `modal-title-${Math.random().toString(36).slice(2, 11)}`;

	function handleBackdropClick() {
		onClose();
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			onClose();
		}
	}

	function handleModalClick(event: MouseEvent) {
		event.stopPropagation();
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={handleBackdropClick} role="presentation">
		<div
			class="modal modal-{size} card"
			onclick={handleModalClick}
			role="dialog"
			aria-modal="true"
			aria-labelledby={titleId}
			tabindex="-1"
		>
			<h2 id={titleId}>{title}</h2>
			{@render children()}
			{#if footer}
				<div class="modal-footer">
					{@render footer()}
				</div>
			{/if}
		</div>
	</div>
{/if}

<style>
	.modal-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.5);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 100;
		padding: 1rem;
	}

	.modal {
		width: 100%;
		max-height: 90vh;
		overflow-y: auto;
	}

	.modal-sm {
		max-width: 400px;
	}

	.modal-md {
		max-width: 600px;
	}

	.modal-lg {
		max-width: 800px;
	}

	.modal-xl {
		max-width: 900px;
	}

	.modal h2 {
		margin-bottom: 1.5rem;
	}

	.modal-footer {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1.5rem;
	}

	/* Mobile responsive styles */
	@media (max-width: 768px) {
		.modal-backdrop {
			padding: 0;
			align-items: flex-end;
		}

		.modal {
			max-width: 100%;
			max-height: 95vh;
			border-radius: 1rem 1rem 0 0;
		}

		.modal-footer {
			flex-direction: column-reverse;
		}

		.modal-footer :global(button) {
			width: 100%;
			min-height: 44px;
		}
	}
</style>
