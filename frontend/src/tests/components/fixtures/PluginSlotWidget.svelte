<script lang="ts">
	import type { PluginSlotRegistration } from '$lib/plugins';

	interface Props {
		registration: PluginSlotRegistration;
		headline?: string;
		tenantId?: string;
	}

	let { registration, headline = 'Registered component', tenantId }: Props = $props();
</script>

<article
	data-testid="registered-plugin-component"
	data-plugin-id={registration.pluginId}
	data-tenant-id={tenantId ?? ''}
>
	<h2>{headline}</h2>
	<p>{registration.label}</p>
</article>
