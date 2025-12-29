<script lang="ts">
	import '../app.css';
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { pluginManager, type PluginNavigationItem } from '$lib/plugins';
	import LanguageSelector from '$lib/components/LanguageSelector.svelte';
	import * as m from '$lib/paraglide/messages.js';

	let { children } = $props();

	let isAuthenticated = $state(api.isAuthenticated);
	let pluginNavItems = $state<PluginNavigationItem[]>([]);
	let mobileMenuOpen = $state(false);
	let expandedDropdown = $state<string | null>(null);

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

	// Close mobile menu on route change
	$effect(() => {
		$page.url.pathname;
		mobileMenuOpen = false;
		expandedDropdown = null;
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

	function toggleMobileMenu() {
		mobileMenuOpen = !mobileMenuOpen;
		if (!mobileMenuOpen) {
			expandedDropdown = null;
		}
	}

	function toggleDropdown(name: string) {
		expandedDropdown = expandedDropdown === name ? null : name;
	}

	function closeMobileMenu() {
		mobileMenuOpen = false;
		expandedDropdown = null;
	}
</script>

<div class="app">
	{#if isAuthenticated}
		<nav class="navbar">
			<div class="container navbar-content">
				<a href="/" class="logo">Open Accounting</a>

				<!-- Desktop Navigation -->
				<div class="nav-links hide-mobile-flex">
					<a href="/dashboard">{m.nav_dashboard()}</a>
					<a href="/accounts">{m.nav_accounts()}</a>
					<a href="/journal">{m.nav_journal()}</a>
					<a href="/contacts">{m.nav_contacts()}</a>
					<a href="/invoices">{m.nav_invoices()}</a>
					<a href="/payments">{m.nav_payments()}</a>
					<a href="/reports">{m.nav_reports()}</a>
					<div class="nav-dropdown">
						<span class="nav-dropdown-trigger">{m.nav_payroll()}</span>
						<div class="nav-dropdown-menu">
							<a href="/employees">{m.nav_employees()}</a>
							<a href="/payroll">{m.nav_payrollRuns()}</a>
							<a href="/tsd">{m.nav_tsd()}</a>
						</div>
					</div>
					{#if pluginNavItems.length > 0}
						{#each pluginNavItems as navItem (navItem.path)}
							<a href={getPluginNavUrl(navItem)} class="plugin-nav-item" title={navItem.pluginName}>
								{navItem.label}
							</a>
						{/each}
					{/if}
					<div class="nav-dropdown">
						<span class="nav-dropdown-trigger">{m.nav_admin()}</span>
						<div class="nav-dropdown-menu">
							<a href="/admin/plugins">{m.nav_plugins()}</a>
							<a href="/settings">{m.nav_settings()}</a>
						</div>
					</div>
					<LanguageSelector />
					<button class="btn btn-secondary" onclick={handleLogout}>{m.nav_logout()}</button>
				</div>

				<!-- Mobile Menu Button -->
				<button class="mobile-menu-btn show-mobile" onclick={toggleMobileMenu} aria-label="Toggle menu">
					<span class="hamburger" class:open={mobileMenuOpen}>
						<span></span>
						<span></span>
						<span></span>
					</span>
				</button>
			</div>
		</nav>

		<!-- Mobile Navigation Drawer -->
		{#if mobileMenuOpen}
			<div class="mobile-nav-backdrop" onclick={closeMobileMenu} role="presentation"></div>
			<div class="mobile-nav" class:open={mobileMenuOpen}>
				<div class="mobile-nav-header">
					<span class="mobile-nav-title">Menu</span>
					<button class="mobile-nav-close" onclick={closeMobileMenu} aria-label="Close menu">×</button>
				</div>
				<div class="mobile-nav-content">
					<a href="/dashboard" class="mobile-nav-link">{m.nav_dashboard()}</a>
					<a href="/accounts" class="mobile-nav-link">{m.nav_accounts()}</a>
					<a href="/journal" class="mobile-nav-link">{m.nav_journal()}</a>
					<a href="/contacts" class="mobile-nav-link">{m.nav_contacts()}</a>
					<a href="/invoices" class="mobile-nav-link">{m.nav_invoices()}</a>
					<a href="/payments" class="mobile-nav-link">{m.nav_payments()}</a>
					<a href="/reports" class="mobile-nav-link">{m.nav_reports()}</a>

					<!-- Payroll Accordion -->
					<div class="mobile-nav-accordion">
						<button class="mobile-nav-accordion-trigger" onclick={() => toggleDropdown('payroll')}>
							<span>{m.nav_payroll()}</span>
							<span class="accordion-arrow" class:expanded={expandedDropdown === 'payroll'}>▸</span>
						</button>
						{#if expandedDropdown === 'payroll'}
							<div class="mobile-nav-accordion-content">
								<a href="/employees" class="mobile-nav-link sub">{m.nav_employees()}</a>
								<a href="/payroll" class="mobile-nav-link sub">{m.nav_payrollRuns()}</a>
								<a href="/tsd" class="mobile-nav-link sub">{m.nav_tsd()}</a>
							</div>
						{/if}
					</div>

					{#if pluginNavItems.length > 0}
						<div class="mobile-nav-divider"></div>
						<span class="mobile-nav-section-title">{m.nav_plugins()}</span>
						{#each pluginNavItems as navItem (navItem.path)}
							<a href={getPluginNavUrl(navItem)} class="mobile-nav-link plugin" title={navItem.pluginName}>
								{navItem.label}
							</a>
						{/each}
					{/if}

					<!-- Admin Accordion -->
					<div class="mobile-nav-accordion">
						<button class="mobile-nav-accordion-trigger" onclick={() => toggleDropdown('admin')}>
							<span>{m.nav_admin()}</span>
							<span class="accordion-arrow" class:expanded={expandedDropdown === 'admin'}>▸</span>
						</button>
						{#if expandedDropdown === 'admin'}
							<div class="mobile-nav-accordion-content">
								<a href="/admin/plugins" class="mobile-nav-link sub">{m.nav_plugins()}</a>
								<a href="/settings" class="mobile-nav-link sub">{m.nav_settings()}</a>
							</div>
						{/if}
					</div>

					<div class="mobile-nav-divider"></div>
					<div class="mobile-nav-language">
						<LanguageSelector />
					</div>
					<button class="btn btn-secondary mobile-nav-logout" onclick={handleLogout}>{m.nav_logout()}</button>
				</div>
			</div>
		{/if}
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
		position: sticky;
		top: 0;
		z-index: 40;
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

	/* Mobile Menu Button */
	.mobile-menu-btn {
		display: none;
		background: none;
		border: none;
		padding: 0.5rem;
		cursor: pointer;
	}

	.hamburger {
		display: flex;
		flex-direction: column;
		justify-content: space-between;
		width: 24px;
		height: 18px;
	}

	.hamburger span {
		display: block;
		height: 2px;
		background: var(--color-text);
		border-radius: 1px;
		transition: all 0.3s ease;
	}

	.hamburger.open span:nth-child(1) {
		transform: rotate(45deg) translate(5px, 5px);
	}

	.hamburger.open span:nth-child(2) {
		opacity: 0;
	}

	.hamburger.open span:nth-child(3) {
		transform: rotate(-45deg) translate(5px, -5px);
	}

	/* Mobile Navigation */
	.mobile-nav-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.5);
		z-index: 45;
	}

	.mobile-nav {
		position: fixed;
		top: 0;
		left: 0;
		bottom: 0;
		width: 280px;
		max-width: 85vw;
		background: var(--color-surface);
		z-index: 50;
		display: flex;
		flex-direction: column;
		box-shadow: 4px 0 20px rgba(0, 0, 0, 0.15);
		animation: slideIn 0.3s ease;
	}

	@keyframes slideIn {
		from {
			transform: translateX(-100%);
		}
		to {
			transform: translateX(0);
		}
	}

	.mobile-nav-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 1rem;
		border-bottom: 1px solid var(--color-border);
	}

	.mobile-nav-title {
		font-weight: 600;
		font-size: 1.125rem;
	}

	.mobile-nav-close {
		background: none;
		border: none;
		font-size: 1.5rem;
		cursor: pointer;
		padding: 0.25rem 0.5rem;
		color: var(--color-text-muted);
	}

	.mobile-nav-close:hover {
		color: var(--color-text);
	}

	.mobile-nav-content {
		flex: 1;
		overflow-y: auto;
		padding: 0.5rem 0;
	}

	.mobile-nav-link {
		display: block;
		padding: 0.875rem 1rem;
		color: var(--color-text);
		font-weight: 500;
		text-decoration: none;
		transition: background 0.15s ease;
	}

	.mobile-nav-link:hover {
		background: var(--color-bg);
		text-decoration: none;
	}

	.mobile-nav-link.sub {
		padding-left: 2rem;
		font-weight: 400;
		color: var(--color-text-muted);
	}

	.mobile-nav-link.sub:hover {
		color: var(--color-text);
	}

	.mobile-nav-link.plugin::before {
		content: '';
		display: inline-block;
		width: 6px;
		height: 6px;
		background: var(--color-primary);
		border-radius: 50%;
		margin-right: 0.5rem;
		opacity: 0.6;
	}

	.mobile-nav-accordion {
		border-top: 1px solid var(--color-border);
	}

	.mobile-nav-accordion:first-of-type {
		border-top: none;
	}

	.mobile-nav-accordion-trigger {
		display: flex;
		align-items: center;
		justify-content: space-between;
		width: 100%;
		padding: 0.875rem 1rem;
		background: none;
		border: none;
		font-size: inherit;
		font-weight: 500;
		color: var(--color-text);
		cursor: pointer;
		text-align: left;
	}

	.mobile-nav-accordion-trigger:hover {
		background: var(--color-bg);
	}

	.accordion-arrow {
		transition: transform 0.2s ease;
		color: var(--color-text-muted);
	}

	.accordion-arrow.expanded {
		transform: rotate(90deg);
	}

	.mobile-nav-accordion-content {
		background: var(--color-bg);
	}

	.mobile-nav-divider {
		height: 1px;
		background: var(--color-border);
		margin: 0.5rem 0;
	}

	.mobile-nav-section-title {
		display: block;
		padding: 0.5rem 1rem;
		font-size: 0.75rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--color-text-muted);
	}

	.mobile-nav-language {
		padding: 0.5rem 1rem;
	}

	.mobile-nav-logout {
		margin: 1rem;
		width: calc(100% - 2rem);
	}

	/* Mobile responsive */
	@media (max-width: 768px) {
		.mobile-menu-btn {
			display: block;
		}

		.main-content {
			padding: 1rem 0;
		}
	}
</style>
