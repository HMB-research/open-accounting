<script lang="ts">
	import { pluginManager, type PluginSlotRegistration } from './manager';

	interface Props {
		/** The slot name to render (e.g., "dashboard.widgets", "invoice.sidebar") */
		name: string;
		/** Props to pass to slot components */
		props?: Record<string, unknown>;
		/** Fallback content if no plugins register for this slot */
		fallback?: import('svelte').Snippet;
	}

	let { name, fallback }: Props = $props();

	let registrations = $state<PluginSlotRegistration[]>([]);
	let warnedUnsupportedSlot = false;

	// Subscribe to plugin manager changes
	$effect(() => {
		const unsubscribe = pluginManager.subscribe(() => {
			registrations = pluginManager.getSlotRegistrations(name);
		});

		return unsubscribe;
	});

	$effect(() => {
		if (registrations.length === 0) {
			warnedUnsupportedSlot = false;
			return;
		}

		if (!warnedUnsupportedSlot) {
			console.warn(
				`Plugin slot "${name}" has registered content, but frontend plugin rendering is not implemented.`
			);
			warnedUnsupportedSlot = true;
		}
	});
</script>

{#if fallback}
	{@render fallback()}
{/if}
