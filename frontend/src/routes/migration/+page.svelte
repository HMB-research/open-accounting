<script lang="ts">
	import { page } from '$app/stores';
	import MigrationWorkbench from '$lib/components/MigrationWorkbench.svelte';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let runId = $derived($page.url.searchParams.get('run_id') || '');
</script>

<svelte:head>
	<title>Migration Workbench - Open Accounting</title>
</svelte:head>

<MigrationWorkbench {tenantId} {runId} />
