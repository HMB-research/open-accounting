<script lang="ts">
	import { page } from '$app/stores';
	import { api, type FixedAsset, type AssetStatus, type AssetCategory, type DepreciationEntry, type DepreciationMethod, type DisposalMethod } from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';
	import DateRangeFilter from '$lib/components/DateRangeFilter.svelte';

	let assets = $state<FixedAsset[]>([]);
	let categories = $state<AssetCategory[]>([]);
	let isLoading = $state(true);
	let error = $state('');
	let showCreateAsset = $state(false);
	let showDisposeModal = $state(false);
	let showDepreciationHistory = $state(false);
	let filterStatus = $state<AssetStatus | ''>('');
	let filterFromDate = $state('');
	let filterToDate = $state('');
	let selectedAsset = $state<FixedAsset | null>(null);
	let depreciationHistory = $state<DepreciationEntry[]>([]);

	// New asset form
	let newName = $state('');
	let newDescription = $state('');
	let newCategoryId = $state('');
	let newPurchaseDate = $state(new Date().toISOString().split('T')[0]);
	let newPurchaseCost = $state('');
	let newSerialNumber = $state('');
	let newLocation = $state('');
	let newDepreciationMethod = $state<DepreciationMethod>('STRAIGHT_LINE');
	let newUsefulLifeMonths = $state(60);
	let newResidualValue = $state('0');
	let newDepreciationStartDate = $state('');

	// Dispose form
	let disposeDate = $state(new Date().toISOString().split('T')[0]);
	let disposeMethod = $state<DisposalMethod>('SOLD');
	let disposeProceeds = $state('');
	let disposeNotes = $state('');

	$effect(() => {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadData(tenantId);
		}
	});

	async function loadData(tenantId: string) {
		isLoading = true;
		error = '';

		try {
			const [assetData, categoryData] = await Promise.all([
				api.listAssets(tenantId, {
					status: filterStatus || undefined,
					from_date: filterFromDate || undefined,
					to_date: filterToDate || undefined
				}),
				api.listAssetCategories(tenantId)
			]);
			assets = assetData;
			categories = categoryData;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load data';
		} finally {
			isLoading = false;
		}
	}

	async function createAsset(e: Event) {
		e.preventDefault();
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const asset = await api.createAsset(tenantId, {
				name: newName,
				description: newDescription || undefined,
				category_id: newCategoryId || undefined,
				purchase_date: newPurchaseDate,
				purchase_cost: newPurchaseCost,
				serial_number: newSerialNumber || undefined,
				location: newLocation || undefined,
				depreciation_method: newDepreciationMethod,
				useful_life_months: newUsefulLifeMonths,
				residual_value: newResidualValue || '0',
				depreciation_start_date: newDepreciationStartDate || undefined
			});
			assets = [asset, ...assets];
			showCreateAsset = false;
			resetForm();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create asset';
		}
	}

	function resetForm() {
		newName = '';
		newDescription = '';
		newCategoryId = '';
		newPurchaseDate = new Date().toISOString().split('T')[0];
		newPurchaseCost = '';
		newSerialNumber = '';
		newLocation = '';
		newDepreciationMethod = 'STRAIGHT_LINE';
		newUsefulLifeMonths = 60;
		newResidualValue = '0';
		newDepreciationStartDate = '';
	}

	async function handleFilter() {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadData(tenantId);
		}
	}

	async function activateAsset(assetId: string) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			await api.activateAsset(tenantId, assetId);
			loadData(tenantId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to activate asset';
		}
	}

	function openDisposeModal(asset: FixedAsset) {
		selectedAsset = asset;
		disposeDate = new Date().toISOString().split('T')[0];
		disposeMethod = 'SOLD';
		disposeProceeds = '';
		disposeNotes = '';
		showDisposeModal = true;
	}

	async function disposeAsset(e: Event) {
		e.preventDefault();
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId || !selectedAsset) return;

		try {
			await api.disposeAsset(tenantId, selectedAsset.id, {
				disposal_date: disposeDate,
				disposal_method: disposeMethod,
				disposal_proceeds: disposeProceeds || undefined,
				disposal_notes: disposeNotes || undefined
			});
			showDisposeModal = false;
			selectedAsset = null;
			loadData(tenantId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to dispose asset';
		}
	}

	async function openDepreciationHistory(asset: FixedAsset) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		selectedAsset = asset;
		try {
			depreciationHistory = await api.getDepreciationHistory(tenantId, asset.id);
			showDepreciationHistory = true;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load depreciation history';
		}
	}

	async function recordDepreciation(assetId: string) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		const now = new Date();
		const periodStart = new Date(now.getFullYear(), now.getMonth(), 1).toISOString().split('T')[0];
		const periodEnd = new Date(now.getFullYear(), now.getMonth() + 1, 0).toISOString().split('T')[0];

		try {
			await api.recordDepreciation(tenantId, assetId, {
				period_start: periodStart,
				period_end: periodEnd
			});
			loadData(tenantId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to record depreciation';
		}
	}

	async function deleteAsset(assetId: string) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		if (!confirm(m.assets_confirmDelete())) return;

		try {
			await api.deleteAsset(tenantId, assetId);
			assets = assets.filter(a => a.id !== assetId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete asset';
		}
	}

	function getStatusLabel(status: AssetStatus): string {
		switch (status) {
			case 'DRAFT': return m.assets_statusDraft();
			case 'ACTIVE': return m.assets_statusActive();
			case 'DISPOSED': return m.assets_statusDisposed();
			case 'SOLD': return m.assets_statusSold();
		}
	}

	function getMethodLabel(method: DepreciationMethod): string {
		switch (method) {
			case 'STRAIGHT_LINE': return m.assets_methodStraightLine();
			case 'DECLINING_BALANCE': return m.assets_methodDecliningBalance();
			case 'UNITS_OF_PRODUCTION': return m.assets_methodUnitsOfProduction();
		}
	}

	function getDisposalMethodLabel(method: DisposalMethod): string {
		switch (method) {
			case 'SOLD': return m.assets_disposalSold();
			case 'SCRAPPED': return m.assets_disposalScrapped();
			case 'DONATED': return m.assets_disposalDonated();
			case 'LOST': return m.assets_disposalLost();
		}
	}

	const statusBadgeClass: Record<AssetStatus, string> = {
		DRAFT: 'badge-draft',
		ACTIVE: 'badge-active',
		DISPOSED: 'badge-disposed',
		SOLD: 'badge-sold'
	};

	function formatCurrency(value: Decimal | number | string): string {
		const num = typeof value === 'object' && 'toFixed' in value ? value.toNumber() : Number(value);
		return new Intl.NumberFormat('et-EE', {
			style: 'currency',
			currency: 'EUR'
		}).format(num);
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('et-EE');
	}

	function getCategoryName(categoryId: string | undefined): string {
		if (!categoryId) return '-';
		const category = categories.find((c) => c.id === categoryId);
		return category?.name || '-';
	}
</script>

<svelte:head>
	<title>{m.assets_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="page-header">
		<h1>{m.assets_title()}</h1>
		<div class="page-actions">
			<button class="btn btn-primary" onclick={() => (showCreateAsset = true)}>
				+ {m.assets_newAsset()}
			</button>
		</div>
	</div>

	<div class="filters card">
		<div class="filter-row">
			<select class="input" bind:value={filterStatus} onchange={handleFilter}>
				<option value="">{m.assets_allAssets()}</option>
				<option value="DRAFT">{m.assets_statusDraft()}</option>
				<option value="ACTIVE">{m.assets_statusActive()}</option>
				<option value="DISPOSED">{m.assets_statusDisposed()}</option>
				<option value="SOLD">{m.assets_statusSold()}</option>
			</select>
			<DateRangeFilter
				bind:fromDate={filterFromDate}
				bind:toDate={filterToDate}
				onchange={handleFilter}
			/>
		</div>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<p>{m.common_loading()}</p>
	{:else if assets.length === 0}
		<div class="empty-state card">
			<p>{m.assets_noAssets()}</p>
		</div>
	{:else}
		<div class="card">
			<div class="table-container">
				<table class="table table-mobile-cards">
					<thead>
						<tr>
							<th>{m.assets_assetNumber()}</th>
							<th>{m.common_name()}</th>
							<th>{m.common_status()}</th>
							<th class="hide-mobile">{m.assets_category()}</th>
							<th class="hide-mobile">{m.assets_purchaseDate()}</th>
							<th class="text-right">{m.assets_purchaseCost()}</th>
							<th class="text-right hide-mobile">{m.assets_bookValue()}</th>
							<th class="hide-mobile">{m.common_actions()}</th>
						</tr>
					</thead>
					<tbody>
						{#each assets as asset}
							<tr>
								<td class="number" data-label={m.assets_assetNumber()}>{asset.asset_number}</td>
								<td data-label={m.common_name()}>{asset.name}</td>
								<td data-label={m.common_status()}>
									<span class="badge {statusBadgeClass[asset.status]}">
										{getStatusLabel(asset.status)}
									</span>
								</td>
								<td class="hide-mobile" data-label={m.assets_category()}>{getCategoryName(asset.category_id)}</td>
								<td class="hide-mobile" data-label={m.assets_purchaseDate()}>{formatDate(asset.purchase_date)}</td>
								<td class="amount text-right" data-label={m.assets_purchaseCost()}>{formatCurrency(asset.purchase_cost)}</td>
								<td class="amount text-right hide-mobile" data-label={m.assets_bookValue()}>{formatCurrency(asset.book_value)}</td>
								<td class="actions hide-mobile" data-label={m.common_actions()}>
									{#if asset.status === 'DRAFT'}
										<button class="btn btn-small btn-success" onclick={() => activateAsset(asset.id)} title={m.assets_activate()}>
											{m.assets_activate()}
										</button>
										<button class="btn btn-small btn-danger" onclick={() => deleteAsset(asset.id)} title={m.common_delete()}>
											{m.common_delete()}
										</button>
									{:else if asset.status === 'ACTIVE'}
										<button class="btn btn-small" onclick={() => recordDepreciation(asset.id)} title={m.assets_depreciate()}>
											{m.assets_depreciate()}
										</button>
										<button class="btn btn-small" onclick={() => openDepreciationHistory(asset)} title={m.assets_viewHistory()}>
											{m.assets_viewHistory()}
										</button>
										<button class="btn btn-small btn-warning" onclick={() => openDisposeModal(asset)} title={m.assets_dispose()}>
											{m.assets_dispose()}
										</button>
									{:else}
										<button class="btn btn-small" onclick={() => openDepreciationHistory(asset)} title={m.assets_viewHistory()}>
											{m.assets_viewHistory()}
										</button>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	{/if}
</div>

{#if showCreateAsset}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showCreateAsset = false)} role="presentation">
		<div class="modal card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="create-asset-title" tabindex="-1">
			<h2 id="create-asset-title">{m.assets_newAsset()}</h2>
			<form onsubmit={createAsset}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="name">{m.common_name()} *</label>
						<input class="input" type="text" id="name" bind:value={newName} required />
					</div>
					<div class="form-group">
						<label class="label" for="category">{m.assets_category()}</label>
						<select class="input" id="category" bind:value={newCategoryId}>
							<option value="">-</option>
							{#each categories as category}
								<option value={category.id}>{category.name}</option>
							{/each}
						</select>
					</div>
				</div>

				<div class="form-group">
					<label class="label" for="description">{m.common_description()}</label>
					<textarea class="input" id="description" bind:value={newDescription} rows="2"></textarea>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="purchase-date">{m.assets_purchaseDate()} *</label>
						<input class="input" type="date" id="purchase-date" bind:value={newPurchaseDate} required />
					</div>
					<div class="form-group">
						<label class="label" for="purchase-cost">{m.assets_purchaseCost()} *</label>
						<input class="input" type="number" step="0.01" min="0" id="purchase-cost" bind:value={newPurchaseCost} required />
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="serial-number">{m.assets_serialNumber()}</label>
						<input class="input" type="text" id="serial-number" bind:value={newSerialNumber} />
					</div>
					<div class="form-group">
						<label class="label" for="location">{m.assets_location()}</label>
						<input class="input" type="text" id="location" bind:value={newLocation} />
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="depreciation-method">{m.assets_depreciationMethod()}</label>
						<select class="input" id="depreciation-method" bind:value={newDepreciationMethod}>
							<option value="STRAIGHT_LINE">{m.assets_methodStraightLine()}</option>
							<option value="DECLINING_BALANCE">{m.assets_methodDecliningBalance()}</option>
							<option value="UNITS_OF_PRODUCTION">{m.assets_methodUnitsOfProduction()}</option>
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="useful-life">{m.assets_usefulLife()}</label>
						<input class="input" type="number" min="1" id="useful-life" bind:value={newUsefulLifeMonths} />
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="residual-value">{m.assets_residualValue()}</label>
						<input class="input" type="number" step="0.01" min="0" id="residual-value" bind:value={newResidualValue} />
					</div>
					<div class="form-group">
						<label class="label" for="depreciation-start">{m.assets_depreciationStartDate()}</label>
						<input class="input" type="date" id="depreciation-start" bind:value={newDepreciationStartDate} />
					</div>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={() => (showCreateAsset = false)}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary">{m.assets_createAsset()}</button>
				</div>
			</form>
		</div>
	</div>
{/if}

{#if showDisposeModal && selectedAsset}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showDisposeModal = false)} role="presentation">
		<div class="modal card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="dispose-asset-title" tabindex="-1">
			<h2 id="dispose-asset-title">{m.assets_dispose()}: {selectedAsset.name}</h2>
			<form onsubmit={disposeAsset}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="dispose-date">{m.assets_disposalDate()} *</label>
						<input class="input" type="date" id="dispose-date" bind:value={disposeDate} required />
					</div>
					<div class="form-group">
						<label class="label" for="dispose-method">{m.assets_disposalMethod()} *</label>
						<select class="input" id="dispose-method" bind:value={disposeMethod} required>
							<option value="SOLD">{m.assets_disposalSold()}</option>
							<option value="SCRAPPED">{m.assets_disposalScrapped()}</option>
							<option value="DONATED">{m.assets_disposalDonated()}</option>
							<option value="LOST">{m.assets_disposalLost()}</option>
						</select>
					</div>
				</div>

				{#if disposeMethod === 'SOLD'}
					<div class="form-group">
						<label class="label" for="dispose-proceeds">{m.assets_disposalProceeds()}</label>
						<input class="input" type="number" step="0.01" min="0" id="dispose-proceeds" bind:value={disposeProceeds} />
					</div>
				{/if}

				<div class="form-group">
					<label class="label" for="dispose-notes">{m.assets_disposalNotes()}</label>
					<textarea class="input" id="dispose-notes" bind:value={disposeNotes} rows="2"></textarea>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={() => (showDisposeModal = false)}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-warning">{m.assets_dispose()}</button>
				</div>
			</form>
		</div>
	</div>
{/if}

{#if showDepreciationHistory && selectedAsset}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showDepreciationHistory = false)} role="presentation">
		<div class="modal card modal-wide" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="depreciation-history-title" tabindex="-1">
			<h2 id="depreciation-history-title">{m.assets_depreciationHistory()}: {selectedAsset.name}</h2>

			<div class="asset-summary">
				<div class="summary-item">
					<span class="summary-label">{m.assets_purchaseCost()}:</span>
					<span class="summary-value">{formatCurrency(selectedAsset.purchase_cost)}</span>
				</div>
				<div class="summary-item">
					<span class="summary-label">{m.assets_accumulatedDepreciation()}:</span>
					<span class="summary-value">{formatCurrency(selectedAsset.accumulated_depreciation)}</span>
				</div>
				<div class="summary-item">
					<span class="summary-label">{m.assets_bookValue()}:</span>
					<span class="summary-value">{formatCurrency(selectedAsset.book_value)}</span>
				</div>
			</div>

			{#if depreciationHistory.length === 0}
				<p class="empty-history">{m.assets_noDepreciation()}</p>
			{:else}
				<div class="table-container">
					<table class="table">
						<thead>
							<tr>
								<th>{m.common_date()}</th>
								<th>{m.assets_periodStart()}</th>
								<th>{m.assets_periodEnd()}</th>
								<th class="text-right">{m.assets_depreciationAmount()}</th>
								<th class="text-right">{m.assets_bookValueAfter()}</th>
							</tr>
						</thead>
						<tbody>
							{#each depreciationHistory as entry}
								<tr>
									<td>{formatDate(entry.depreciation_date)}</td>
									<td>{formatDate(entry.period_start)}</td>
									<td>{formatDate(entry.period_end)}</td>
									<td class="amount text-right">{formatCurrency(entry.depreciation_amount)}</td>
									<td class="amount text-right">{formatCurrency(entry.book_value_after)}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}

			<div class="modal-actions">
				<button type="button" class="btn btn-secondary" onclick={() => (showDepreciationHistory = false)}>
					{m.common_close()}
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.page-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1.5rem;
	}

	h1 {
		font-size: 1.75rem;
	}

	.filters {
		margin-bottom: 1.5rem;
		padding: 1rem;
	}

	.filter-row {
		display: flex;
		gap: 1rem;
		flex-wrap: wrap;
	}

	.number {
		font-family: var(--font-mono);
		font-weight: 500;
	}

	.amount {
		font-family: var(--font-mono);
	}

	.text-right {
		text-align: right;
	}

	.badge-draft {
		background: #f3f4f6;
		color: #6b7280;
	}

	.badge-active {
		background: #dcfce7;
		color: #166534;
	}

	.badge-disposed {
		background: #fee2e2;
		color: #dc2626;
	}

	.badge-sold {
		background: #dbeafe;
		color: #1d4ed8;
	}

	.actions {
		display: flex;
		gap: 0.5rem;
		flex-wrap: wrap;
	}

	.btn-small {
		padding: 0.25rem 0.5rem;
		font-size: 0.75rem;
	}

	.btn-success {
		background: #22c55e;
		color: white;
	}

	.btn-success:hover {
		background: #16a34a;
	}

	.btn-warning {
		background: #f59e0b;
		color: white;
	}

	.btn-warning:hover {
		background: #d97706;
	}

	.btn-danger {
		background: #ef4444;
		color: white;
	}

	.btn-danger:hover {
		background: #dc2626;
	}

	.empty-state {
		text-align: center;
		padding: 3rem;
		color: var(--color-text-muted);
	}

	.modal-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.5);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 100;
	}

	.modal {
		width: 100%;
		max-width: 600px;
		margin: 1rem;
		max-height: 90vh;
		overflow-y: auto;
	}

	.modal-wide {
		max-width: 800px;
	}

	.modal h2 {
		margin-bottom: 1.5rem;
	}

	.form-row {
		display: flex;
		gap: 1rem;
	}

	.form-row .form-group {
		flex: 1;
	}

	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1.5rem;
	}

	.asset-summary {
		display: flex;
		gap: 2rem;
		background: var(--color-bg-secondary);
		padding: 1rem;
		border-radius: 0.5rem;
		margin-bottom: 1rem;
	}

	.summary-item {
		display: flex;
		flex-direction: column;
	}

	.summary-label {
		font-size: 0.875rem;
		color: var(--color-text-muted);
	}

	.summary-value {
		font-family: var(--font-mono);
		font-weight: 600;
	}

	.empty-history {
		text-align: center;
		color: var(--color-text-muted);
		padding: 2rem;
	}

	@media (max-width: 768px) {
		.form-row {
			flex-direction: column;
			gap: 0;
		}

		.asset-summary {
			flex-direction: column;
			gap: 0.5rem;
		}
	}
</style>
