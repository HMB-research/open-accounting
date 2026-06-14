<script lang="ts">
	import { pluginManager, type PluginSlotRegistration } from './manager';
	import { resolvePluginFrontendComponent } from './componentRegistry';

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

{#snippet declarativeSlotItem(registration: PluginSlotRegistration)}
	{#if registration.path}
		<a
			class={`plugin-slot-item plugin-slot-item-${registration.kind}`}
			href={registration.path}
			data-plugin-id={registration.pluginId}
			data-plugin-name={registration.pluginName}
		>
			<span class="plugin-slot-text">
				<span class="plugin-slot-title">{registration.label}</span>
				{#if registration.description}
					<span class="plugin-slot-description">{registration.description}</span>
				{/if}
			</span>
			{#if registration.badge}
				<span class="plugin-slot-badge">{registration.badge}</span>
			{/if}
		</a>
	{:else}
		<section
			class={`plugin-slot-item plugin-slot-item-${registration.kind}`}
			data-plugin-id={registration.pluginId}
			data-plugin-name={registration.pluginName}
		>
			<span class="plugin-slot-text">
				<span class="plugin-slot-title">{registration.label}</span>
				{#if registration.description}
					<span class="plugin-slot-description">{registration.description}</span>
				{/if}
			</span>
			{#if registration.badge}
				<span class="plugin-slot-badge">{registration.badge}</span>
			{/if}
		</section>
	{/if}
{/snippet}

{#if registrations.length > 0}
	<div class="plugin-slot" data-slot={name}>
		{#each registrations as registration (registration.pluginId + registration.componentName)}
			{@const PluginComponent = resolvePluginFrontendComponent(registration)}
			{#if PluginComponent}
				<svelte:boundary>
					<PluginComponent {...props} {registration} />

					{#snippet failed()}
						{@render declarativeSlotItem(registration)}
					{/snippet}
				</svelte:boundary>
			{:else}
				{@render declarativeSlotItem(registration)}
			{/if}
		{/each}
	</div>
{:else if fallback}
	{@render fallback()}
{/if}

<style>
	.plugin-slot {
		display: grid;
		gap: 0.75rem;
	}

	.plugin-slot-item {
		align-items: center;
		background: var(--color-surface, #ffffff);
		border: 1px solid var(--color-border, #d8dee4);
		border-radius: 0.5rem;
		color: inherit;
		display: flex;
		gap: 0.75rem;
		justify-content: space-between;
		min-height: 3rem;
		padding: 0.75rem;
		text-decoration: none;
	}

	.plugin-slot-item-link,
	.plugin-slot-item-action {
		cursor: pointer;
	}

	.plugin-slot-item-link:hover,
	.plugin-slot-item-action:hover {
		border-color: var(--color-primary, #2563eb);
	}

	.plugin-slot-text {
		display: grid;
		gap: 0.25rem;
		min-width: 0;
	}

	.plugin-slot-title,
	.plugin-slot-description {
		overflow-wrap: anywhere;
	}

	.plugin-slot-title {
		font-weight: 600;
	}

	.plugin-slot-description {
		color: var(--color-muted, #5f6b7a);
		font-size: 0.875rem;
	}

	.plugin-slot-badge {
		background: var(--color-accent-subtle, #eef2ff);
		border-radius: 999px;
		color: var(--color-accent, #3730a3);
		flex: 0 0 auto;
		font-size: 0.75rem;
		font-weight: 600;
		line-height: 1;
		max-width: 12rem;
		overflow-wrap: anywhere;
		padding: 0.375rem 0.5rem;
	}
</style>
