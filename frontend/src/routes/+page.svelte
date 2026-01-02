<script lang="ts">
	import { browser } from '$app/environment';
	import { api } from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';

	// Check if user is already authenticated
	let isAuthenticated = $state(false);

	if (browser) {
		isAuthenticated = api.isAuthenticated;
	}
</script>

<svelte:head>
	<title>Open Accounting - Modern Accounting Software</title>
	<meta name="description" content="Open-source accounting software for Estonian businesses. Manage invoices, payroll, VAT reporting, and more." />
</svelte:head>

<div class="landing">
	<!-- Hero Section -->
	<section class="hero">
		<div class="hero-content">
			<h1>Open Accounting</h1>
			<p class="tagline">Modern, open-source accounting for Estonian businesses</p>
			<p class="description">
				Manage invoices, contacts, payroll, and tax declarations with ease.
				Built for Estonian compliance including TSD reporting and e-invoicing.
			</p>
			<div class="cta-buttons">
				{#if isAuthenticated}
					<a href="/dashboard" class="btn btn-primary btn-lg">Go to Dashboard</a>
				{:else}
					<a href="/login" class="btn btn-primary btn-lg">Get Started</a>
					<a href="#features" class="btn btn-secondary btn-lg">Learn More</a>
				{/if}
			</div>
		</div>
	</section>

	<!-- Features Section -->
	<section id="features" class="features">
		<h2>Everything you need</h2>
		<div class="features-grid">
			<div class="feature-card">
				<div class="feature-icon">
					<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
						<polyline points="14 2 14 8 20 8"></polyline>
						<line x1="16" y1="13" x2="8" y2="13"></line>
						<line x1="16" y1="17" x2="8" y2="17"></line>
					</svg>
				</div>
				<h3>Invoicing</h3>
				<p>Create professional invoices, track payments, and send automated reminders. Support for recurring invoices.</p>
			</div>

			<div class="feature-card">
				<div class="feature-icon">
					<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path>
						<circle cx="9" cy="7" r="4"></circle>
						<path d="M23 21v-2a4 4 0 0 0-3-3.87"></path>
						<path d="M16 3.13a4 4 0 0 1 0 7.75"></path>
					</svg>
				</div>
				<h3>Payroll</h3>
				<p>Estonian payroll with automatic tax calculations. Social tax, income tax, pension, and unemployment insurance.</p>
			</div>

			<div class="feature-card">
				<div class="feature-icon">
					<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<rect x="1" y="4" width="22" height="16" rx="2" ry="2"></rect>
						<line x1="1" y1="10" x2="23" y2="10"></line>
					</svg>
				</div>
				<h3>Banking</h3>
				<p>Import bank statements, reconcile transactions, and keep your books in sync with your bank accounts.</p>
			</div>

			<div class="feature-card">
				<div class="feature-icon">
					<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path>
						<polyline points="7.5 4.21 12 6.81 16.5 4.21"></polyline>
						<polyline points="7.5 19.79 7.5 14.6 3 12"></polyline>
						<polyline points="21 12 16.5 14.6 16.5 19.79"></polyline>
						<polyline points="3.27 6.96 12 12.01 20.73 6.96"></polyline>
						<line x1="12" y1="22.08" x2="12" y2="12"></line>
					</svg>
				</div>
				<h3>TSD Reporting</h3>
				<p>Generate TSD declarations for Estonian Tax Board. Export XML for e-MTA submission.</p>
			</div>

			<div class="feature-card">
				<div class="feature-icon">
					<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<line x1="18" y1="20" x2="18" y2="10"></line>
						<line x1="12" y1="20" x2="12" y2="4"></line>
						<line x1="6" y1="20" x2="6" y2="14"></line>
					</svg>
				</div>
				<h3>Reports</h3>
				<p>Trial balance, balance sheet, income statement. Export to Excel, CSV, or PDF.</p>
			</div>

			<div class="feature-card">
				<div class="feature-icon">
					<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path>
					</svg>
				</div>
				<h3>Open Source</h3>
				<p>Fully open source. Self-host or use our cloud. Your data, your control.</p>
			</div>
		</div>
	</section>

	<!-- Demo Section -->
	<section class="demo-section">
		<h2>Try the Demo</h2>
		<p>Explore Open Accounting with our demo account. No signup required.</p>
		<div class="demo-credentials">
			<p><strong>Email:</strong> demo@openaccounting.io</p>
			<p><strong>Password:</strong> demo123</p>
		</div>
		<a href="/login" class="btn btn-primary btn-lg">Try Demo</a>
		<p class="demo-note">Demo resets every hour</p>
	</section>

	<!-- Footer -->
	<footer class="landing-footer">
		<div class="footer-content">
			<p>&copy; {new Date().getFullYear()} Open Accounting. Open source under MIT License.</p>
			<div class="footer-links">
				<a href="https://github.com/HMB-research/open-accounting" target="_blank" rel="noopener">GitHub</a>
			</div>
		</div>
	</footer>
</div>

<style>
	.landing {
		min-height: 100vh;
		background: var(--color-bg);
	}

	/* Hero */
	.hero {
		padding: 6rem 2rem;
		text-align: center;
		background: linear-gradient(135deg, var(--color-primary) 0%, #6366f1 100%);
		color: white;
	}

	.hero-content {
		max-width: 800px;
		margin: 0 auto;
	}

	.hero h1 {
		font-size: 3.5rem;
		font-weight: 700;
		margin-bottom: 1rem;
	}

	.tagline {
		font-size: 1.5rem;
		opacity: 0.9;
		margin-bottom: 1rem;
	}

	.description {
		font-size: 1.125rem;
		opacity: 0.85;
		margin-bottom: 2rem;
		line-height: 1.7;
	}

	.cta-buttons {
		display: flex;
		gap: 1rem;
		justify-content: center;
		flex-wrap: wrap;
	}

	.btn-lg {
		padding: 1rem 2rem;
		font-size: 1.125rem;
		min-height: 52px;
	}

	.hero .btn-primary {
		background: white;
		color: var(--color-primary);
	}

	.hero .btn-primary:hover {
		background: #f0f0f0;
	}

	.hero .btn-secondary {
		background: transparent;
		border: 2px solid white;
		color: white;
	}

	.hero .btn-secondary:hover {
		background: rgba(255, 255, 255, 0.1);
	}

	/* Features */
	.features {
		padding: 5rem 2rem;
		max-width: 1200px;
		margin: 0 auto;
	}

	.features h2 {
		text-align: center;
		font-size: 2.5rem;
		margin-bottom: 3rem;
		color: var(--color-text);
	}

	.features-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
		gap: 2rem;
	}

	.feature-card {
		background: var(--color-surface);
		padding: 2rem;
		border-radius: 1rem;
		border: 1px solid var(--color-border);
		transition: transform 0.2s, box-shadow 0.2s;
	}

	.feature-card:hover {
		transform: translateY(-4px);
		box-shadow: 0 12px 24px rgba(0, 0, 0, 0.1);
	}

	.feature-icon {
		width: 64px;
		height: 64px;
		background: var(--color-primary);
		color: white;
		border-radius: 1rem;
		display: flex;
		align-items: center;
		justify-content: center;
		margin-bottom: 1.5rem;
	}

	.feature-card h3 {
		font-size: 1.25rem;
		margin-bottom: 0.75rem;
		color: var(--color-text);
	}

	.feature-card p {
		color: var(--color-text-muted);
		line-height: 1.6;
	}

	/* Demo Section */
	.demo-section {
		padding: 5rem 2rem;
		background: var(--color-surface);
		text-align: center;
		border-top: 1px solid var(--color-border);
		border-bottom: 1px solid var(--color-border);
	}

	.demo-section h2 {
		font-size: 2rem;
		margin-bottom: 1rem;
		color: var(--color-text);
	}

	.demo-section > p {
		color: var(--color-text-muted);
		margin-bottom: 2rem;
	}

	.demo-credentials {
		background: var(--color-bg);
		padding: 1.5rem 2rem;
		border-radius: 0.75rem;
		display: inline-block;
		margin-bottom: 1.5rem;
		font-family: monospace;
		border: 1px solid var(--color-border);
	}

	.demo-credentials p {
		margin: 0.5rem 0;
		color: var(--color-text);
	}

	.demo-note {
		margin-top: 1rem;
		font-size: 0.875rem;
		color: var(--color-text-muted);
	}

	/* Footer */
	.landing-footer {
		padding: 2rem;
		text-align: center;
		color: var(--color-text-muted);
	}

	.footer-content {
		max-width: 1200px;
		margin: 0 auto;
	}

	.footer-links {
		margin-top: 1rem;
	}

	.footer-links a {
		color: var(--color-primary);
		text-decoration: none;
	}

	.footer-links a:hover {
		text-decoration: underline;
	}

	/* Mobile */
	@media (max-width: 768px) {
		.hero {
			padding: 4rem 1.5rem;
		}

		.hero h1 {
			font-size: 2.5rem;
		}

		.tagline {
			font-size: 1.25rem;
		}

		.description {
			font-size: 1rem;
		}

		.cta-buttons {
			flex-direction: column;
		}

		.cta-buttons .btn {
			width: 100%;
		}

		.features {
			padding: 3rem 1.5rem;
		}

		.features h2 {
			font-size: 2rem;
		}

		.features-grid {
			grid-template-columns: 1fr;
		}

		.demo-section {
			padding: 3rem 1.5rem;
		}
	}
</style>
