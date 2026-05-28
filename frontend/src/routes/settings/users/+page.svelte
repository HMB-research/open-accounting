<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import {
		api,
		type EditableTenantRole,
		type TenantUser,
		type UserInvitation
	} from '$lib/api';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import * as m from '$lib/paraglide/messages.js';
	import { parseApiError } from '$lib/utils/tenant';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let users = $state<TenantUser[]>([]);
	let invitations = $state<UserInvitation[]>([]);
	let selectedRoles = $state<Record<string, EditableTenantRole>>({});
	let isLoading = $state(true);
	let error = $state('');
	let success = $state('');
	let actionId = $state('');
	let inviteEmail = $state('');
	let inviteRole = $state<EditableTenantRole>('viewer');

	const editableRoles: EditableTenantRole[] = ['admin', 'accountant', 'viewer'];

	onMount(() => {
		void loadWorkspace();
	});

	function setNoTenantState() {
		users = [];
		invitations = [];
		selectedRoles = {};
		error = m.settings_noTenantSelected();
		isLoading = false;
	}

	async function loadWorkspace(showLoading = true) {
		if (!tenantId) {
			setNoTenantState();
			return;
		}

		if (showLoading) {
			isLoading = true;
		}
		error = '';

		try {
			const [loadedUsers, loadedInvitations] = await Promise.all([
				api.listTenantUsers(tenantId),
				api.listInvitations(tenantId)
			]);
			users = loadedUsers;
			invitations = loadedInvitations;
			selectedRoles = Object.fromEntries(
				loadedUsers
					.filter((user) => user.role !== 'owner')
					.map((user) => [user.user_id, user.role as EditableTenantRole])
			);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			isLoading = false;
		}
	}

	function isEditableRole(value: string): value is EditableTenantRole {
		return editableRoles.includes(value as EditableTenantRole);
	}

	function changeSelectedRole(userId: string, event: Event) {
		const nextRole = (event.currentTarget as HTMLSelectElement).value;
		if (!isEditableRole(nextRole)) {
			return;
		}

		selectedRoles = { ...selectedRoles, [userId]: nextRole };
	}

	function canEditUser(user: TenantUser): boolean {
		return user.role !== 'owner';
	}

	function isRoleDirty(user: TenantUser): boolean {
		return canEditUser(user) && selectedRoles[user.user_id] !== undefined && selectedRoles[user.user_id] !== user.role;
	}

	async function updateUserRole(user: TenantUser) {
		if (!tenantId || !isRoleDirty(user)) {
			return;
		}

		const nextRole = selectedRoles[user.user_id];
		if (!nextRole) {
			return;
		}

		actionId = `role:${user.user_id}`;
		error = '';
		success = '';

		try {
			await api.updateTenantUserRole(tenantId, user.user_id, nextRole);
			success = `Updated ${user.user_id} to ${formatRole(nextRole)}`;
			await loadWorkspace(false);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionId = '';
		}
	}

	async function removeUser(user: TenantUser) {
		if (!tenantId || !canEditUser(user)) {
			return;
		}

		if (!confirm(`Remove ${user.user_id} from this tenant?`)) {
			return;
		}

		actionId = `remove:${user.user_id}`;
		error = '';
		success = '';

		try {
			await api.removeTenantUser(tenantId, user.user_id);
			users = users.filter((item) => item.user_id !== user.user_id);
			success = `Removed ${user.user_id}`;
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionId = '';
		}
	}

	async function submitInvitation(event: Event) {
		event.preventDefault();
		if (!tenantId) {
			setNoTenantState();
			return;
		}

		actionId = 'invite';
		error = '';
		success = '';

		try {
			const invitation = await api.createInvitation(tenantId, {
				email: inviteEmail.trim(),
				role: inviteRole
			});
			invitations = [invitation, ...invitations];
			inviteEmail = '';
			inviteRole = 'viewer';
			success = `Invited ${invitation.email}`;
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionId = '';
		}
	}

	async function revokeInvitation(invitation: UserInvitation) {
		if (!tenantId) {
			return;
		}

		if (!confirm(`Revoke invitation for ${invitation.email}?`)) {
			return;
		}

		actionId = `revoke:${invitation.id}`;
		error = '';
		success = '';

		try {
			await api.revokeInvitation(tenantId, invitation.id);
			invitations = invitations.filter((item) => item.id !== invitation.id);
			success = `Revoked invitation for ${invitation.email}`;
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionId = '';
		}
	}

	function formatRole(role: string): string {
		return role
			.split('_')
			.map((part) => part.charAt(0).toUpperCase() + part.slice(1))
			.join(' ');
	}

	function formatDateTime(value: string | undefined): string {
		if (!value) {
			return '-';
		}

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
</script>

<svelte:head>
	<title>Users and invitations - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<div>
			<a class="back-link" href={tenantId ? `/settings?tenant=${tenantId}` : '/settings'}>
				{m.settings_backToSettings()}
			</a>
			<h1>Users and invitations</h1>
		</div>

		<button class="btn btn-secondary" type="button" onclick={() => loadWorkspace()} disabled={isLoading || !!actionId}>
			Refresh
		</button>
	</div>

	{#if success}
		<ErrorAlert message={success} type="success" onDismiss={() => (success = '')} />
	{/if}

	{#if error}
		<ErrorAlert message={error} type="error" onDismiss={() => (error = '')} />
	{/if}

	{#if isLoading}
		<div class="loading">{m.common_loading()}</div>
	{:else if !tenantId}
		<div class="card empty-state">
			<p>{m.settings_selectTenantDashboard()} <a href="/dashboard">{m.dashboard_title()}</a>.</p>
		</div>
	{:else}
		<section class="admin-section">
			<div class="section-heading">
				<h2>Members</h2>
			</div>

			{#if users.length === 0}
				<div class="card empty-state">
					<p>No members found.</p>
				</div>
			{:else}
				<div class="card table-container">
					<table class="table members-table">
						<thead>
							<tr>
								<th>User</th>
								<th>Role</th>
								<th>Default</th>
								<th>Joined</th>
								<th>Actions</th>
							</tr>
						</thead>
						<tbody>
							{#each users as user (user.user_id)}
								<tr>
									<td data-label="User" class="mono">{user.user_id}</td>
									<td data-label="Role">
										{#if canEditUser(user)}
											<select
												class="input role-select"
												aria-label={`Role for ${user.user_id}`}
												name={`role-${user.user_id}`}
												value={selectedRoles[user.user_id] || user.role}
												onchange={(event) => changeSelectedRole(user.user_id, event)}
											>
												{#each editableRoles as role (role)}
													<option value={role}>{formatRole(role)}</option>
												{/each}
											</select>
										{:else}
											<span class="role-badge">{formatRole(user.role)}</span>
										{/if}
									</td>
									<td data-label="Default">{user.is_default ? 'Yes' : 'No'}</td>
									<td data-label="Joined">{formatDateTime(user.created_at)}</td>
									<td data-label="Actions">
										<div class="row-actions">
											<button
												class="btn btn-secondary btn-sm"
												type="button"
												onclick={() => updateUserRole(user)}
												disabled={!isRoleDirty(user) || !!actionId}
											>
												Update
											</button>
											<button
												class="btn btn-danger btn-sm"
												type="button"
												onclick={() => removeUser(user)}
												disabled={!canEditUser(user) || !!actionId}
											>
												Remove
											</button>
										</div>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</section>

		<section class="admin-section">
			<div class="section-heading">
				<h2>Invite user</h2>
			</div>

			<form class="card invite-form" onsubmit={submitInvitation}>
				<div class="form-group">
					<label class="label" for="invite-email">Email</label>
					<input
						id="invite-email"
						class="input"
						type="email"
						bind:value={inviteEmail}
						autocomplete="email"
						required
					/>
				</div>
				<div class="form-group">
					<label class="label" for="invite-role">Role</label>
					<select id="invite-role" class="input" bind:value={inviteRole}>
						{#each editableRoles as role (role)}
							<option value={role}>{formatRole(role)}</option>
						{/each}
					</select>
				</div>
				<button class="btn btn-primary invite-button" type="submit" disabled={actionId === 'invite'}>
					Invite
				</button>
			</form>
		</section>

		<section class="admin-section">
			<div class="section-heading">
				<h2>Pending invitations</h2>
			</div>

			{#if invitations.length === 0}
				<div class="card empty-state">
					<p>No pending invitations.</p>
				</div>
			{:else}
				<div class="card table-container">
					<table class="table invitations-table">
						<thead>
							<tr>
								<th>Email</th>
								<th>Role</th>
								<th>Invited by</th>
								<th>Expires</th>
								<th>Created</th>
								<th>Actions</th>
							</tr>
						</thead>
						<tbody>
							{#each invitations as invitation (invitation.id)}
								<tr>
									<td data-label="Email">{invitation.email}</td>
									<td data-label="Role">
										<span class="role-badge">{formatRole(invitation.role)}</span>
									</td>
									<td data-label="Invited by" class="mono">{invitation.invited_by}</td>
									<td data-label="Expires">{formatDateTime(invitation.expires_at)}</td>
									<td data-label="Created">{formatDateTime(invitation.created_at)}</td>
									<td data-label="Actions">
										<button
											class="btn btn-danger btn-sm"
											type="button"
											onclick={() => revokeInvitation(invitation)}
											disabled={!!actionId}
										>
											Revoke
										</button>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</section>
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

	.admin-section {
		margin-bottom: 1.5rem;
	}

	.section-heading {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 0.75rem;
	}

	h2 {
		font-size: 1.15rem;
		margin: 0;
	}

	.loading,
	.empty-state {
		text-align: center;
		padding: 2rem;
		color: var(--color-text-muted);
	}

	.members-table {
		min-width: 840px;
	}

	.invitations-table {
		min-width: 960px;
	}

	.members-table th,
	.members-table td,
	.invitations-table th,
	.invitations-table td {
		vertical-align: middle;
	}

	.mono {
		font-family: var(--font-mono);
		font-size: 0.8125rem;
	}

	.role-select {
		width: 150px;
	}

	.role-badge {
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

	.row-actions {
		display: flex;
		gap: 0.5rem;
		flex-wrap: wrap;
	}

	.btn-sm {
		padding: 0.4rem 0.7rem;
		font-size: 0.8125rem;
	}

	.btn-danger {
		background: rgba(239, 68, 68, 0.1);
		border-color: rgba(239, 68, 68, 0.25);
		color: var(--color-error);
	}

	.btn-danger:hover {
		background: rgba(239, 68, 68, 0.16);
		text-decoration: none;
	}

	.invite-form {
		display: grid;
		grid-template-columns: minmax(220px, 1fr) minmax(180px, 220px) auto;
		gap: 1rem;
		align-items: end;
	}

	.invite-form .form-group {
		margin-bottom: 0;
	}

	.invite-button {
		justify-content: center;
		min-width: 110px;
	}

	@media (max-width: 780px) {
		.header {
			align-items: stretch;
			flex-direction: column;
		}

		.header .btn {
			justify-content: center;
		}

		.invite-form {
			grid-template-columns: 1fr;
		}

		.invite-button {
			width: 100%;
		}
	}
</style>
