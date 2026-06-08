<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import {
		api,
		type BudgetPeriod,
		type CostAllocation,
		type CostCenter,
		type CreateCostAllocationRequest,
		type JournalEntry,
		type JournalEntryLine,
	} from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';

	type JournalLineOption = {
		id: string;
		label: string;
	};

	function todayISODate(): string {
		return new Date().toISOString().split('T')[0];
	}

	function valueToString(value: unknown): string {
		if (value === null || value === undefined) return '';
		if (typeof value === 'string') return value;
		if (typeof value === 'number') return String(value);
		if (typeof value === 'object' && 'toString' in value)
			return value.toString();
		return String(value);
	}

	function formatMoney(value: unknown): string {
		const raw = valueToString(value);
		const parsed = Number(raw);
		if (Number.isNaN(parsed)) return raw || '-';
		return parsed.toLocaleString(undefined, {
			minimumFractionDigits: 2,
			maximumFractionDigits: 2,
		});
	}

	function formatDate(value: string): string {
		if (!value) return '-';
		const parsed = new Date(value.includes('T') ? value : `${value}T00:00:00Z`);
		if (Number.isNaN(parsed.getTime())) return value;
		return new Intl.DateTimeFormat(undefined, {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			timeZone: 'UTC',
		}).format(parsed);
	}

	function formatJournalLineAmount(line: JournalEntryLine): string {
		const debit = valueToString(line.debit_amount);
		const credit = valueToString(line.credit_amount);
		if (Number(debit) > 0) return `Dr ${formatMoney(debit)} ${line.currency}`;
		return `Cr ${formatMoney(credit)} ${line.currency}`;
	}

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let costCenters = $state<CostCenter[]>([]);
	let costAllocations = $state<CostAllocation[]>([]);
	let journalEntries = $state<JournalEntry[]>([]);
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

	// Allocation form state
	let allocationCostCenterId = $state('');
	let allocationJournalLineId = $state('');
	let allocationAmount = $state('');
	let allocationPercentage = $state('');
	let allocationDate = $state(todayISODate());
	let allocationNotes = $state('');
	let isCreatingAllocation = $state(false);

	// Delete confirmation
	let showDeleteConfirm = $state(false);
	let deletingId = $state('');
	let deletingName = $state('');
	let isDeleting = $state(false);

	let activeCostCenters = $derived(costCenters.filter((cc) => cc.is_active));
	let journalLineOptions = $derived.by<JournalLineOption[]>(() =>
		journalEntries.flatMap((entry) =>
			(entry.lines || []).map((line) => {
				const accountLabel = line.account
					? `${line.account.code} ${line.account.name}`
					: line.account_id;
				const lineDescription = line.description || entry.description;

				return {
					id: line.id,
					label: `${entry.entry_number} - ${formatDate(entry.entry_date)} - ${accountLabel} - ${lineDescription} - ${formatJournalLineAmount(line)}`,
				};
			}),
		),
	);

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
			const [loadedCostCenters, loadedAllocations, loadedJournalEntries] =
				await Promise.all([
					api.listCostCenters(tenantId),
					api.listCostAllocations(tenantId),
					api.listJournalEntries(tenantId, 25),
				]);
			costCenters = loadedCostCenters;
			costAllocations = loadedAllocations;
			journalEntries = loadedJournalEntries;
			allocationCostCenterId =
				allocationCostCenterId ||
				loadedCostCenters.find((cc) => cc.is_active)?.id ||
				loadedCostCenters[0]?.id ||
				'';
			allocationJournalLineId =
				allocationJournalLineId ||
				loadedJournalEntries.flatMap((entry) => entry.lines || [])[0]?.id ||
				'';
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
				budget_period: budgetPeriod,
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

	async function loadCostAllocations() {
		costAllocations = await api.listCostAllocations(tenantId);
	}

	async function saveCostAllocation(e: Event) {
		e.preventDefault();
		isCreatingAllocation = true;
		error = '';
		success = '';

		const request: CreateCostAllocationRequest = {
			cost_center_id: allocationCostCenterId,
			journal_entry_line_id: allocationJournalLineId,
			amount: allocationAmount,
			allocation_date: `${allocationDate}T00:00:00Z`,
			notes: allocationNotes || undefined,
		};

		if (allocationPercentage.trim()) {
			request.allocation_percentage = allocationPercentage;
		}

		try {
			await api.createCostAllocation(tenantId, request);
			success = m.costCenter_allocationCreated();
			allocationAmount = '';
			allocationPercentage = '';
			allocationNotes = '';
			allocationDate = todayISODate();
			await loadCostAllocations();
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_saveFailed();
		} finally {
			isCreatingAllocation = false;
		}
	}

	function formatAllocationCostCenter(allocation: CostAllocation): string {
		if (allocation.cost_center_code || allocation.cost_center_name) {
			return [allocation.cost_center_code, allocation.cost_center_name]
				.filter(Boolean)
				.join(' - ');
		}

		const costCenter = costCenters.find(
			(cc) => cc.id === allocation.cost_center_id,
		);
		return costCenter
			? `${costCenter.code} - ${costCenter.name}`
			: allocation.cost_center_id;
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
			<a
				href="/settings?tenant={tenantId}"
				class="btn btn-secondary btn-sm mb-2"
			>
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
			<button
				type="button"
				class="btn-close"
				aria-label={m.common_close()}
				onclick={() => (error = '')}
			></button>
		</div>
	{/if}

	{#if success}
		<div class="alert alert-success alert-dismissible fade show" role="alert">
			{success}
			<button
				type="button"
				class="btn-close"
				aria-label={m.common_close()}
				onclick={() => (success = '')}
			></button>
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
						{#each costCenters as cc (cc.id)}
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
										<span class="badge bg-secondary">{m.common_inactive()}</span
										>
									{/if}
								</td>
								<td class="text-end">
									<button
										class="btn btn-sm btn-outline-primary me-1"
										onclick={() => openEditModal(cc)}
									>
										{m.common_edit()}
									</button>
									<button
										class="btn btn-sm btn-outline-danger"
										onclick={() => confirmDelete(cc)}
									>
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

	{#if !isLoading}
		<div class="card mt-4">
			<div class="card-body">
				<div class="d-flex justify-content-between align-items-start mb-3">
					<div>
						<h2 class="h5 mb-1">{m.costCenter_allocationsTitle()}</h2>
						<p class="text-muted mb-0">
							{m.costCenter_allocationsDescription()}
						</p>
					</div>
				</div>

				<form class="allocation-form mb-4" onsubmit={saveCostAllocation}>
					<div class="row g-3 align-items-end">
						<div class="col-lg-3 col-md-6">
							<label for="allocationCostCenter" class="form-label"
								>{m.costCenter_name()} *</label
							>
							<select
								class="form-select"
								id="allocationCostCenter"
								bind:value={allocationCostCenterId}
								required
								disabled={activeCostCenters.length === 0}
							>
								{#each activeCostCenters as costCenter (costCenter.id)}
									<option value={costCenter.id}
										>{costCenter.code} - {costCenter.name}</option
									>
								{/each}
							</select>
						</div>

						<div class="col-lg-4 col-md-6">
							<label for="allocationJournalLine" class="form-label"
								>{m.costCenter_journalLine()} *</label
							>
							<select
								class="form-select"
								id="allocationJournalLine"
								bind:value={allocationJournalLineId}
								required
								disabled={journalLineOptions.length === 0}
							>
								{#each journalLineOptions as line (line.id)}
									<option value={line.id}>{line.label}</option>
								{/each}
							</select>
						</div>

						<div class="col-lg-2 col-md-4">
							<label for="allocationAmount" class="form-label"
								>{m.common_amount()} *</label
							>
							<input
								type="number"
								class="form-control"
								id="allocationAmount"
								bind:value={allocationAmount}
								min="0.01"
								step="0.01"
								required
							/>
						</div>

						<div class="col-lg-2 col-md-4">
							<label for="allocationDate" class="form-label"
								>{m.common_date()} *</label
							>
							<input
								type="date"
								class="form-control"
								id="allocationDate"
								bind:value={allocationDate}
								required
							/>
						</div>

						<div class="col-lg-1 col-md-4">
							<label for="allocationPercentage" class="form-label"
								>{m.costCenter_percentage()}</label
							>
							<input
								type="number"
								class="form-control"
								id="allocationPercentage"
								bind:value={allocationPercentage}
								min="0"
								max="100"
								step="0.01"
							/>
						</div>

						<div class="col-lg-9">
							<label for="allocationNotes" class="form-label"
								>{m.costCenter_notes()}</label
							>
							<input
								type="text"
								class="form-control"
								id="allocationNotes"
								bind:value={allocationNotes}
								maxlength="500"
							/>
						</div>

						<div class="col-lg-3 d-grid">
							<button
								type="submit"
								class="btn btn-primary"
								disabled={isCreatingAllocation ||
									activeCostCenters.length === 0 ||
									journalLineOptions.length === 0}
							>
								{#if isCreatingAllocation}
									<span class="spinner-border spinner-border-sm me-1"></span>
								{/if}
								{m.costCenter_createAllocation()}
							</button>
						</div>
					</div>
				</form>

				{#if activeCostCenters.length === 0}
					<p class="text-muted mb-0">
						{m.costCenter_allocationNeedsCostCenter()}
					</p>
				{:else if journalLineOptions.length === 0}
					<p class="text-muted mb-0">
						{m.costCenter_allocationNeedsJournalLine()}
					</p>
				{:else if costAllocations.length === 0}
					<p class="text-muted mb-0">{m.costCenter_noAllocations()}</p>
				{:else}
					<div class="table-responsive">
						<table class="table table-sm table-hover mb-0">
							<thead>
								<tr>
									<th>{m.costCenter_name()}</th>
									<th>{m.costCenter_journalLine()}</th>
									<th>{m.common_amount()}</th>
									<th>{m.costCenter_percentage()}</th>
									<th>{m.common_date()}</th>
									<th>{m.costCenter_notes()}</th>
								</tr>
							</thead>
							<tbody>
								{#each costAllocations as allocation (allocation.id)}
									<tr>
										<td>{formatAllocationCostCenter(allocation)}</td>
										<td><code>{allocation.journal_entry_line_id}</code></td>
										<td>{formatMoney(allocation.amount)}</td>
										<td>
											{#if allocation.allocation_percentage}
												{formatMoney(allocation.allocation_percentage)}%
											{:else}
												<span class="text-muted">-</span>
											{/if}
										</td>
										<td>{formatDate(allocation.allocation_date)}</td>
										<td>{allocation.notes || '-'}</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>

<!-- Create/Edit Modal -->
{#if showModal}
	<div
		class="modal show d-block"
		tabindex="-1"
		style="background: rgba(0,0,0,0.5)"
	>
		<div class="modal-dialog">
			<div class="modal-content">
				<div class="modal-header">
					<h5 class="modal-title">
						{isEditing ? m.costCenter_edit() : m.costCenter_addNew()}
					</h5>
					<button
						type="button"
						class="btn-close"
						aria-label={m.common_close()}
						onclick={() => (showModal = false)}
					></button>
				</div>
				<form onsubmit={saveCostCenter}>
					<div class="modal-body">
						<div class="mb-3">
							<label for="code" class="form-label"
								>{m.costCenter_code()} *</label
							>
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
							<label for="name" class="form-label"
								>{m.costCenter_name()} *</label
							>
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
							<label for="description" class="form-label"
								>{m.common_description()}</label
							>
							<textarea
								class="form-control"
								id="description"
								bind:value={description}
								rows="2"
							></textarea>
						</div>

						<div class="mb-3">
							<label for="parentId" class="form-label"
								>{m.costCenter_parent()}</label
							>
							<select class="form-select" id="parentId" bind:value={parentId}>
								<option value={undefined}>{m.costCenter_noParent()}</option>
								{#each costCenters.filter((c) => c.id !== editingId) as parent (parent.id)}
									<option value={parent.id}
										>{parent.code} - {parent.name}</option
									>
								{/each}
							</select>
						</div>

						<div class="row">
							<div class="col-md-6 mb-3">
								<label for="budgetAmount" class="form-label"
									>{m.costCenter_budget()}</label
								>
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
								<label for="budgetPeriod" class="form-label"
									>{m.costCenter_period()}</label
								>
								<select
									class="form-select"
									id="budgetPeriod"
									bind:value={budgetPeriod}
								>
									<option value="MONTHLY">{m.costCenter_periodMonthly()}</option
									>
									<option value="QUARTERLY"
										>{m.costCenter_periodQuarterly()}</option
									>
									<option value="ANNUAL">{m.costCenter_periodAnnual()}</option>
								</select>
							</div>
						</div>

						<div class="form-check">
							<input
								class="form-check-input"
								type="checkbox"
								id="isActive"
								bind:checked={isActive}
							/>
							<label class="form-check-label" for="isActive"
								>{m.common_active()}</label
							>
						</div>
					</div>
					<div class="modal-footer">
						<button
							type="button"
							class="btn btn-secondary"
							onclick={() => (showModal = false)}
						>
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
	<div
		class="modal show d-block"
		tabindex="-1"
		style="background: rgba(0,0,0,0.5)"
	>
		<div class="modal-dialog">
			<div class="modal-content">
				<div class="modal-header">
					<h5 class="modal-title">{m.costCenter_confirmDelete()}</h5>
					<button
						type="button"
						class="btn-close"
						aria-label={m.common_close()}
						onclick={() => (showDeleteConfirm = false)}
					></button>
				</div>
				<div class="modal-body">
					<p>{m.costCenter_deleteWarning({ name: deletingName })}</p>
				</div>
				<div class="modal-footer">
					<button
						type="button"
						class="btn btn-secondary"
						onclick={() => (showDeleteConfirm = false)}
					>
						{m.common_cancel()}
					</button>
					<button
						type="button"
						class="btn btn-danger"
						onclick={deleteCostCenter}
						disabled={isDeleting}
					>
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
