<script lang="ts">
	import { page } from '$app/stores';
	import DocumentReviewQueuePanel from '$lib/components/DocumentReviewQueuePanel.svelte';
	import * as m from '$lib/paraglide/messages.js';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let settingsHref = $derived(tenantId ? `/settings?tenant=${tenantId}` : '/settings');
</script>

<svelte:head>
	<title>{m.documents_reviewQueueTitle()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<DocumentReviewQueuePanel {tenantId} backHref={settingsHref} />
</div>
