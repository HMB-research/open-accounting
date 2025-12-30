<script lang="ts">
	import { browser } from '$app/environment';
	import { api } from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';

	if (browser) {
		if (api.isAuthenticated) {
			window.location.href = '/dashboard';
		} else {
			window.location.href = '/login';
		}
	}
</script>

<div class="container">
	<p>{m.common_redirecting()}</p>
</div>
