<script lang="ts">
	import { api } from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';
	import LanguageSelector from '$lib/components/LanguageSelector.svelte';

	let email = $state('');
	let password = $state('');
	let showPassword = $state(false);
	let error = $state('');
	let errorType = $state<'auth' | 'network' | 'server' | 'unknown'>('unknown');
	let isLoading = $state(false);
	let isRegister = $state(false);
	let name = $state('');

	function parseError(err: unknown): { message: string; type: 'auth' | 'network' | 'server' | 'unknown' } {
		if (err instanceof Error) {
			const msg = err.message.toLowerCase();

			// Network/connection errors
			if (msg.includes('failed to fetch') || msg.includes('networkerror') || msg.includes('network')) {
				return {
					message: m.auth_errorNetwork?.() || 'Unable to connect to server. Please check your internet connection.',
					type: 'network'
				};
			}

			// JSON parsing error (usually means server returned HTML error page)
			if (msg.includes('unexpected token') || msg.includes('json') || msg.includes('<!doctype')) {
				return {
					message: m.auth_errorServer?.() || 'Server configuration error. Please try again later.',
					type: 'server'
				};
			}

			// Authentication errors
			if (msg.includes('invalid') || msg.includes('credentials') || msg.includes('unauthorized')) {
				return {
					message: m.auth_errorInvalidCredentials?.() || 'Invalid email or password. Please try again.',
					type: 'auth'
				};
			}

			// Email already exists
			if (msg.includes('email') && msg.includes('exists')) {
				return {
					message: m.auth_errorEmailExists?.() || 'An account with this email already exists.',
					type: 'auth'
				};
			}

			// Rate limiting
			if (msg.includes('rate') || msg.includes('too many')) {
				return {
					message: m.auth_errorRateLimit?.() || 'Too many attempts. Please wait a moment and try again.',
					type: 'auth'
				};
			}

			// Generic error with original message
			return { message: err.message, type: 'unknown' };
		}

		return {
			message: m.auth_errorUnknown?.() || 'An unexpected error occurred. Please try again.',
			type: 'unknown'
		};
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();
		error = '';
		errorType = 'unknown';
		isLoading = true;

		try {
			if (isRegister) {
				await api.register(email, password, name);
			}
			await api.login(email, password);
			window.location.href = '/dashboard';
		} catch (err) {
			const parsed = parseError(err);
			error = parsed.message;
			errorType = parsed.type;
		} finally {
			isLoading = false;
		}
	}
</script>

<svelte:head>
	<title>{isRegister ? m.auth_register() : m.auth_login()} - Open Accounting</title>
</svelte:head>

<div class="login-page">
	<div class="login-card card">
		<div class="language-top">
			<LanguageSelector />
		</div>
		<h1>{isRegister ? m.auth_register() : m.auth_welcomeBack()}</h1>
		<p class="subtitle">
			{isRegister ? m.auth_register() : m.auth_signInPrompt()}
		</p>

		{#if error}
			<div class="alert alert-error" class:alert-warning={errorType === 'network' || errorType === 'server'}>
				<span class="error-icon">
					{#if errorType === 'network'}
						<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12.55a11 11 0 0 1 14.08 0"/><path d="M1.42 9a16 16 0 0 1 21.16 0"/><path d="M8.53 16.11a6 6 0 0 1 6.95 0"/><line x1="12" y1="20" x2="12.01" y2="20"/></svg>
					{:else if errorType === 'server'}
						<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"/><rect x="2" y="14" width="20" height="8" rx="2" ry="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/></svg>
					{:else}
						<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
					{/if}
				</span>
				<span class="error-message">{error}</span>
			</div>
		{/if}

		<form onsubmit={handleSubmit}>
			{#if isRegister}
				<div class="form-group">
					<label class="label" for="name">{m.common_name()}</label>
					<input
						class="input"
						type="text"
						id="name"
						bind:value={name}
						required
						placeholder="Your name"
					/>
				</div>
			{/if}

			<div class="form-group">
				<label class="label" for="email">{m.auth_email()}</label>
				<input
					class="input"
					type="email"
					id="email"
					bind:value={email}
					required
					placeholder="you@example.com"
				/>
			</div>

			<div class="form-group">
				<label class="label" for="password">{m.auth_password()}</label>
				<div class="password-input-wrapper">
					<input
						class="input"
						type={showPassword ? 'text' : 'password'}
						id="password"
						bind:value={password}
						required
						minlength={isRegister ? 8 : undefined}
						placeholder={isRegister ? m.auth_passwordMinLength() : m.auth_password()}
					/>
					<button
						type="button"
						class="password-toggle"
						onclick={() => showPassword = !showPassword}
						aria-label={showPassword ? 'Hide password' : 'Show password'}
					>
						{#if showPassword}
							<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
						{:else}
							<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
						{/if}
					</button>
				</div>
			</div>

			<button class="btn btn-primary btn-full" type="submit" disabled={isLoading}>
				{#if isLoading}
					{m.common_loading()}
				{:else if isRegister}
					{m.auth_register()}
				{:else}
					{m.auth_login()}
				{/if}
			</button>
		</form>

		<p class="toggle-mode">
			{#if isRegister}
				{m.auth_hasAccount()}
				<button class="link-btn" type="button" onclick={() => (isRegister = false)}>
					{m.auth_login()}
				</button>
			{:else}
				{m.auth_noAccount()}
				<button class="link-btn" type="button" onclick={() => (isRegister = true)}>
					{m.auth_register()}
				</button>
			{/if}
		</p>
	</div>
</div>

<style>
	.login-page {
		min-height: 100vh;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 1rem;
	}

	.login-card {
		width: 100%;
		max-width: 400px;
		position: relative;
	}

	.language-top {
		position: absolute;
		top: 1rem;
		right: 1rem;
	}

	h1 {
		font-size: 1.5rem;
		margin-bottom: 0.25rem;
	}

	.subtitle {
		color: var(--color-text-muted);
		margin-bottom: 1.5rem;
	}

	.btn-full {
		width: 100%;
		justify-content: center;
	}

	.toggle-mode {
		margin-top: 1.5rem;
		text-align: center;
		color: var(--color-text-muted);
	}

	.link-btn {
		background: none;
		border: none;
		color: var(--color-primary);
		font-weight: 500;
	}

	.link-btn:hover {
		text-decoration: underline;
	}

	.alert {
		display: flex;
		align-items: flex-start;
		gap: 0.75rem;
		padding: 0.875rem 1rem;
		border-radius: 0.5rem;
		margin-bottom: 1rem;
	}

	.alert-error {
		background-color: #fef2f2;
		border: 1px solid #fecaca;
		color: #dc2626;
	}

	.alert-warning {
		background-color: #fffbeb;
		border: 1px solid #fde68a;
		color: #d97706;
	}

	.error-icon {
		flex-shrink: 0;
		margin-top: 0.125rem;
	}

	.error-message {
		font-size: 0.875rem;
		line-height: 1.5;
	}

	.password-input-wrapper {
		position: relative;
	}

	.password-input-wrapper .input {
		padding-right: 2.75rem;
	}

	.password-toggle {
		position: absolute;
		right: 0.5rem;
		top: 50%;
		transform: translateY(-50%);
		background: none;
		border: none;
		cursor: pointer;
		color: var(--color-text-muted);
		padding: 0.25rem;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.password-toggle:hover {
		color: var(--color-text);
	}
</style>
