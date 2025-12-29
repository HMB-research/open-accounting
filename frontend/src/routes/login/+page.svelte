<script lang="ts">
	import { api } from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';
	import LanguageSelector from '$lib/components/LanguageSelector.svelte';

	let email = $state('');
	let password = $state('');
	let error = $state('');
	let isLoading = $state(false);
	let isRegister = $state(false);
	let name = $state('');

	async function handleSubmit(e: Event) {
		e.preventDefault();
		error = '';
		isLoading = true;

		try {
			if (isRegister) {
				await api.register(email, password, name);
			}
			await api.login(email, password);
			window.location.href = '/dashboard';
		} catch (err) {
			error = err instanceof Error ? err.message : 'An error occurred';
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
			<div class="alert alert-error">{error}</div>
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
				<input
					class="input"
					type="password"
					id="password"
					bind:value={password}
					required
					minlength="8"
					placeholder="Min 8 characters"
				/>
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
</style>
