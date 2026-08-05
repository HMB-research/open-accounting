<script lang="ts">
	import { page } from '$app/stores';
	import * as m from '$lib/paraglide/messages.js';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
</script>

<svelte:head>
	<title>{m.settings_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<h1>{m.settings_title()}</h1>

	<div class="settings-grid">
		<a href="/settings/company?tenant={tenantId}" class="settings-card card">
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
					<path d="M6 22V4a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v18Z" />
					<path d="M6 12H4a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h2" />
					<path d="M18 9h2a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2h-2" />
					<path d="M10 6h4" />
					<path d="M10 10h4" />
					<path d="M10 14h4" />
					<path d="M10 18h4" />
				</svg>
			</div>
			<div class="card-content">
				<h3>{m.settings_companyProfile()}</h3>
				<p>{m.settings_companyProfileDesc()}</p>
			</div>
		</a>

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
				<h3>{m.settings_emailSettings()}</h3>
				<p>{m.settings_emailSettingsDesc()}</p>
			</div>
		</a>

		<a href="/settings/reminders?tenant={tenantId}" class="settings-card card">
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
					<path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9" />
					<path d="M13.73 21a2 2 0 0 1-3.46 0" />
				</svg>
			</div>
			<div class="card-content">
				<h3>Reminder Rules</h3>
				<p>Configure automated payment reminders</p>
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
				<h3>{m.settings_plugins()}</h3>
				<p>{m.settings_pluginsDesc()}</p>
			</div>
		</a>

		<a href="/settings/interest?tenant={tenantId}" class="settings-card card">
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
					<line x1="12" y1="1" x2="12" y2="23" />
					<path d="M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6" />
				</svg>
			</div>
			<div class="card-content">
				<h3>Late Payment Interest</h3>
				<p>Configure interest rates for overdue invoices</p>
			</div>
		</a>

		<a href="/settings/audit?tenant={tenantId}" class="settings-card card">
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
					<path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.5 3.8 17 5 19 5a1 1 0 0 1 1 1z" />
					<path d="m9 12 2 2 4-4" />
				</svg>
			</div>
			<div class="card-content">
				<h3>Audit events</h3>
				<p>Review tenant administration activity</p>
			</div>
		</a>

		<a href="/documents?tenant={tenantId}" class="settings-card card">
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
					<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
					<path d="M14 2v6h6" />
					<path d="M9 15l2 2 4-4" />
				</svg>
			</div>
			<div class="card-content">
				<h3>{m.settings_documentsReview()}</h3>
				<p>{m.settings_documentsReviewDesc()}</p>
			</div>
		</a>

		<a href="/settings/users?tenant={tenantId}" class="settings-card card">
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
					<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
					<circle cx="9" cy="7" r="4" />
					<path d="M22 21v-2a4 4 0 0 0-3-3.87" />
					<path d="M16 3.13a4 4 0 0 1 0 7.75" />
				</svg>
			</div>
			<div class="card-content">
				<h3>Users and invitations</h3>
				<p>Manage tenant members and pending invitations</p>
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
