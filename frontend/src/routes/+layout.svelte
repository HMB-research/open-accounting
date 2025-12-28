<script lang="ts">
	import '../app.css';
	import { api } from '$lib/api';

	let { children } = $props();

	let isAuthenticated = $state(api.isAuthenticated);

	function handleLogout() {
		api.logout();
		isAuthenticated = false;
		window.location.href = '/login';
	}
</script>

<div class="app">
	{#if isAuthenticated}
		<nav class="navbar">
			<div class="container navbar-content">
				<a href="/" class="logo">Open Accounting</a>
				<div class="nav-links">
					<a href="/dashboard">Dashboard</a>
					<a href="/accounts">Accounts</a>
					<a href="/journal">Journal</a>
					<a href="/contacts">Contacts</a>
					<a href="/invoices">Invoices</a>
					<a href="/payments">Payments</a>
					<a href="/reports">Reports</a>
					<button class="btn btn-secondary" onclick={handleLogout}>Logout</button>
				</div>
			</div>
		</nav>
	{/if}

	<main class="main-content">
		{@render children()}
	</main>
</div>

<style>
	.app {
		min-height: 100vh;
		display: flex;
		flex-direction: column;
	}

	.navbar {
		background: var(--color-surface);
		border-bottom: 1px solid var(--color-border);
		padding: 1rem 0;
	}

	.navbar-content {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.logo {
		font-size: 1.25rem;
		font-weight: 700;
		color: var(--color-primary);
	}

	.logo:hover {
		text-decoration: none;
	}

	.nav-links {
		display: flex;
		align-items: center;
		gap: 1.5rem;
	}

	.nav-links a {
		color: var(--color-text);
		font-weight: 500;
	}

	.nav-links a:hover {
		color: var(--color-primary);
		text-decoration: none;
	}

	.main-content {
		flex: 1;
		padding: 2rem 0;
	}
</style>
