<script lang="ts">
	import FormModal from '$lib/components/FormModal.svelte';

	let {
		open = true,
		title = 'Edit record',
		size = 'md',
		onClose = () => {}
	}: {
		open?: boolean;
		title?: string;
		size?: 'sm' | 'md' | 'lg' | 'xl';
		onClose?: () => void;
	} = $props();
</script>

<FormModal {open} {title} {size} {onClose}>
	<p data-testid="modal-body">Modal body content</p>

	{#snippet footer()}
		<button type="button">Footer Action</button>
	{/snippet}
</FormModal>
