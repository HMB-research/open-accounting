<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';

	// Keep the historical /tax URL working while the canonical VAT workflow lives
	// at /vat-returns. Preserve the tenant context used by older bookmarks.
	onMount(() => {
		const tenant = $page.url.searchParams.get('tenant');
		const target = tenant
			? `/vat-returns?tenant=${encodeURIComponent(tenant)}`
			: '/vat-returns';
		void goto(target, { replaceState: true });
	});
</script>

<svelte:head>
	<title>VAT Declarations - Open Accounting</title>
</svelte:head>

<div class="container">
	<p>Opening the canonical VAT declarations page…</p>
</div>
