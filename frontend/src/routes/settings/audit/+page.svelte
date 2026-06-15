<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { api, type TenantAuditEvent } from '$lib/api';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import * as m from '$lib/paraglide/messages.js';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let events = $state<TenantAuditEvent[]>([]);
	let isLoading = $state(true);
	let error = $state('');
	let eventLimit = $state(50);

	const limitOptions = [25, 50, 100, 200];

	onMount(() => {
		refreshEvents();
	});

	function setNoTenantState() {
		events = [];
		error = m.settings_noTenantSelected();
		isLoading = false;
	}

	function refreshEvents() {
		if (!tenantId) {
			setNoTenantState();
			return;
		}

		void loadAuditEvents(tenantId, eventLimit);
	}

	async function loadAuditEvents(selectedTenantId: string, limit: number) {
		isLoading = true;
		error = '';

		try {
			events = await api.listTenantAuditEvents(selectedTenantId, limit);
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_loadFailed();
		} finally {
			isLoading = false;
		}
	}

	function changeEventLimit(event: Event) {
		const nextLimit = Number((event.currentTarget as HTMLSelectElement).value);
		if (!limitOptions.includes(nextLimit)) {
			return;
		}

		eventLimit = nextLimit;
		if (!tenantId) {
			setNoTenantState();
			return;
		}

		void loadAuditEvents(tenantId, nextLimit);
	}

	function formatAction(action: TenantAuditEvent['action']): string {
		switch (action) {
			case 'user_role_updated':
				return 'Role updated';
			case 'user_removed':
				return 'User removed';
			case 'invitation_created':
				return 'Invitation created';
			case 'invitation_revoked':
				return 'Invitation revoked';
			case 'tenant_updated':
				return 'Tenant updated';
			case 'user_session_revoked':
				return 'Session revoked';
			case 'user_sessions_revoked':
				return 'Sessions revoked';
			case 'user_api_token_revoked':
				return 'API token revoked';
			case 'user_status_updated':
				return 'User status updated';
		}
	}

	function formatDateTime(value: string): string {
		const parsed = new Date(value);
		if (Number.isNaN(parsed.getTime())) {
			return value;
		}

		return new Intl.DateTimeFormat(undefined, {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		}).format(parsed);
	}

	function formatMetadataKey(key: string): string {
		return key
			.split('_')
			.map((part) => part.charAt(0).toUpperCase() + part.slice(1))
			.join(' ');
	}

	function formatMetadata(metadata: TenantAuditEvent['metadata']): string {
		const entries = Object.entries(metadata || {}).filter(([, value]) => value);
		if (entries.length === 0) {
			return '-';
		}

		return entries
			.sort(([left], [right]) => left.localeCompare(right))
			.map(([key, value]) => `${formatMetadataKey(key)}: ${value}`)
			.join(', ');
	}

	function getTargetLabel(event: TenantAuditEvent): string {
		return `${formatMetadataKey(event.target_type)}: ${event.target_id}`;
	}
</script>

<svelte:head>
	<title>Audit events - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<div>
			<a class="back-link" href={tenantId ? `/settings?tenant=${tenantId}` : '/settings'}>
				{m.settings_backToSettings()}
			</a>
			<h1>Audit events</h1>
		</div>

		<div class="toolbar" aria-label="Audit event controls">
			<label class="limit-label" for="event-limit">Rows</label>
			<select id="event-limit" class="input limit-select" value={eventLimit} onchange={changeEventLimit}>
				{#each limitOptions as option (option)}
					<option value={option}>{option}</option>
				{/each}
			</select>
			<button class="btn btn-secondary" type="button" onclick={refreshEvents} disabled={isLoading || !tenantId}>
				Refresh
			</button>
		</div>
	</div>

	{#if error}
		<ErrorAlert message={error} type="error" onDismiss={() => (error = '')} />
	{/if}

	{#if isLoading}
		<div class="loading">{m.common_loading()}</div>
	{:else if !tenantId}
		<div class="card empty-state">
			<p>
				{m.settings_selectTenantDashboard()}
				<a href="/dashboard">{m.dashboard_title()}</a>.
			</p>
		</div>
	{:else if events.length === 0}
		<div class="card empty-state">
			<p>No audit events found.</p>
		</div>
	{:else}
		<div class="card table-container">
			<table class="table audit-table">
				<thead>
					<tr>
						<th>Time</th>
						<th>Action</th>
						<th>Actor</th>
						<th>Target</th>
						<th>Email</th>
						<th>Details</th>
					</tr>
				</thead>
				<tbody>
					{#each events as event (event.id)}
						<tr>
							<td data-label="Time">{formatDateTime(event.created_at)}</td>
							<td data-label="Action">
								<span class="action-badge">{formatAction(event.action)}</span>
							</td>
							<td data-label="Actor" class="mono">{event.actor_user_id || '-'}</td>
							<td data-label="Target" class="mono">{getTargetLabel(event)}</td>
							<td data-label="Email">{event.target_email || '-'}</td>
							<td data-label="Details" class="details">{formatMetadata(event.metadata)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

<style>
	.header {
		display: flex;
		align-items: flex-end;
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 1.5rem;
	}

	.back-link {
		display: inline-flex;
		margin-bottom: 0.5rem;
		font-weight: 600;
	}

	h1 {
		font-size: 1.75rem;
		margin: 0;
	}

	.toolbar {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		flex-wrap: wrap;
		justify-content: flex-end;
	}

	.limit-label {
		color: var(--color-text-muted);
		font-size: 0.875rem;
		font-weight: 600;
	}

	.limit-select {
		width: 96px;
	}

	.loading,
	.empty-state {
		text-align: center;
		padding: 2rem;
		color: var(--color-text-muted);
	}

	.audit-table {
		min-width: 960px;
	}

	.audit-table th,
	.audit-table td {
		vertical-align: top;
	}

	.action-badge {
		display: inline-flex;
		align-items: center;
		min-height: 1.75rem;
		padding: 0.25rem 0.6rem;
		border-radius: 999px;
		background: rgba(37, 99, 235, 0.1);
		color: var(--color-primary-dark);
		font-size: 0.8125rem;
		font-weight: 700;
		white-space: nowrap;
	}

	.mono {
		font-family: var(--font-mono);
		font-size: 0.8125rem;
	}

	.details {
		max-width: 360px;
		color: var(--color-text-muted);
	}

	@media (max-width: 720px) {
		.header {
			align-items: stretch;
			flex-direction: column;
		}

		.toolbar {
			justify-content: flex-start;
		}

		.toolbar .btn {
			flex: 1 1 140px;
			justify-content: center;
		}
	}
</style>
