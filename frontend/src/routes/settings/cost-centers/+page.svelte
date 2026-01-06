<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { api, type CostCenter, type BudgetPeriod } from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let costCenters = $state<CostCenter[]>([]);
	let isLoading = $state(true);
	let error = $state('');
	let success = $state('');

	// Modal state
	let showModal = $state(false);
	let isEditing = $state(false);
	let isSaving = $state(false);
	let editingId = $state('');

	// Form state
	let code = $state('');
	let name = $state('');
	let description = $state('');
	let parentId = $state<string | undefined>(undefined);
	let isActive = $state(true);
	let budgetAmount = $state('');
	let budgetPeriod = $state<BudgetPeriod>('ANNUAL');

	// Delete confirmation
	let showDeleteConfirm = $state(false);
	let deletingId = $state('');
	let deletingName = $state('');
	let isDeleting = $state(false);

	onMount(async () => {
		await loadCostCenters();
	});

	async function loadCostCenters() {
		if (!tenantId) {
			error = m.settings_noTenantSelected();
			isLoading = false;
			return;
		}

		try {
			costCenters = await api.listCostCenters(tenantId);
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_loadFailed();
		} finally {
			isLoading = false;
		}
	}

	function openCreateModal() {
		isEditing = false;
		editingId = '';
		code = '';
		name = '';
		description = '';
		parentId = undefined;
		isActive = true;
		budgetAmount = '';
		budgetPeriod = 'ANNUAL';
		showModal = true;
	}

	function openEditModal(cc: CostCenter) {
		isEditing = true;
		editingId = cc.id;
		code = cc.code;
		name = cc.name;
		description = cc.description || '';
		parentId = cc.parent_id;
		isActive = cc.is_active;
		budgetAmount = cc.budget_amount || '';
		budgetPeriod = cc.budget_period;
		showModal = true;
	}

	async function saveCostCenter(e: Event) {
		e.preventDefault();
		isSaving = true;
		error = '';
		success = '';

		try {
			const data = {
				code,
				name,
				description: description || undefined,
				parent_id: parentId || undefined,
				is_active: isActive,
				budget_amount: budgetAmount || undefined,
				budget_period: budgetPeriod
			};

			if (isEditing) {
				await api.updateCostCenter(tenantId, editingId, data);
				success = m.costCenter_updated();
			} else {
				await api.createCostCenter(tenantId, data);
				success = m.costCenter_created();
			}

			showModal = false;
			await loadCostCenters();
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_saveFailed();
		} finally {
			isSaving = false;
		}
	}

	function confirmDelete(cc: CostCenter) {
		deletingId = cc.id;
		deletingName = cc.name;
		showDeleteConfirm = true;
	}

	async function deleteCostCenter() {
		isDeleting = true;
		error = '';

		try {
			await api.deleteCostCenter(tenantId, deletingId);
			success = m.costCenter_deleted();
			showDeleteConfirm = false;
			await loadCostCenters();
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_deleteFailed();
		} finally {
			isDeleting = false;
		}
	}

	function formatBudgetPeriod(period: BudgetPeriod): string {
		switch (period) {
			case 'MONTHLY':
				return m.costCenter_periodMonthly();
			case 'QUARTERLY':
				return m.costCenter_periodQuarterly();
			case 'ANNUAL':
				return m.costCenter_periodAnnual();
			default:
				return period;
		}
	}
</script>

<svelte:head>
	<title>{m.costCenter_title()}</title>
</svelte:head>

<div class="container-fluid py-4">
	<div class="d-flex justify-content-between align-items-center mb-4">
		<div>
			<a href="/settings?tenant={tenantId}" class="btn btn-secondary btn-sm mb-2">
				&larr; {m.common_back()}
			</a>
			<h1 class="h3 mb-0">{m.costCenter_title()}</h1>
			<p class="text-muted mb-0">{m.costCenter_description()}</p>
		</div>
		<button class="btn btn-primary" onclick={openCreateModal}>
			+ {m.costCenter_addNew()}
		</button>
	</div>

	{#if error}
		<div class="alert alert-danger alert-dismissible fade show" role="alert">
			{error}
			<button type="button" class="btn-close" onclick={() => (error = '')}></button>
		</div>
	{/if}

	{#if success}
		<div class="alert alert-success alert-dismissible fade show" role="alert">
			{success}
			<button type="button" class="btn-close" onclick={() => (success = '')}></button>
		</div>
	{/if}

	{#if isLoading}
		<div class="d-flex justify-content-center py-5">
			<div class="spinner-border" role="status">
				<span class="visually-hidden">{m.common_loading()}</span>
			</div>
		</div>
	{:else if costCenters.length === 0}
		<div class="card">
			<div class="card-body text-center py-5">
				<h5 class="text-muted">{m.costCenter_noCostCenters()}</h5>
				<p class="text-muted">{m.costCenter_noCostCentersDescription()}</p>
				<button class="btn btn-primary" onclick={openCreateModal}>
					{m.costCenter_addNew()}
				</button>
			</div>
		</div>
	{:else}
		<div class="card">
			<div class="table-responsive">
				<table class="table table-hover mb-0">
					<thead>
						<tr>
							<th>{m.costCenter_code()}</th>
							<th>{m.costCenter_name()}</th>
							<th>{m.costCenter_budget()}</th>
							<th>{m.costCenter_period()}</th>
							<th>{m.common_status()}</th>
							<th class="text-end">{m.common_actions()}</th>
						</tr>
					</thead>
					<tbody>
						{#each costCenters as cc}
							<tr>
								<td><code>{cc.code}</code></td>
								<td>
									{cc.name}
									{#if cc.description}
										<small class="text-muted d-block">{cc.description}</small>
									{/if}
								</td>
								<td>
									{#if cc.budget_amount}
										{parseFloat(cc.budget_amount).toLocaleString()} EUR
									{:else}
										<span class="text-muted">-</span>
									{/if}
								</td>
								<td>{formatBudgetPeriod(cc.budget_period)}</td>
								<td>
									{#if cc.is_active}
										<span class="badge bg-success">{m.common_active()}</span>
									{:else}
										<span class="badge bg-secondary">{m.common_inactive()}</span>
									{/if}
								</td>
								<td class="text-end">
									<button class="btn btn-sm btn-outline-primary me-1" onclick={() => openEditModal(cc)}>
										{m.common_edit()}
									</button>
									<button class="btn btn-sm btn-outline-danger" onclick={() => confirmDelete(cc)}>
										{m.common_delete()}
									</button>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	{/if}
</div>

<!-- Create/Edit Modal -->
{#if showModal}
	<div class="modal show d-block" tabindex="-1" style="background: rgba(0,0,0,0.5)">
		<div class="modal-dialog">
			<div class="modal-content">
				<div class="modal-header">
					<h5 class="modal-title">
						{isEditing ? m.costCenter_edit() : m.costCenter_addNew()}
					</h5>
					<button type="button" class="btn-close" onclick={() => (showModal = false)}></button>
				</div>
				<form onsubmit={saveCostCenter}>
					<div class="modal-body">
						<div class="mb-3">
							<label for="code" class="form-label">{m.costCenter_code()} *</label>
							<input
								type="text"
								class="form-control"
								id="code"
								bind:value={code}
								required
								maxlength="20"
								placeholder="CC001"
							/>
						</div>

						<div class="mb-3">
							<label for="name" class="form-label">{m.costCenter_name()} *</label>
							<input
								type="text"
								class="form-control"
								id="name"
								bind:value={name}
								required
								maxlength="200"
							/>
						</div>

						<div class="mb-3">
							<label for="description" class="form-label">{m.common_description()}</label>
							<textarea class="form-control" id="description" bind:value={description} rows="2"></textarea>
						</div>

						<div class="mb-3">
							<label for="parentId" class="form-label">{m.costCenter_parent()}</label>
							<select class="form-select" id="parentId" bind:value={parentId}>
								<option value={undefined}>{m.costCenter_noParent()}</option>
								{#each costCenters.filter((c) => c.id !== editingId) as parent}
									<option value={parent.id}>{parent.code} - {parent.name}</option>
								{/each}
							</select>
						</div>

						<div class="row">
							<div class="col-md-6 mb-3">
								<label for="budgetAmount" class="form-label">{m.costCenter_budget()}</label>
								<div class="input-group">
									<input
										type="number"
										class="form-control"
										id="budgetAmount"
										bind:value={budgetAmount}
										step="0.01"
										min="0"
									/>
									<span class="input-group-text">EUR</span>
								</div>
							</div>
							<div class="col-md-6 mb-3">
								<label for="budgetPeriod" class="form-label">{m.costCenter_period()}</label>
								<select class="form-select" id="budgetPeriod" bind:value={budgetPeriod}>
									<option value="MONTHLY">{m.costCenter_periodMonthly()}</option>
									<option value="QUARTERLY">{m.costCenter_periodQuarterly()}</option>
									<option value="ANNUAL">{m.costCenter_periodAnnual()}</option>
								</select>
							</div>
						</div>

						<div class="form-check">
							<input class="form-check-input" type="checkbox" id="isActive" bind:checked={isActive} />
							<label class="form-check-label" for="isActive">{m.common_active()}</label>
						</div>
					</div>
					<div class="modal-footer">
						<button type="button" class="btn btn-secondary" onclick={() => (showModal = false)}>
							{m.common_cancel()}
						</button>
						<button type="submit" class="btn btn-primary" disabled={isSaving}>
							{#if isSaving}
								<span class="spinner-border spinner-border-sm me-1"></span>
							{/if}
							{m.common_save()}
						</button>
					</div>
				</form>
			</div>
		</div>
	</div>
{/if}

<!-- Delete Confirmation Modal -->
{#if showDeleteConfirm}
	<div class="modal show d-block" tabindex="-1" style="background: rgba(0,0,0,0.5)">
		<div class="modal-dialog">
			<div class="modal-content">
				<div class="modal-header">
					<h5 class="modal-title">{m.costCenter_confirmDelete()}</h5>
					<button type="button" class="btn-close" onclick={() => (showDeleteConfirm = false)}></button>
				</div>
				<div class="modal-body">
					<p>{m.costCenter_deleteWarning({ name: deletingName })}</p>
				</div>
				<div class="modal-footer">
					<button type="button" class="btn btn-secondary" onclick={() => (showDeleteConfirm = false)}>
						{m.common_cancel()}
					</button>
					<button type="button" class="btn btn-danger" onclick={deleteCostCenter} disabled={isDeleting}>
						{#if isDeleting}
							<span class="spinner-border spinner-border-sm me-1"></span>
						{/if}
						{m.common_delete()}
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}
