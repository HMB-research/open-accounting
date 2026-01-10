<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import * as m from '$lib/paraglide/messages.js';

	function handleGoHome() {
		goto('/');
	}

	function handleGoBack() {
		history.back();
	}

	function handleRetry() {
		location.reload();
	}

	$effect(() => {
		// Log error for debugging (in dev mode)
		if (import.meta.env.DEV && $page.error) {
			console.error('Page error:', $page.error);
		}
	});
</script>

<svelte:head>
	<title>{m.errors_loadFailed()} - Open Accounting</title>
</svelte:head>

<div class="error-container">
	<div class="error-card">
		<div class="error-icon">
			{#if $page.status === 404}
				<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="64" height="64">
					<circle cx="11" cy="11" r="8" />
					<path d="m21 21-4.35-4.35" />
					<path d="M8 8l6 6" />
					<path d="M14 8l-6 6" />
				</svg>
			{:else if $page.status === 403}
				<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="64" height="64">
					<rect width="18" height="11" x="3" y="11" rx="2" ry="2" />
					<path d="M7 11V7a5 5 0 0 1 10 0v4" />
					<circle cx="12" cy="16" r="1" />
				</svg>
			{:else}
				<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="64" height="64">
					<circle cx="12" cy="12" r="10" />
					<path d="M12 8v4" />
					<path d="M12 16h.01" />
				</svg>
			{/if}
		</div>

		<h1 class="error-status">{$page.status}</h1>

		<p class="error-message">
			{#if $page.status === 404}
				{m.errors_notFound()}
			{:else if $page.status === 403}
				{m.errors_accessDenied()}
			{:else if $page.status === 401}
				{m.errors_sessionExpired()}
			{:else}
				{$page.error?.message || m.errors_loadFailed()}
			{/if}
		</p>

		<div class="error-actions">
			{#if $page.status === 401}
				<a href="/login" class="btn btn-primary">
					{m.auth_login()}
				</a>
			{:else}
				<button class="btn btn-secondary" onclick={handleGoBack}>
					{m.common_back()}
				</button>
				<button class="btn btn-primary" onclick={handleRetry}>
					{m.common_retry()}
				</button>
			{/if}
		</div>

		<div class="error-home">
			<button class="link-btn" onclick={handleGoHome}>
				{m.common_goHome()}
			</button>
		</div>
	</div>
</div>

<style>
	.error-container {
		min-height: 100vh;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 2rem;
		background: var(--color-bg, #f9fafb);
	}

	.error-card {
		max-width: 400px;
		width: 100%;
		text-align: center;
		padding: 2.5rem;
		background: white;
		border-radius: 1rem;
		box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
	}

	.error-icon {
		color: var(--color-text-muted, #6b7280);
		margin-bottom: 1.5rem;
		display: flex;
		justify-content: center;
	}

	.error-status {
		font-size: 3rem;
		font-weight: 700;
		color: var(--color-text, #111827);
		margin: 0 0 0.5rem;
		line-height: 1;
	}

	.error-message {
		color: var(--color-text-muted, #6b7280);
		margin: 0 0 1.5rem;
		font-size: 1rem;
		line-height: 1.5;
	}

	.error-actions {
		display: flex;
		gap: 0.75rem;
		justify-content: center;
		flex-wrap: wrap;
	}

	.btn {
		padding: 0.625rem 1.25rem;
		border-radius: 0.5rem;
		font-weight: 500;
		font-size: 0.875rem;
		cursor: pointer;
		border: none;
		transition: background-color 0.2s, transform 0.1s;
	}

	.btn:active {
		transform: scale(0.98);
	}

	.btn-primary {
		background: var(--color-primary, #2563eb);
		color: white;
	}

	.btn-primary:hover {
		background: var(--color-primary-dark, #1d4ed8);
	}

	.btn-secondary {
		background: var(--color-secondary, #e5e7eb);
		color: var(--color-text, #374151);
	}

	.btn-secondary:hover {
		background: #d1d5db;
	}

	.error-home {
		margin-top: 1.5rem;
	}

	.link-btn {
		background: none;
		border: none;
		color: var(--color-primary, #2563eb);
		cursor: pointer;
		font-size: 0.875rem;
		text-decoration: underline;
	}

	.link-btn:hover {
		color: var(--color-primary-dark, #1d4ed8);
	}

	/* Dark mode */
	@media (prefers-color-scheme: dark) {
		.error-container {
			background: #111827;
		}

		.error-card {
			background: #1f2937;
		}

		.error-status {
			color: #f9fafb;
		}

		.btn-secondary {
			background: #374151;
			color: #f9fafb;
		}

		.btn-secondary:hover {
			background: #4b5563;
		}
	}
</style>
