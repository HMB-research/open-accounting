<script lang="ts">
	import '../app.css';
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { pluginManager, type PluginNavigationItem } from '$lib/plugins';

	let { children } = $props();

	let isAuthenticated = $state(api.isAuthenticated);
	let pluginNavItems = $state<PluginNavigationItem[]>([]);

	// Load plugins when tenant changes
	$effect(() => {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId && isAuthenticated) {
			pluginManager.loadPlugins(tenantId);
		}
	});

	// Subscribe to plugin navigation changes
	$effect(() => {
		const unsubscribe = pluginManager.subscribe(() => {
			pluginNavItems = pluginManager.getNavigation();
		});
		return unsubscribe;
	});

	function handleLogout() {
		api.logout();
		pluginManager.clear();
		isAuthenticated = false;
		window.location.href = '/login';
	}

	function getPluginNavUrl(item: PluginNavigationItem): string {
		const tenantId = $page.url.searchParams.get('tenant');
		return tenantId ? `${item.path}?tenant=${tenantId}` : item.path;
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
					<div class="nav-dropdown">
						<span class="nav-dropdown-trigger">Payroll</span>
						<div class="nav-dropdown-menu">
							<a href="/employees">Employees</a>
							<a href="/payroll">Payroll Runs</a>
							<a href="/tsd">TSD Declarations</a>
						</div>
					</div>
					{#if pluginNavItems.length > 0}
						{#each pluginNavItems as navItem}
							<a href={getPluginNavUrl(navItem)} class="plugin-nav-item" title={navItem.pluginName}>
								{navItem.label}
							</a>
						{/each}
					{/if}
					<div class="nav-dropdown">
						<span class="nav-dropdown-trigger">Admin</span>
						<div class="nav-dropdown-menu">
							<a href="/admin/plugins">Plugin Marketplace</a>
							<a href="/settings">Settings</a>
						</div>
					</div>
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

	.nav-dropdown {
		position: relative;
	}

	.nav-dropdown-trigger {
		color: var(--color-text);
		font-weight: 500;
		cursor: pointer;
	}

	.nav-dropdown-trigger::after {
		content: ' \25BE';
		font-size: 0.75rem;
	}

	.nav-dropdown-menu {
		display: none;
		position: absolute;
		top: 100%;
		left: 0;
		background: var(--color-surface);
		border: 1px solid var(--color-border);
		border-radius: 4px;
		padding: 0.5rem 0;
		min-width: 160px;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
		z-index: 50;
	}

	.nav-dropdown:hover .nav-dropdown-menu {
		display: block;
	}

	.nav-dropdown-menu a {
		display: block;
		padding: 0.5rem 1rem;
		color: var(--color-text);
	}

	.nav-dropdown-menu a:hover {
		background: var(--color-bg);
	}

	.plugin-nav-item {
		position: relative;
	}

	.plugin-nav-item::before {
		content: '';
		display: inline-block;
		width: 6px;
		height: 6px;
		background: var(--color-primary);
		border-radius: 50%;
		margin-right: 0.25rem;
		opacity: 0.6;
	}

	.main-content {
		flex: 1;
		padding: 2rem 0;
	}
</style>
