<script lang="ts">
	import { api } from '$lib/api';

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
	<title>{isRegister ? 'Register' : 'Login'} - Open Accounting</title>
</svelte:head>

<div class="login-page">
	<div class="login-card card">
		<h1>{isRegister ? 'Create Account' : 'Welcome Back'}</h1>
		<p class="subtitle">
			{isRegister ? 'Start managing your finances' : 'Sign in to your account'}
		</p>

		{#if error}
			<div class="alert alert-error">{error}</div>
		{/if}

		<form onsubmit={handleSubmit}>
			{#if isRegister}
				<div class="form-group">
					<label class="label" for="name">Name</label>
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
				<label class="label" for="email">Email</label>
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
				<label class="label" for="password">Password</label>
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
					Loading...
				{:else if isRegister}
					Create Account
				{:else}
					Sign In
				{/if}
			</button>
		</form>

		<p class="toggle-mode">
			{#if isRegister}
				Already have an account?
				<button class="link-btn" type="button" onclick={() => (isRegister = false)}>
					Sign in
				</button>
			{:else}
				Don't have an account?
				<button class="link-btn" type="button" onclick={() => (isRegister = true)}>
					Create one
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
