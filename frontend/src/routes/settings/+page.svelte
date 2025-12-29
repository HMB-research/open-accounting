<script lang="ts">
	import { page } from '$app/stores';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
</script>

<svelte:head>
	<title>Settings - Open Accounting</title>
</svelte:head>

<div class="container">
	<h1>Settings</h1>

	<div class="settings-grid">
		<a href="/settings/email?tenant={tenantId}" class="settings-card card">
			<div class="card-icon">
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="24"
					height="24"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
				>
					<rect width="20" height="16" x="2" y="4" rx="2" />
					<path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7" />
				</svg>
			</div>
			<div class="card-content">
				<h3>Email Settings</h3>
				<p>Configure SMTP settings and email templates for invoices and notifications.</p>
			</div>
		</a>

		<a href="/settings/plugins?tenant={tenantId}" class="settings-card card">
			<div class="card-icon">
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="24"
					height="24"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
				>
					<path
						d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83"
					/>
				</svg>
			</div>
			<div class="card-content">
				<h3>Plugins</h3>
				<p>Enable or disable plugins for your organization and configure their settings.</p>
			</div>
		</a>
	</div>
</div>

<style>
	h1 {
		font-size: 1.75rem;
		margin-bottom: 1.5rem;
	}

	.settings-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
		gap: 1rem;
	}

	.settings-card {
		display: flex;
		gap: 1rem;
		text-decoration: none;
		color: inherit;
		transition:
			transform 0.2s,
			box-shadow 0.2s;
	}

	.settings-card:hover {
		transform: translateY(-2px);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
		text-decoration: none;
	}

	.card-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 48px;
		height: 48px;
		background: var(--color-primary-light, #e0e7ff);
		border-radius: 8px;
		color: var(--color-primary);
		flex-shrink: 0;
	}

	.card-content h3 {
		margin: 0 0 0.5rem;
		font-size: 1.1rem;
	}

	.card-content p {
		margin: 0;
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}
</style>
