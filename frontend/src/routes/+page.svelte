<script lang="ts">
	import { browser } from '$app/environment';
	import { api } from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';

	let isAuthenticated = $state(false);
	let email = $state('');
	let waitlistSubmitted = $state(false);
	let waitlistError = $state('');

	if (browser) {
		isAuthenticated = api.isAuthenticated;
	}

	async function submitWaitlist(e: Event) {
		e.preventDefault();
		waitlistError = '';
		
		if (!email || !email.includes('@')) {
			waitlistError = 'Please enter a valid email';
			return;
		}

		// For now, just show success - backend integration can come later
		waitlistSubmitted = true;
		
		// Store in localStorage for now
		if (browser) {
			const waitlist = JSON.parse(localStorage.getItem('tallion_waitlist') || '[]');
			waitlist.push({ email, timestamp: new Date().toISOString() });
			localStorage.setItem('tallion_waitlist', JSON.stringify(waitlist));
		}
	}
</script>

<svelte:head>
	<title>{m.landing_title()}</title>
	<meta name="description" content={m.landing_metaDescription()} />
	<meta property="og:title" content={m.landing_title()} />
	<meta property="og:description" content={m.landing_metaDescription()} />
	<meta property="og:type" content="website" />
	<link rel="canonical" href="https://tallion.app" />
</svelte:head>

<div class="landing">
	<!-- Navigation -->
	<nav class="landing-nav">
		<div class="nav-container">
			<a href="/" class="brand">
				<span class="brand-icon">📊</span>
				<span class="brand-name">Tallion</span>
			</a>
			<div class="nav-links">
				<a href="#features">{m.landing_navFeatures()}</a>
				<a href="#pricing">{m.landing_navPricing()}</a>
				{#if isAuthenticated}
					<a href="/dashboard" class="btn btn-primary">{m.landing_goDashboard()}</a>
				{:else}
					<a href="/login" class="btn btn-outline">{m.landing_signIn()}</a>
				{/if}
			</div>
		</div>
	</nav>

	<!-- Hero Section -->
	<section class="hero">
		<div class="hero-content">
			<div class="hero-badge">{m.landing_heroBadge()}</div>
			<h1>{m.landing_heroTitle()}</h1>
			<p class="hero-subtitle">{m.landing_heroSubtitle()}</p>
			
			<div class="hero-features">
				<div class="hero-feature">
					<span class="check">✓</span>
					<span>{m.landing_heroFeature1()}</span>
				</div>
				<div class="hero-feature">
					<span class="check">✓</span>
					<span>{m.landing_heroFeature2()}</span>
				</div>
				<div class="hero-feature">
					<span class="check">✓</span>
					<span>{m.landing_heroFeature3()}</span>
				</div>
			</div>

			<div class="waitlist-form">
				{#if waitlistSubmitted}
					<div class="waitlist-success">
						<span class="success-icon">🎉</span>
						<p>{m.landing_waitlistSuccess()}</p>
					</div>
				{:else}
					<form onsubmit={submitWaitlist}>
						<input 
							type="email" 
							bind:value={email}
							placeholder={m.landing_emailPlaceholder()}
							class="waitlist-input"
						/>
						<button type="submit" class="btn btn-primary btn-lg">
							{m.landing_joinWaitlist()}
						</button>
					</form>
					{#if waitlistError}
						<p class="waitlist-error">{waitlistError}</p>
					{/if}
					<p class="waitlist-note">{m.landing_waitlistNote()}</p>
				{/if}
			</div>
		</div>
		
		<div class="hero-visual">
			<div class="dashboard-preview">
				<div class="preview-header">
					<div class="preview-dots">
						<span></span><span></span><span></span>
					</div>
					<span class="preview-title">Tallion Dashboard</span>
				</div>
				<div class="preview-content">
					<div class="preview-stat">
						<span class="stat-label">{m.landing_previewRevenue()}</span>
						<span class="stat-value">€24,580</span>
						<span class="stat-change positive">+12%</span>
					</div>
					<div class="preview-stat">
						<span class="stat-label">{m.landing_previewExpenses()}</span>
						<span class="stat-value">€8,240</span>
						<span class="stat-change">-3%</span>
					</div>
					<div class="preview-stat">
						<span class="stat-label">{m.landing_previewProfit()}</span>
						<span class="stat-value">€16,340</span>
						<span class="stat-change positive">+18%</span>
					</div>
				</div>
			</div>
		</div>
	</section>

	<!-- Social Proof -->
	<section class="social-proof">
		<p>{m.landing_targetAudience()}</p>
	</section>

	<!-- Features Section -->
	<section id="features" class="features">
		<div class="section-header">
			<h2>{m.landing_featuresTitle()}</h2>
			<p>{m.landing_featuresSubtitle()}</p>
		</div>
		
		<div class="features-grid">
			<div class="feature-card">
				<div class="feature-icon">📄</div>
				<h3>{m.landing_featureInvoicing()}</h3>
				<p>{m.landing_featureInvoicingDesc()}</p>
			</div>

			<div class="feature-card">
				<div class="feature-icon">💰</div>
				<h3>{m.landing_featureExpenses()}</h3>
				<p>{m.landing_featureExpensesDesc()}</p>
			</div>

			<div class="feature-card">
				<div class="feature-icon">📊</div>
				<h3>{m.landing_featureReports()}</h3>
				<p>{m.landing_featureReportsDesc()}</p>
			</div>

			<div class="feature-card">
				<div class="feature-icon">🏦</div>
				<h3>{m.landing_featureBanking()}</h3>
				<p>{m.landing_featureBankingDesc()}</p>
			</div>

			<div class="feature-card">
				<div class="feature-icon">📑</div>
				<h3>{m.landing_featureTax()}</h3>
				<p>{m.landing_featureTaxDesc()}</p>
			</div>

			<div class="feature-card coming-soon">
				<div class="feature-badge">{m.landing_comingSoon()}</div>
				<div class="feature-icon">📧</div>
				<h3>{m.landing_featureEInvoice()}</h3>
				<p>{m.landing_featureEInvoiceDesc()}</p>
			</div>
		</div>
	</section>

	<!-- Why Tallion -->
	<section class="why-section">
		<div class="why-content">
			<h2>{m.landing_whyTitle()}</h2>
			<div class="why-grid">
				<div class="why-item">
					<div class="why-icon">🇪🇪</div>
					<h3>{m.landing_whyEstonian()}</h3>
					<p>{m.landing_whyEstonianDesc()}</p>
				</div>
				<div class="why-item">
					<div class="why-icon">💡</div>
					<h3>{m.landing_whySimple()}</h3>
					<p>{m.landing_whySimpleDesc()}</p>
				</div>
				<div class="why-item">
					<div class="why-icon">🔓</div>
					<h3>{m.landing_whyOpen()}</h3>
					<p>{m.landing_whyOpenDesc()}</p>
				</div>
				<div class="why-item">
					<div class="why-icon">💸</div>
					<h3>{m.landing_whyAffordable()}</h3>
					<p>{m.landing_whyAffordableDesc()}</p>
				</div>
			</div>
		</div>
	</section>

	<!-- Pricing Section -->
	<section id="pricing" class="pricing">
		<div class="section-header">
			<h2>{m.landing_pricingTitle()}</h2>
			<p>{m.landing_pricingSubtitle()}</p>
		</div>
		
		<div class="pricing-grid">
			<div class="pricing-card">
				<div class="pricing-header">
					<h3>{m.landing_pricingFree()}</h3>
					<div class="price">
						<span class="price-amount">€0</span>
						<span class="price-period">/{m.landing_pricingMonth()}</span>
					</div>
				</div>
				<ul class="pricing-features">
					<li><span class="check">✓</span> {m.landing_pricingFeature1Free()}</li>
					<li><span class="check">✓</span> {m.landing_pricingFeature2Free()}</li>
					<li><span class="check">✓</span> {m.landing_pricingFeature3Free()}</li>
					<li><span class="check">✓</span> {m.landing_pricingFeature4Free()}</li>
				</ul>
				<button type="button" class="btn btn-outline btn-block" onclick={() => (document.querySelector('.hero .waitlist-input') as HTMLInputElement)?.focus()}>{m.landing_joinWaitlist()}</button>
			</div>

			<div class="pricing-card featured">
				<div class="pricing-badge">{m.landing_pricingPopular()}</div>
				<div class="pricing-header">
					<h3>{m.landing_pricingPro()}</h3>
					<div class="price">
						<span class="price-amount">€19</span>
						<span class="price-period">/{m.landing_pricingMonth()}</span>
					</div>
				</div>
				<ul class="pricing-features">
					<li><span class="check">✓</span> {m.landing_pricingFeature1Pro()}</li>
					<li><span class="check">✓</span> {m.landing_pricingFeature2Pro()}</li>
					<li><span class="check">✓</span> {m.landing_pricingFeature3Pro()}</li>
					<li><span class="check">✓</span> {m.landing_pricingFeature4Pro()}</li>
					<li><span class="check">✓</span> {m.landing_pricingFeature5Pro()}</li>
				</ul>
				<button type="button" class="btn btn-primary btn-block" onclick={() => (document.querySelector('.hero .waitlist-input') as HTMLInputElement)?.focus()}>{m.landing_joinWaitlist()}</button>
			</div>

			<div class="pricing-card">
				<div class="pricing-header">
					<h3>{m.landing_pricingBusiness()}</h3>
					<div class="price">
						<span class="price-amount">€49</span>
						<span class="price-period">/{m.landing_pricingMonth()}</span>
					</div>
				</div>
				<ul class="pricing-features">
					<li><span class="check">✓</span> {m.landing_pricingFeature1Biz()}</li>
					<li><span class="check">✓</span> {m.landing_pricingFeature2Biz()}</li>
					<li><span class="check">✓</span> {m.landing_pricingFeature3Biz()}</li>
					<li><span class="check">✓</span> {m.landing_pricingFeature4Biz()}</li>
					<li><span class="check">✓</span> {m.landing_pricingFeature5Biz()}</li>
				</ul>
				<button type="button" class="btn btn-outline btn-block" onclick={() => (document.querySelector('.hero .waitlist-input') as HTMLInputElement)?.focus()}>{m.landing_joinWaitlist()}</button>
			</div>
		</div>
	</section>

	<!-- Final CTA -->
	<section class="final-cta">
		<h2>{m.landing_ctaTitle()}</h2>
		<p>{m.landing_ctaSubtitle()}</p>
		<div class="cta-form">
			{#if waitlistSubmitted}
				<div class="waitlist-success light">
					<span class="success-icon">🎉</span>
					<p>{m.landing_waitlistSuccess()}</p>
				</div>
			{:else}
				<form onsubmit={submitWaitlist}>
					<input 
						type="email" 
						bind:value={email}
						placeholder={m.landing_emailPlaceholder()}
						class="waitlist-input"
					/>
					<button type="submit" class="btn btn-white btn-lg">
						{m.landing_joinWaitlist()}
					</button>
				</form>
			{/if}
		</div>
	</section>

	<!-- Footer -->
	<footer class="landing-footer">
		<div class="footer-content">
			<div class="footer-brand">
				<span class="brand-icon">📊</span>
				<span class="brand-name">Tallion</span>
			</div>
			<div class="footer-links">
				<a href="https://github.com/HMB-research/open-accounting" target="_blank" rel="noopener">GitHub</a>
				<span class="separator">•</span>
				<a href="mailto:info@tallion.app">info@tallion.app</a>
			</div>
			<p class="copyright">&copy; {new Date().getFullYear()} Tallion. {m.landing_footerLicense()}</p>
		</div>
	</footer>
</div>

<style>
	.landing {
		min-height: 100vh;
		background: var(--color-bg);
	}

	/* Navigation */
	.landing-nav {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		z-index: 100;
		background: rgba(255, 255, 255, 0.95);
		backdrop-filter: blur(10px);
		border-bottom: 1px solid var(--color-border);
	}

	.nav-container {
		max-width: 1200px;
		margin: 0 auto;
		padding: 1rem 2rem;
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	.brand {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 1.5rem;
		font-weight: 700;
		color: var(--color-text);
		text-decoration: none;
	}

	.brand-icon {
		font-size: 1.75rem;
	}

	.nav-links {
		display: flex;
		align-items: center;
		gap: 2rem;
	}

	.nav-links a {
		color: var(--color-text-muted);
		text-decoration: none;
		font-weight: 500;
		transition: color 0.2s;
	}

	.nav-links a:hover {
		color: var(--color-primary);
		text-decoration: none;
	}

	.btn-outline {
		background: transparent;
		border: 2px solid var(--color-primary);
		color: var(--color-primary);
		padding: 0.5rem 1.25rem;
		border-radius: 0.5rem;
		font-weight: 600;
	}

	.btn-outline:hover {
		background: var(--color-primary);
		color: white;
	}

	/* Hero */
	.hero {
		padding: 8rem 2rem 4rem;
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 4rem;
		max-width: 1200px;
		margin: 0 auto;
		align-items: center;
	}

	.hero-badge {
		display: inline-block;
		background: linear-gradient(135deg, #dbeafe 0%, #e0e7ff 100%);
		color: var(--color-primary);
		padding: 0.5rem 1rem;
		border-radius: 2rem;
		font-size: 0.875rem;
		font-weight: 600;
		margin-bottom: 1.5rem;
	}

	.hero h1 {
		font-size: 3.5rem;
		font-weight: 800;
		line-height: 1.1;
		margin-bottom: 1.5rem;
		background: linear-gradient(135deg, var(--color-text) 0%, var(--color-primary) 100%);
		-webkit-background-clip: text;
		-webkit-text-fill-color: transparent;
		background-clip: text;
	}

	.hero-subtitle {
		font-size: 1.25rem;
		color: var(--color-text-muted);
		line-height: 1.7;
		margin-bottom: 2rem;
	}

	.hero-features {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		margin-bottom: 2rem;
	}

	.hero-feature {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		color: var(--color-text);
	}

	.check {
		color: var(--color-success);
		font-weight: bold;
	}

	/* Waitlist Form */
	.waitlist-form form {
		display: flex;
		gap: 0.75rem;
		margin-bottom: 0.75rem;
	}

	.waitlist-input {
		flex: 1;
		padding: 1rem 1.25rem;
		border: 2px solid var(--color-border);
		border-radius: 0.5rem;
		font-size: 1rem;
		transition: border-color 0.2s;
	}

	.waitlist-input:focus {
		outline: none;
		border-color: var(--color-primary);
	}

	.btn-lg {
		padding: 1rem 2rem;
		font-size: 1rem;
	}

	.waitlist-note {
		font-size: 0.875rem;
		color: var(--color-text-muted);
	}

	.waitlist-success {
		display: flex;
		align-items: center;
		gap: 1rem;
		padding: 1.5rem;
		background: #dcfce7;
		border-radius: 0.75rem;
		color: #166534;
	}

	.waitlist-success.light {
		background: rgba(255, 255, 255, 0.2);
		color: white;
	}

	.success-icon {
		font-size: 2rem;
	}

	.waitlist-error {
		color: var(--color-error);
		font-size: 0.875rem;
		margin-top: 0.5rem;
	}

	/* Dashboard Preview */
	.hero-visual {
		display: flex;
		justify-content: center;
	}

	.dashboard-preview {
		background: white;
		border-radius: 1rem;
		box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.15);
		overflow: hidden;
		width: 100%;
		max-width: 400px;
		border: 1px solid var(--color-border);
	}

	.preview-header {
		background: var(--color-bg);
		padding: 0.75rem 1rem;
		display: flex;
		align-items: center;
		gap: 1rem;
		border-bottom: 1px solid var(--color-border);
	}

	.preview-dots {
		display: flex;
		gap: 0.375rem;
	}

	.preview-dots span {
		width: 0.75rem;
		height: 0.75rem;
		border-radius: 50%;
		background: var(--color-border);
	}

	.preview-dots span:first-child { background: #ef4444; }
	.preview-dots span:nth-child(2) { background: #f59e0b; }
	.preview-dots span:nth-child(3) { background: #22c55e; }

	.preview-title {
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}

	.preview-content {
		padding: 1.5rem;
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.preview-stat {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 1rem;
		background: var(--color-bg);
		border-radius: 0.75rem;
	}

	.stat-label {
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}

	.stat-value {
		font-size: 1.25rem;
		font-weight: 700;
		color: var(--color-text);
	}

	.stat-change {
		font-size: 0.75rem;
		padding: 0.25rem 0.5rem;
		border-radius: 1rem;
		background: #fee2e2;
		color: #dc2626;
	}

	.stat-change.positive {
		background: #dcfce7;
		color: #16a34a;
	}

	/* Social Proof */
	.social-proof {
		text-align: center;
		padding: 3rem 2rem;
		background: var(--color-surface);
		border-top: 1px solid var(--color-border);
		border-bottom: 1px solid var(--color-border);
	}

	.social-proof p {
		color: var(--color-text-muted);
		font-size: 1.125rem;
	}

	/* Features */
	.features {
		padding: 5rem 2rem;
		max-width: 1200px;
		margin: 0 auto;
	}

	.section-header {
		text-align: center;
		margin-bottom: 4rem;
	}

	.section-header h2 {
		font-size: 2.5rem;
		font-weight: 700;
		margin-bottom: 1rem;
		color: var(--color-text);
	}

	.section-header p {
		font-size: 1.125rem;
		color: var(--color-text-muted);
		max-width: 600px;
		margin: 0 auto;
	}

	.features-grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 2rem;
	}

	.feature-card {
		background: var(--color-surface);
		padding: 2rem;
		border-radius: 1rem;
		border: 1px solid var(--color-border);
		transition: all 0.3s ease;
		position: relative;
	}

	.feature-card:hover {
		transform: translateY(-4px);
		box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1);
		border-color: var(--color-primary);
	}

	.feature-card.coming-soon {
		opacity: 0.8;
	}

	.feature-badge {
		position: absolute;
		top: 1rem;
		right: 1rem;
		background: linear-gradient(135deg, #fef3c7 0%, #fde68a 100%);
		color: #92400e;
		padding: 0.25rem 0.75rem;
		border-radius: 1rem;
		font-size: 0.75rem;
		font-weight: 600;
	}

	.feature-icon {
		font-size: 2.5rem;
		margin-bottom: 1rem;
	}

	.feature-card h3 {
		font-size: 1.25rem;
		font-weight: 600;
		margin-bottom: 0.75rem;
		color: var(--color-text);
	}

	.feature-card p {
		color: var(--color-text-muted);
		line-height: 1.6;
	}

	/* Why Section */
	.why-section {
		background: linear-gradient(135deg, #1e3a5f 0%, #2563eb 100%);
		padding: 5rem 2rem;
	}

	.why-content {
		max-width: 1200px;
		margin: 0 auto;
	}

	.why-section h2 {
		text-align: center;
		font-size: 2.5rem;
		font-weight: 700;
		margin-bottom: 3rem;
		color: white;
	}

	.why-grid {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 2rem;
	}

	.why-item {
		text-align: center;
		color: white;
	}

	.why-icon {
		font-size: 3rem;
		margin-bottom: 1rem;
	}

	.why-item h3 {
		font-size: 1.25rem;
		font-weight: 600;
		margin-bottom: 0.75rem;
	}

	.why-item p {
		font-size: 0.875rem;
		opacity: 0.9;
		line-height: 1.6;
	}

	/* Pricing */
	.pricing {
		padding: 5rem 2rem;
		background: var(--color-bg);
	}

	.pricing-grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 2rem;
		max-width: 1000px;
		margin: 0 auto;
	}

	.pricing-card {
		background: var(--color-surface);
		border: 1px solid var(--color-border);
		border-radius: 1rem;
		padding: 2rem;
		position: relative;
		transition: all 0.3s ease;
	}

	.pricing-card:hover {
		transform: translateY(-4px);
		box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1);
	}

	.pricing-card.featured {
		border: 2px solid var(--color-primary);
		transform: scale(1.05);
	}

	.pricing-card.featured:hover {
		transform: scale(1.05) translateY(-4px);
	}

	.pricing-badge {
		position: absolute;
		top: -0.75rem;
		left: 50%;
		transform: translateX(-50%);
		background: var(--color-primary);
		color: white;
		padding: 0.25rem 1rem;
		border-radius: 1rem;
		font-size: 0.75rem;
		font-weight: 600;
	}

	.pricing-header {
		text-align: center;
		padding-bottom: 1.5rem;
		margin-bottom: 1.5rem;
		border-bottom: 1px solid var(--color-border);
	}

	.pricing-header h3 {
		font-size: 1.25rem;
		font-weight: 600;
		margin-bottom: 0.5rem;
		color: var(--color-text);
	}

	.price {
		display: flex;
		align-items: baseline;
		justify-content: center;
		gap: 0.25rem;
	}

	.price-amount {
		font-size: 3rem;
		font-weight: 800;
		color: var(--color-text);
	}

	.price-period {
		color: var(--color-text-muted);
	}

	.pricing-features {
		list-style: none;
		margin-bottom: 2rem;
	}

	.pricing-features li {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.5rem 0;
		color: var(--color-text);
	}

	.btn-block {
		display: block;
		width: 100%;
		text-align: center;
	}

	/* Final CTA */
	.final-cta {
		background: linear-gradient(135deg, var(--color-primary) 0%, #6366f1 100%);
		padding: 5rem 2rem;
		text-align: center;
		color: white;
	}

	.final-cta h2 {
		font-size: 2.5rem;
		font-weight: 700;
		margin-bottom: 1rem;
	}

	.final-cta p {
		font-size: 1.25rem;
		opacity: 0.9;
		margin-bottom: 2rem;
	}

	.cta-form {
		max-width: 500px;
		margin: 0 auto;
	}

	.cta-form form {
		display: flex;
		gap: 0.75rem;
	}

	.cta-form .waitlist-input {
		border-color: rgba(255, 255, 255, 0.3);
		background: rgba(255, 255, 255, 0.1);
		color: white;
	}

	.cta-form .waitlist-input::placeholder {
		color: rgba(255, 255, 255, 0.7);
	}

	.btn-white {
		background: white;
		color: var(--color-primary);
		font-weight: 600;
	}

	.btn-white:hover {
		background: #f0f0f0;
	}

	/* Footer */
	.landing-footer {
		padding: 3rem 2rem;
		text-align: center;
		background: var(--color-surface);
		border-top: 1px solid var(--color-border);
	}

	.footer-content {
		max-width: 1200px;
		margin: 0 auto;
	}

	.footer-brand {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.5rem;
		font-size: 1.25rem;
		font-weight: 700;
		margin-bottom: 1rem;
		color: var(--color-text);
	}

	.footer-links {
		margin-bottom: 1rem;
	}

	.footer-links a {
		color: var(--color-text-muted);
	}

	.separator {
		color: var(--color-border);
		margin: 0 0.75rem;
	}

	.copyright {
		font-size: 0.875rem;
		color: var(--color-text-muted);
	}

	/* Responsive */
	@media (max-width: 1024px) {
		.hero {
			grid-template-columns: 1fr;
			text-align: center;
			padding-top: 6rem;
		}

		.hero-features {
			align-items: center;
		}

		.waitlist-form form {
			flex-direction: column;
		}

		.hero-visual {
			order: -1;
		}

		.features-grid {
			grid-template-columns: repeat(2, 1fr);
		}

		.why-grid {
			grid-template-columns: repeat(2, 1fr);
		}

		.pricing-grid {
			grid-template-columns: 1fr;
			max-width: 400px;
		}

		.pricing-card.featured {
			transform: none;
		}
	}

	@media (max-width: 768px) {
		.nav-container {
			padding: 1rem;
		}

		.nav-links {
			gap: 1rem;
		}

		.nav-links a:not(.btn) {
			display: none;
		}

		.hero {
			padding: 5rem 1rem 3rem;
		}

		.hero h1 {
			font-size: 2.5rem;
		}

		.hero-subtitle {
			font-size: 1rem;
		}

		.features-grid {
			grid-template-columns: 1fr;
		}

		.why-grid {
			grid-template-columns: 1fr;
			gap: 2rem;
		}

		.section-header h2,
		.why-section h2,
		.final-cta h2 {
			font-size: 2rem;
		}

		.cta-form form {
			flex-direction: column;
		}
	}
</style>
