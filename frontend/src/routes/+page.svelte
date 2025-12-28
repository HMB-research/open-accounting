<script lang="ts">
	import { browser } from '$app/environment';
	import { api } from '$lib/api';

	if (browser) {
		if (api.isAuthenticated) {
			window.location.href = '/dashboard';
		} else {
			window.location.href = '/login';
		}
	}
</script>

<div class="container">
	<p>Redirecting...</p>
</div>
