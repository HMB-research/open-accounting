<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import {
		api,
		type APIToken,
		type EditableTenantRole,
		type RefreshSession,
		type SecurityAuditEvent,
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
	let selectedUserId = $state('');
	let userSessions = $state<RefreshSession[]>([]);
	let userTokens = $state<APIToken[]>([]);
	let userSecurityEvents = $state<SecurityAuditEvent[]>([]);
	let includeInactiveSessions = $state(false);
	let securityEventLimit = $state(25);
	let isLoadingUserSecurity = $state(false);
	let isLoading = $state(true);
	let error = $state('');
	let success = $state('');
	let actionId = $state('');
	let inviteEmail = $state('');
	let inviteRole = $state<EditableTenantRole>('viewer');
	let selectedUser = $derived(users.find((user) => user.user_id === selectedUserId));
	let canManageSelectedUser = $derived(selectedUser ? canEditUser(selectedUser) : false);

	const editableRoles: EditableTenantRole[] = ['admin', 'accountant', 'viewer'];
	const securityEventLimitOptions = [10, 25, 50, 100];

	onMount(() => {
		void loadWorkspace();
	});

	function setNoTenantState() {
		users = [];
		invitations = [];
		selectedRoles = {};
		selectedUserId = '';
		clearUserSecurityState();
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
			if (selectedUserId && !loadedUsers.some((user) => user.user_id === selectedUserId)) {
				selectedUserId = '';
				clearUserSecurityState();
			}
		} catch (err) {
			error = parseApiError(err);
		} finally {
			isLoading = false;
		}
	}

	function clearUserSecurityState() {
		userSessions = [];
		userTokens = [];
		userSecurityEvents = [];
		isLoadingUserSecurity = false;
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

	function canChangeUserStatus(user: TenantUser): boolean {
		return canEditUser(user);
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

	async function updateUserStatus(user: TenantUser, isActive: boolean) {
		if (!tenantId || !canChangeUserStatus(user)) {
			return;
		}

		if (!confirm(`${isActive ? 'Restore' : 'Suspend'} ${user.user_id}?`)) {
			return;
		}

		actionId = `status:${user.user_id}`;
		error = '';
		success = '';

		try {
			await api.updateTenantUserStatus(tenantId, user.user_id, isActive);
			users = users.map((item) => (item.user_id === user.user_id ? { ...item, is_active: isActive } : item));
			success = `${isActive ? 'Restored' : 'Suspended'} ${user.user_id}`;
			if (selectedUserId === user.user_id) {
				await loadUserSecurity(user.user_id, false);
			}
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
			if (selectedUserId === user.user_id) {
				selectedUserId = '';
				clearUserSecurityState();
			}
			success = `Removed ${user.user_id}`;
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionId = '';
		}
	}

	async function inspectUserSecurity(user: TenantUser) {
		if (!tenantId) {
			return;
		}

		selectedUserId = user.user_id;
		await loadUserSecurity(user.user_id);
	}

	async function loadUserSecurity(userId = selectedUserId, showLoading = true) {
		if (!tenantId || !userId) {
			clearUserSecurityState();
			return;
		}

		if (showLoading) {
			isLoadingUserSecurity = true;
		}
		error = '';

		try {
			const [sessions, tokens, events] = await Promise.all([
				api.listTenantUserAuthSessions(tenantId, userId, includeInactiveSessions),
				api.listTenantUserAPITokens(tenantId, userId),
				api.listTenantUserSecurityAuditEvents(tenantId, userId, securityEventLimit)
			]);
			userSessions = sessions;
			userTokens = tokens;
			userSecurityEvents = events;
		} catch (err) {
			error = parseApiError(err);
		} finally {
			isLoadingUserSecurity = false;
		}
	}

	async function toggleInactiveSessions(event: Event) {
		includeInactiveSessions = (event.currentTarget as HTMLInputElement).checked;
		await loadUserSecurity();
	}

	async function changeSecurityEventLimit(event: Event) {
		const nextLimit = Number((event.currentTarget as HTMLSelectElement).value);
		if (!securityEventLimitOptions.includes(nextLimit)) {
			return;
		}
		securityEventLimit = nextLimit;
		await loadUserSecurity();
	}

	async function revokeSession(session: RefreshSession) {
		if (!tenantId || !selectedUserId || !canManageSelectedUser) {
			return;
		}

		if (!confirm(`Revoke session ${session.id}?`)) {
			return;
		}

		actionId = `session:${session.id}`;
		error = '';
		success = '';

		try {
			await api.revokeTenantUserAuthSession(tenantId, selectedUserId, session.id);
			success = `Revoked session ${session.id}`;
			await loadUserSecurity(selectedUserId, false);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionId = '';
		}
	}

	async function revokeAllSessions() {
		if (!tenantId || !selectedUserId || !canManageSelectedUser) {
			return;
		}

		if (!confirm(`Revoke all active sessions for ${selectedUserId}?`)) {
			return;
		}

		actionId = `sessions:${selectedUserId}`;
		error = '';
		success = '';

		try {
			await api.revokeTenantUserAuthSessions(tenantId, selectedUserId);
			success = `Revoked all active sessions for ${selectedUserId}`;
			await loadUserSecurity(selectedUserId, false);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionId = '';
		}
	}

	async function revokeAPIToken(token: APIToken) {
		if (!tenantId || !selectedUserId || !canManageSelectedUser) {
			return;
		}

		if (!confirm(`Revoke API token ${token.name}?`)) {
			return;
		}

		actionId = `token:${token.id}`;
		error = '';
		success = '';

		try {
			await api.revokeTenantUserAPIToken(tenantId, selectedUserId, token.id);
			success = `Revoked API token ${token.name}`;
			await loadUserSecurity(selectedUserId, false);
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

	function formatStatus(user: TenantUser): string {
		return user.is_active ? 'Active' : 'Suspended';
	}

	function isActiveSession(session: RefreshSession): boolean {
		if (session.revoked_at) {
			return false;
		}
		const expiresAt = new Date(session.expires_at);
		return !Number.isNaN(expiresAt.getTime()) && expiresAt.getTime() > Date.now();
	}

	function isActiveToken(token: APIToken): boolean {
		if (token.revoked_at) {
			return false;
		}
		if (!token.expires_at) {
			return true;
		}
		const expiresAt = new Date(token.expires_at);
		return !Number.isNaN(expiresAt.getTime()) && expiresAt.getTime() > Date.now();
	}

	function formatSecurityAction(action: string): string {
		return action
			.split('_')
			.map((part) => part.charAt(0).toUpperCase() + part.slice(1))
			.join(' ');
	}

	function formatMetadata(metadata: Record<string, string> | undefined): string {
		const entries = Object.entries(metadata || {}).filter(([, value]) => value);
		if (entries.length === 0) {
			return '-';
		}

		return entries
			.sort(([left], [right]) => left.localeCompare(right))
			.map(([key, value]) => `${formatSecurityAction(key)}: ${value}`)
			.join(', ');
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
			<p>
				{m.settings_selectTenantDashboard()}
				<a href="/dashboard">{m.dashboard_title()}</a>.
			</p>
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
								<th>Status</th>
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
									<td data-label="Status">
										<span class:status-suspended={!user.is_active} class="status-badge">
											{formatStatus(user)}
										</span>
									</td>
									<td data-label="Default">{user.is_default ? 'Yes' : 'No'}</td>
									<td data-label="Joined">{formatDateTime(user.created_at)}</td>
									<td data-label="Actions">
										<div class="row-actions">
											<button
												class="btn btn-secondary btn-sm"
												type="button"
												onclick={() => inspectUserSecurity(user)}
												disabled={!!actionId || isLoadingUserSecurity}
											>
												Inspect
											</button>
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
											{#if user.is_active}
												<button
													class="btn btn-danger btn-sm"
													type="button"
													onclick={() => updateUserStatus(user, false)}
													disabled={!canChangeUserStatus(user) || !!actionId}
												>
													Suspend
												</button>
											{:else}
												<button
													class="btn btn-secondary btn-sm"
													type="button"
													onclick={() => updateUserStatus(user, true)}
													disabled={!canChangeUserStatus(user) || !!actionId}
												>
													Restore
												</button>
											{/if}
										</div>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</section>

		{#if selectedUser}
			<section class="admin-section user-security-section">
				<div class="section-heading security-heading">
					<div>
						<h2>Security for {selectedUser.user_id}</h2>
						<p class="section-note">
							Role: {formatRole(selectedUser.role)} · Status: {formatStatus(selectedUser)}
						</p>
					</div>
					<div class="row-actions">
						<button
							class="btn btn-secondary btn-sm"
							type="button"
							onclick={() => loadUserSecurity()}
							disabled={isLoadingUserSecurity || !!actionId}
						>
							Refresh security
						</button>
						<button
							class="btn btn-danger btn-sm"
							type="button"
							onclick={revokeAllSessions}
							disabled={!canManageSelectedUser || isLoadingUserSecurity || !!actionId}
						>
							Revoke sessions
						</button>
					</div>
				</div>

				{#if isLoadingUserSecurity}
					<div class="loading compact-loading">{m.common_loading()}</div>
				{:else}
					<div class="security-grid">
						<div class="security-panel">
							<div class="panel-heading">
								<h3>Refresh sessions</h3>
								<label class="checkbox-label" for="include-inactive-sessions">
									<input
										id="include-inactive-sessions"
										type="checkbox"
										checked={includeInactiveSessions}
										onchange={toggleInactiveSessions}
									/>
									Include inactive
								</label>
							</div>

							{#if userSessions.length === 0}
								<p class="empty-inline">No refresh sessions found.</p>
							{:else}
								<div class="table-container compact-table">
									<table class="table security-table">
										<thead>
											<tr>
												<th>Session</th>
												<th>Status</th>
												<th>Last used</th>
												<th>Expires</th>
												<th>Actions</th>
											</tr>
										</thead>
										<tbody>
											{#each userSessions as session (session.id)}
												<tr>
													<td data-label="Session" class="mono">{session.id}</td>
													<td data-label="Status">
														<span class:status-suspended={!isActiveSession(session)} class="status-badge">
															{isActiveSession(session) ? 'Active' : 'Inactive'}
														</span>
													</td>
													<td data-label="Last used">{formatDateTime(session.last_used_at)}</td>
													<td data-label="Expires">{formatDateTime(session.expires_at)}</td>
													<td data-label="Actions">
														<button
															class="btn btn-danger btn-sm"
															type="button"
															onclick={() => revokeSession(session)}
															disabled={!isActiveSession(session) || !canManageSelectedUser || !!actionId}
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
						</div>

						<div class="security-panel">
							<div class="panel-heading">
								<h3>API tokens</h3>
							</div>

							{#if userTokens.length === 0}
								<p class="empty-inline">No API tokens found.</p>
							{:else}
								<div class="table-container compact-table">
									<table class="table security-table">
										<thead>
											<tr>
												<th>Name</th>
												<th>Prefix</th>
												<th>Status</th>
												<th>Last used</th>
												<th>Actions</th>
											</tr>
										</thead>
										<tbody>
											{#each userTokens as token (token.id)}
												<tr>
													<td data-label="Name">{token.name}</td>
													<td data-label="Prefix" class="mono">{token.token_prefix}</td>
													<td data-label="Status">
														<span class:status-suspended={!isActiveToken(token)} class="status-badge">
															{isActiveToken(token) ? 'Active' : 'Inactive'}
														</span>
													</td>
													<td data-label="Last used">{formatDateTime(token.last_used_at)}</td>
													<td data-label="Actions">
														<button
															class="btn btn-danger btn-sm"
															type="button"
															onclick={() => revokeAPIToken(token)}
															disabled={!isActiveToken(token) || !canManageSelectedUser || !!actionId}
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
						</div>
					</div>

					<div class="security-panel audit-panel">
						<div class="panel-heading">
							<h3>Security events</h3>
							<div class="limit-control">
								<label class="label compact-label" for="security-event-limit">Rows</label>
								<select
									id="security-event-limit"
									class="input limit-select"
									value={securityEventLimit}
									onchange={changeSecurityEventLimit}
								>
									{#each securityEventLimitOptions as option (option)}
										<option value={option}>{option}</option>
									{/each}
								</select>
							</div>
						</div>

						{#if userSecurityEvents.length === 0}
							<p class="empty-inline">No security events found.</p>
						{:else}
							<div class="table-container compact-table">
								<table class="table security-events-table">
									<thead>
										<tr>
											<th>Time</th>
											<th>Action</th>
											<th>Actor</th>
											<th>Target</th>
											<th>Details</th>
										</tr>
									</thead>
									<tbody>
										{#each userSecurityEvents as event (event.id)}
											<tr>
												<td data-label="Time">{formatDateTime(event.created_at)}</td>
												<td data-label="Action">{formatSecurityAction(event.action)}</td>
												<td data-label="Actor" class="mono">
													{event.actor_email || event.actor_user_id || '-'}
												</td>
												<td data-label="Target" class="mono">
													{event.target_email || event.target_user_id || '-'}
												</td>
												<td data-label="Details">{formatMetadata(event.metadata)}</td>
											</tr>
										{/each}
									</tbody>
								</table>
							</div>
						{/if}
					</div>
				{/if}
			</section>
		{/if}

		<section class="admin-section">
			<div class="section-heading">
				<h2>Invite user</h2>
			</div>

			<form class="card invite-form" onsubmit={submitInvitation}>
				<div class="form-group">
					<label class="label" for="invite-email">Email</label>
					<input id="invite-email" class="input" type="email" bind:value={inviteEmail} autocomplete="email" required />
				</div>
				<div class="form-group">
					<label class="label" for="invite-role">Role</label>
					<select id="invite-role" class="input" bind:value={inviteRole}>
						{#each editableRoles as role (role)}
							<option value={role}>{formatRole(role)}</option>
						{/each}
					</select>
				</div>
				<button class="btn btn-primary invite-button" type="submit" disabled={actionId === 'invite'}> Invite </button>
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
		min-width: 1040px;
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

	.status-badge {
		display: inline-flex;
		align-items: center;
		min-height: 1.75rem;
		padding: 0.25rem 0.6rem;
		border-radius: 999px;
		background: rgba(22, 163, 74, 0.1);
		color: #15803d;
		font-size: 0.8125rem;
		font-weight: 700;
		white-space: nowrap;
	}

	.status-suspended {
		background: rgba(239, 68, 68, 0.1);
		color: var(--color-error);
	}

	.user-security-section {
		border-top: 1px solid var(--color-border);
		padding-top: 1.5rem;
	}

	.security-heading {
		align-items: flex-start;
		gap: 1rem;
	}

	.section-note {
		margin: 0.25rem 0 0;
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}

	.compact-loading {
		padding: 1rem;
	}

	.security-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 1rem;
		margin-bottom: 1rem;
	}

	.security-panel {
		border: 1px solid var(--color-border);
		border-radius: 8px;
		padding: 1rem;
		background: var(--color-surface);
		min-width: 0;
	}

	.panel-heading {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 0.75rem;
	}

	h3 {
		font-size: 1rem;
		margin: 0;
	}

	.checkbox-label,
	.limit-control {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		color: var(--color-text-muted);
		font-size: 0.875rem;
		font-weight: 600;
	}

	.checkbox-label input {
		margin: 0;
	}

	.limit-select {
		width: 88px;
	}

	.compact-label {
		margin: 0;
	}

	.empty-inline {
		margin: 0;
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}

	.compact-table {
		border: 0;
		border-radius: 0;
		box-shadow: none;
		overflow-x: auto;
	}

	.security-table {
		min-width: 720px;
	}

	.security-events-table {
		min-width: 960px;
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

		.security-grid {
			grid-template-columns: 1fr;
		}

		.security-heading,
		.panel-heading {
			align-items: stretch;
			flex-direction: column;
		}

		.invite-button {
			width: 100%;
		}
	}
</style>
