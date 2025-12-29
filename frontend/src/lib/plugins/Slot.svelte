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

	let { name, props = {}, fallback }: Props = $props();

	let registrations = $state<PluginSlotRegistration[]>([]);

	// Subscribe to plugin manager changes
	$effect(() => {
		const unsubscribe = pluginManager.subscribe(() => {
			registrations = pluginManager.getSlotRegistrations(name);
		});

		return unsubscribe;
	});
</script>

{#if registrations.length > 0}
	<div class="plugin-slot" data-slot={name}>
		{#each registrations as reg}
			<div class="plugin-slot-item" data-plugin={reg.pluginName}>
				<!--
					Note: In a full implementation, this would dynamically load and render
					the plugin's Svelte component. Since Svelte doesn't support dynamic
					component loading at runtime, plugins would need to:

					1. Register their components via a build-time process, or
					2. Use web components that can be loaded dynamically, or
					3. Use iframe-based isolation for plugin UIs

					For now, this displays a placeholder indicating where plugin content
					would appear. The architecture is in place for future enhancement.
				-->
				<div class="plugin-placeholder">
					<span class="plugin-badge">{reg.pluginName}</span>
					<span class="component-name">{reg.componentName}</span>
				</div>
			</div>
		{/each}
	</div>
{:else if fallback}
	{@render fallback()}
{/if}

<style>
	.plugin-slot {
		display: contents;
	}

	.plugin-slot-item {
		display: contents;
	}

	/* Development placeholder styles - would be removed in production with real plugin components */
	.plugin-placeholder {
		padding: 1rem;
		border: 2px dashed var(--color-border, #e5e7eb);
		border-radius: 8px;
		background: var(--color-bg, #f9fafb);
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.875rem;
		color: var(--color-text-muted, #6b7280);
	}

	.plugin-badge {
		background: var(--color-primary-light, #e0e7ff);
		color: var(--color-primary, #4f46e5);
		padding: 0.125rem 0.5rem;
		border-radius: 9999px;
		font-size: 0.75rem;
		font-weight: 500;
	}

	.component-name {
		font-family: monospace;
		font-size: 0.8rem;
	}
</style>
