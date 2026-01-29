<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { api, type ReminderRule, type CreateReminderRuleRequest, type AutomatedReminderResult } from '$lib/api';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let isLoading = $state(false);
	let isSaving = $state(false);
	let isTriggering = $state(false);
	let error = $state('');
	let successMessage = $state('');

	let rules = $state<ReminderRule[]>([]);
	let showCreateModal = $state(false);
	let editingRule = $state<ReminderRule | null>(null);
	let triggerResults = $state<AutomatedReminderResult[] | null>(null);

	// Form state
	let formName = $state('');
	let formTriggerType = $state<'BEFORE_DUE' | 'ON_DUE' | 'AFTER_DUE'>('AFTER_DUE');
	let formDaysOffset = $state(7);
	let formIsActive = $state(true);

	const triggerTypeLabels = {
		BEFORE_DUE: 'Before Due Date',
		ON_DUE: 'On Due Date',
		AFTER_DUE: 'After Due Date (Overdue)'
	};

	onMount(() => {
		if (tenantId) {
			loadRules();
		}
	});

	async function loadRules() {
		isLoading = true;
		error = '';
		try {
			rules = await api.listReminderRules(tenantId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load rules';
		} finally {
			isLoading = false;
		}
	}

	function openCreateModal() {
		formName = '';
		formTriggerType = 'AFTER_DUE';
		formDaysOffset = 7;
		formIsActive = true;
		editingRule = null;
		showCreateModal = true;
	}

	function openEditModal(rule: ReminderRule) {
		formName = rule.name;
		formTriggerType = rule.trigger_type;
		formDaysOffset = rule.days_offset;
		formIsActive = rule.is_active;
		editingRule = rule;
		showCreateModal = true;
	}

	function closeModal() {
		showCreateModal = false;
		editingRule = null;
	}

	async function saveRule() {
		isSaving = true;
		error = '';
		successMessage = '';

		try {
			if (editingRule) {
				await api.updateReminderRule(tenantId, editingRule.id, {
					name: formName,
					is_active: formIsActive
				});
				successMessage = 'Rule updated successfully';
			} else {
				await api.createReminderRule(tenantId, {
					name: formName,
					trigger_type: formTriggerType,
					days_offset: formDaysOffset,
					is_active: formIsActive
				});
				successMessage = 'Rule created successfully';
			}
			closeModal();
			await loadRules();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to save rule';
		} finally {
			isSaving = false;
		}
	}

	async function deleteRule(rule: ReminderRule) {
		if (!confirm(`Delete rule "${rule.name}"?`)) return;

		try {
			await api.deleteReminderRule(tenantId, rule.id);
			successMessage = 'Rule deleted';
			await loadRules();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete rule';
		}
	}

	async function toggleRule(rule: ReminderRule) {
		try {
			await api.updateReminderRule(tenantId, rule.id, {
				is_active: !rule.is_active
			});
			await loadRules();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to toggle rule';
		}
	}

	async function triggerReminders() {
		isTriggering = true;
		error = '';
		triggerResults = null;

		try {
			triggerResults = await api.triggerReminders(tenantId);
			successMessage = `Processed ${triggerResults.length} rules`;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to trigger reminders';
		} finally {
			isTriggering = false;
		}
	}

	function formatTrigger(rule: ReminderRule): string {
		if (rule.trigger_type === 'BEFORE_DUE') {
			return `${rule.days_offset} days before due`;
		} else if (rule.trigger_type === 'ON_DUE') {
			return 'On due date';
		} else {
			return `${rule.days_offset} days overdue`;
		}
	}
</script>

<svelte:head>
	<title>Reminder Settings - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>Payment Reminder Rules</h1>
		<div class="header-actions">
			<button class="btn btn-secondary" onclick={triggerReminders} disabled={isTriggering}>
				{isTriggering ? 'Processing...' : 'Run Now'}
			</button>
			<button class="btn btn-primary" onclick={openCreateModal}>
				Add Rule
			</button>
		</div>
	</div>

	{#if !tenantId}
		<div class="card empty-state">
			<p>Select a tenant from <a href="/dashboard">Dashboard</a>.</p>
		</div>
	{:else}
		{#if error}
			<div class="alert alert-error">{error}</div>
		{/if}

		{#if successMessage}
			<div class="alert alert-success">{successMessage}</div>
		{/if}

		{#if triggerResults}
			<div class="card results-card">
				<h3>Manual Run Results</h3>
				{#each triggerResults as result}
					<div class="result-item">
						<strong>{result.rule_name}</strong>:
						{result.reminders_sent} sent, {result.skipped} skipped, {result.failed} failed
						{#if result.errors?.length}
							<ul class="error-list">
								{#each result.errors as err}
									<li>{err}</li>
								{/each}
							</ul>
						{/if}
					</div>
				{/each}
				<button class="btn btn-sm btn-secondary" onclick={() => (triggerResults = null)}>Dismiss</button>
			</div>
		{/if}

		<div class="card">
			<p class="description">
				Configure when automatic payment reminders are sent to your customers. Reminders are
				processed daily at 9:00 AM.
			</p>

			{#if isLoading}
				<p>Loading...</p>
			{:else if rules.length === 0}
				<p class="empty-state">No reminder rules configured. Click "Add Rule" to create one.</p>
			{:else}
				<table class="table">
					<thead>
						<tr>
							<th>Name</th>
							<th>Trigger</th>
							<th>Template</th>
							<th>Status</th>
							<th class="text-right">Actions</th>
						</tr>
					</thead>
					<tbody>
						{#each rules as rule}
							<tr class:inactive={!rule.is_active}>
								<td>{rule.name}</td>
								<td>{formatTrigger(rule)}</td>
								<td><code>{rule.email_template_type}</code></td>
								<td>
									<button
										class="status-toggle"
										class:active={rule.is_active}
										onclick={() => toggleRule(rule)}
									>
										{rule.is_active ? 'Active' : 'Inactive'}
									</button>
								</td>
								<td class="text-right">
									<button class="btn btn-sm btn-secondary" onclick={() => openEditModal(rule)}>
										Edit
									</button>
									<button class="btn btn-sm btn-danger" onclick={() => deleteRule(rule)}>
										Delete
									</button>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>
	{/if}
</div>

{#if showCreateModal}
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="modal-overlay"
		onclick={closeModal}
		onkeydown={(e) => e.key === 'Escape' && closeModal()}
		role="presentation"
	>
		<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
		<div
			class="modal"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
			tabindex="-1"
		>
			<div class="modal-header">
				<h2>{editingRule ? 'Edit Rule' : 'Create Rule'}</h2>
				<button class="btn-close" onclick={closeModal}>&times;</button>
			</div>

			<div class="modal-body">
				<div class="form-group">
					<label for="name">Rule Name</label>
					<input type="text" id="name" bind:value={formName} placeholder="e.g., 7 Days Overdue" />
				</div>

				{#if !editingRule}
					<div class="form-group">
						<label for="triggerType">Trigger Type</label>
						<select id="triggerType" bind:value={formTriggerType}>
							<option value="BEFORE_DUE">Before Due Date</option>
							<option value="ON_DUE">On Due Date</option>
							<option value="AFTER_DUE">After Due Date (Overdue)</option>
						</select>
					</div>

					{#if formTriggerType !== 'ON_DUE'}
						<div class="form-group">
							<label for="daysOffset">Days Offset</label>
							<input type="number" id="daysOffset" bind:value={formDaysOffset} min="1" max="365" />
							<small>
								{formTriggerType === 'BEFORE_DUE'
									? `Reminder sent ${formDaysOffset} days before due date`
									: `Reminder sent ${formDaysOffset} days after due date`}
							</small>
						</div>
					{/if}
				{/if}

				<div class="form-group">
					<label class="checkbox-label">
						<input type="checkbox" bind:checked={formIsActive} />
						Active
					</label>
				</div>
			</div>

			<div class="modal-footer">
				<button class="btn btn-secondary" onclick={closeModal} disabled={isSaving}>
					Cancel
				</button>
				<button class="btn btn-primary" onclick={saveRule} disabled={isSaving || !formName}>
					{isSaving ? 'Saving...' : 'Save'}
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1.5rem;
	}

	.header-actions {
		display: flex;
		gap: 0.5rem;
	}

	.description {
		color: var(--color-text-muted);
		margin-bottom: 1.5rem;
	}

	.table {
		width: 100%;
		border-collapse: collapse;
	}

	.table th,
	.table td {
		padding: 0.75rem;
		border-bottom: 1px solid var(--color-border);
		text-align: left;
	}

	.table th {
		font-weight: 600;
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}

	.text-right {
		text-align: right;
	}

	.inactive {
		opacity: 0.6;
	}

	.status-toggle {
		padding: 0.25rem 0.5rem;
		border: none;
		border-radius: 0.25rem;
		cursor: pointer;
		font-size: 0.75rem;
		background: var(--color-error);
		color: white;
	}

	.status-toggle.active {
		background: var(--color-success);
	}

	.results-card {
		margin-bottom: 1rem;
		background: var(--color-bg);
	}

	.results-card h3 {
		margin-bottom: 0.5rem;
	}

	.result-item {
		padding: 0.5rem 0;
		border-bottom: 1px solid var(--color-border);
	}

	.error-list {
		color: var(--color-error);
		font-size: 0.875rem;
		margin: 0.25rem 0 0 1rem;
	}

	.btn-sm {
		padding: 0.25rem 0.5rem;
		font-size: 0.875rem;
	}

	.btn-danger {
		background: var(--color-error);
		color: white;
	}

	.empty-state {
		text-align: center;
		padding: 2rem;
		color: var(--color-text-muted);
	}

	code {
		background: var(--color-bg);
		padding: 0.125rem 0.375rem;
		border-radius: 0.25rem;
		font-size: 0.875rem;
	}

	/* Modal styles */
	.modal-overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.5);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 1000;
	}

	.modal {
		background: var(--color-surface);
		border-radius: 0.5rem;
		max-width: 500px;
		width: 100%;
		margin: 1rem;
	}

	.modal-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 1rem 1.5rem;
		border-bottom: 1px solid var(--color-border);
	}

	.modal-header h2 {
		margin: 0;
		font-size: 1.25rem;
	}

	.btn-close {
		background: none;
		border: none;
		font-size: 1.5rem;
		cursor: pointer;
		color: var(--color-text-muted);
	}

	.modal-body {
		padding: 1.5rem;
	}

	.modal-footer {
		display: flex;
		justify-content: flex-end;
		gap: 0.75rem;
		padding: 1rem 1.5rem;
		border-top: 1px solid var(--color-border);
	}

	.form-group {
		margin-bottom: 1rem;
	}

	.form-group label {
		display: block;
		margin-bottom: 0.25rem;
		font-weight: 500;
	}

	.form-group input[type='text'],
	.form-group input[type='number'],
	.form-group select {
		width: 100%;
		padding: 0.5rem;
		border: 1px solid var(--color-border);
		border-radius: 0.25rem;
	}

	.form-group small {
		display: block;
		margin-top: 0.25rem;
		color: var(--color-text-muted);
		font-size: 0.75rem;
	}

	.checkbox-label {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		cursor: pointer;
	}

	.checkbox-label input {
		width: auto;
	}

	.alert-success {
		background: rgba(76, 175, 80, 0.1);
		color: #2e7d32;
		padding: 0.75rem 1rem;
		border-radius: 0.25rem;
		margin-bottom: 1rem;
	}

	.alert-error {
		background: rgba(244, 67, 54, 0.1);
		color: #c62828;
		padding: 0.75rem 1rem;
		border-radius: 0.25rem;
		margin-bottom: 1rem;
	}
</style>
