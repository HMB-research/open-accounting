<script lang="ts">
	import { page } from '$app/stores';
	import MigrationWorkbench from '$lib/components/MigrationWorkbench.svelte';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let runId = $derived($page.url.searchParams.get('run_id') || '');
	// An accountant handoff intentionally carries only opaque server-bound
	// identifiers. It never contains names, source data, capabilities, or
	// evidence/tolerance digests; the mounted component validates them again.
	let accountantReviewBatchId = $derived($page.url.searchParams.get('reconciliation_batch') || '');
	let accountantReviewSourceCompanyId = $derived($page.url.searchParams.get('reconciliation_source') || '');
	// Owner continuation carries only the immutable selected/all batch and
	// opaque source binding. The component re-fetches every package/preview
	// handle from the owner-safe workflow status.
	let ownerContinuationBatchId = $derived($page.url.searchParams.get('workflow_batch') || '');
	let ownerContinuationSourceCompanyId = $derived($page.url.searchParams.get('workflow_source') || '');
</script>

<svelte:head>
	<title>Migration Workbench - Open Accounting</title>
</svelte:head>

<MigrationWorkbench {tenantId} {runId} {accountantReviewBatchId} {accountantReviewSourceCompanyId} {ownerContinuationBatchId} {ownerContinuationSourceCompanyId} />
