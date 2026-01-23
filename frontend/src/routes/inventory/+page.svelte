<script lang="ts">
	import { page } from '$app/stores';
	import { api, type Product, type ProductCategory, type Warehouse, type ProductType, type ProductStatus, type StockLevel, type InventoryMovement, type MovementType } from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';
	import { formatCurrency, formatDate } from '$lib/utils/formatting';

	type Tab = 'products' | 'warehouses' | 'categories';

	let activeTab = $state<Tab>('products');
	let products = $state<Product[]>([]);
	let categories = $state<ProductCategory[]>([]);
	let warehouses = $state<Warehouse[]>([]);
	let isLoading = $state(true);
	let error = $state('');

	// Filter state
	let filterType = $state<ProductType | ''>('');
	let filterStatus = $state<ProductStatus | ''>('');
	let filterCategory = $state('');
	let filterLowStock = $state(false);
	let searchQuery = $state('');

	// Modals
	let showCreateProduct = $state(false);
	let showCreateWarehouse = $state(false);
	let showCreateCategory = $state(false);
	let showAdjustStock = $state(false);
	let showTransferStock = $state(false);
	let showMovements = $state(false);
	let selectedProduct = $state<Product | null>(null);
	let movements = $state<InventoryMovement[]>([]);

	// New product form
	let newProductName = $state('');
	let newProductCode = $state('');
	let newProductDescription = $state('');
	let newProductType = $state<ProductType>('GOODS');
	let newProductCategoryId = $state('');
	let newProductUnit = $state('pcs');
	let newProductPurchasePrice = $state('');
	let newProductSalesPrice = $state('');
	let newProductVatRate = $state('22');
	let newProductMinStock = $state('');
	let newProductReorderPoint = $state('');
	let newProductBarcode = $state('');

	// New warehouse form
	let newWarehouseCode = $state('');
	let newWarehouseName = $state('');
	let newWarehouseAddress = $state('');
	let newWarehouseIsDefault = $state(false);

	// New category form
	let newCategoryName = $state('');
	let newCategoryDescription = $state('');

	// Stock adjustment form
	let adjustQuantity = $state('');
	let adjustUnitCost = $state('');
	let adjustReason = $state('');
	let adjustWarehouseId = $state('');

	// Transfer form
	let transferQuantity = $state('');
	let transferFromWarehouseId = $state('');
	let transferToWarehouseId = $state('');
	let transferNotes = $state('');

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
			const [productData, categoryData, warehouseData] = await Promise.all([
				api.listProducts(tenantId, {
					product_type: filterType || undefined,
					status: filterStatus || undefined,
					category_id: filterCategory || undefined,
					search: searchQuery || undefined,
					low_stock: filterLowStock || undefined
				}),
				api.listProductCategories(tenantId),
				api.listWarehouses(tenantId)
			]);
			products = productData;
			categories = categoryData;
			warehouses = warehouseData;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load data';
		} finally {
			isLoading = false;
		}
	}

	async function handleFilter() {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadData(tenantId);
		}
	}

	async function createProduct(e: Event) {
		e.preventDefault();
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const product = await api.createProduct(tenantId, {
				name: newProductName,
				code: newProductCode || undefined,
				description: newProductDescription || undefined,
				product_type: newProductType,
				category_id: newProductCategoryId || undefined,
				unit: newProductUnit,
				purchase_price: newProductPurchasePrice || undefined,
				sales_price: newProductSalesPrice,
				vat_rate: newProductVatRate || '22',
				min_stock_level: newProductMinStock || undefined,
				reorder_point: newProductReorderPoint || undefined,
				barcode: newProductBarcode || undefined
			});
			products = [product, ...products];
			showCreateProduct = false;
			resetProductForm();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create product';
		}
	}

	function resetProductForm() {
		newProductName = '';
		newProductCode = '';
		newProductDescription = '';
		newProductType = 'GOODS';
		newProductCategoryId = '';
		newProductUnit = 'pcs';
		newProductPurchasePrice = '';
		newProductSalesPrice = '';
		newProductVatRate = '22';
		newProductMinStock = '';
		newProductReorderPoint = '';
		newProductBarcode = '';
	}

	async function createWarehouse(e: Event) {
		e.preventDefault();
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const warehouse = await api.createWarehouse(tenantId, {
				code: newWarehouseCode,
				name: newWarehouseName,
				address: newWarehouseAddress || undefined,
				is_default: newWarehouseIsDefault
			});
			warehouses = [warehouse, ...warehouses];
			showCreateWarehouse = false;
			resetWarehouseForm();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create warehouse';
		}
	}

	function resetWarehouseForm() {
		newWarehouseCode = '';
		newWarehouseName = '';
		newWarehouseAddress = '';
		newWarehouseIsDefault = false;
	}

	async function createCategory(e: Event) {
		e.preventDefault();
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const category = await api.createProductCategory(tenantId, {
				name: newCategoryName,
				description: newCategoryDescription || undefined
			});
			categories = [category, ...categories];
			showCreateCategory = false;
			resetCategoryForm();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create category';
		}
	}

	function resetCategoryForm() {
		newCategoryName = '';
		newCategoryDescription = '';
	}

	async function deleteProduct(productId: string) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		if (!confirm(m.inventory_confirmDeleteProduct())) return;

		try {
			await api.deleteProduct(tenantId, productId);
			products = products.filter(p => p.id !== productId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete product';
		}
	}

	async function deleteWarehouse(warehouseId: string) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		if (!confirm(m.inventory_confirmDeleteWarehouse())) return;

		try {
			await api.deleteWarehouse(tenantId, warehouseId);
			warehouses = warehouses.filter(w => w.id !== warehouseId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete warehouse';
		}
	}

	async function deleteCategory(categoryId: string) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		if (!confirm(m.inventory_confirmDeleteCategory())) return;

		try {
			await api.deleteProductCategory(tenantId, categoryId);
			categories = categories.filter(c => c.id !== categoryId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete category';
		}
	}

	function openAdjustStock(product: Product) {
		selectedProduct = product;
		adjustQuantity = '';
		adjustUnitCost = '';
		adjustReason = '';
		adjustWarehouseId = warehouses.length > 0 ? warehouses[0].id : '';
		showAdjustStock = true;
	}

	async function adjustStock(e: Event) {
		e.preventDefault();
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId || !selectedProduct) return;

		try {
			await api.adjustStock(tenantId, {
				product_id: selectedProduct.id,
				warehouse_id: adjustWarehouseId,
				quantity: adjustQuantity,
				unit_cost: adjustUnitCost || undefined,
				reason: adjustReason || undefined
			});
			showAdjustStock = false;
			selectedProduct = null;
			loadData(tenantId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to adjust stock';
		}
	}

	function openTransferStock(product: Product) {
		selectedProduct = product;
		transferQuantity = '';
		transferFromWarehouseId = warehouses.length > 0 ? warehouses[0].id : '';
		transferToWarehouseId = warehouses.length > 1 ? warehouses[1].id : '';
		transferNotes = '';
		showTransferStock = true;
	}

	async function transferStock(e: Event) {
		e.preventDefault();
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId || !selectedProduct) return;

		try {
			await api.transferStock(tenantId, {
				product_id: selectedProduct.id,
				from_warehouse_id: transferFromWarehouseId,
				to_warehouse_id: transferToWarehouseId,
				quantity: transferQuantity,
				notes: transferNotes || undefined
			});
			showTransferStock = false;
			selectedProduct = null;
			loadData(tenantId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to transfer stock';
		}
	}

	async function openMovements(product: Product) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		selectedProduct = product;
		try {
			movements = await api.getProductMovements(tenantId, product.id);
			showMovements = true;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load movements';
		}
	}

	function getProductTypeLabel(type: ProductType): string {
		switch (type) {
			case 'GOODS': return m.inventory_typeGoods();
			case 'SERVICE': return m.inventory_typeService();
		}
	}

	function getProductStatusLabel(status: ProductStatus): string {
		switch (status) {
			case 'ACTIVE': return m.inventory_statusActive();
			case 'INACTIVE': return m.inventory_statusInactive();
		}
	}

	function getMovementTypeLabel(type: MovementType): string {
		switch (type) {
			case 'IN': return m.inventory_movementIn();
			case 'OUT': return m.inventory_movementOut();
			case 'ADJUSTMENT': return m.inventory_movementAdjustment();
			case 'TRANSFER': return m.inventory_movementTransfer();
		}
	}

	function getCategoryName(categoryId: string | undefined): string {
		if (!categoryId) return '-';
		const category = categories.find(c => c.id === categoryId);
		return category?.name || '-';
	}

	function getWarehouseName(warehouseId: string | undefined): string {
		if (!warehouseId) return '-';
		const warehouse = warehouses.find(w => w.id === warehouseId);
		return warehouse?.name || '-';
	}

	function formatNumber(value: Decimal | number | string | undefined): string {
		if (value === undefined || value === '') return '0';
		const num = typeof value === 'object' && 'toFixed' in value ? value.toNumber() : Number(value);
		return new Intl.NumberFormat('et-EE').format(num);
	}

	function isLowStock(product: Product): boolean {
		const current = typeof product.current_stock === 'object' && 'toNumber' in product.current_stock
			? product.current_stock.toNumber()
			: Number(product.current_stock || 0);
		const minLevel = typeof product.min_stock_level === 'object' && 'toNumber' in product.min_stock_level
			? product.min_stock_level.toNumber()
			: Number(product.min_stock_level || 0);
		return minLevel > 0 && current <= minLevel;
	}
</script>

<svelte:head>
	<title>{m.inventory_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="page-header">
		<h1>{m.inventory_title()}</h1>
	</div>

	<div class="tabs">
		<button class="tab" class:active={activeTab === 'products'} onclick={() => activeTab = 'products'}>
			{m.inventory_products()}
		</button>
		<button class="tab" class:active={activeTab === 'warehouses'} onclick={() => activeTab = 'warehouses'}>
			{m.inventory_warehouses()}
		</button>
		<button class="tab" class:active={activeTab === 'categories'} onclick={() => activeTab = 'categories'}>
			{m.inventory_categories()}
		</button>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<p>{m.common_loading()}</p>
	{:else if activeTab === 'products'}
		<div class="page-actions">
			<button class="btn btn-primary" onclick={() => showCreateProduct = true}>
				+ {m.inventory_newProduct()}
			</button>
		</div>

		<div class="filters card">
			<div class="filter-row">
				<input class="input" type="text" placeholder={m.common_search()} bind:value={searchQuery} onchange={handleFilter} />
				<select class="input" bind:value={filterType} onchange={handleFilter}>
					<option value="">{m.inventory_filterByType()}</option>
					<option value="GOODS">{m.inventory_typeGoods()}</option>
					<option value="SERVICE">{m.inventory_typeService()}</option>
				</select>
				<select class="input" bind:value={filterStatus} onchange={handleFilter}>
					<option value="">{m.inventory_filterByStatus()}</option>
					<option value="ACTIVE">{m.inventory_statusActive()}</option>
					<option value="INACTIVE">{m.inventory_statusInactive()}</option>
				</select>
				<select class="input" bind:value={filterCategory} onchange={handleFilter}>
					<option value="">{m.inventory_filterByCategory()}</option>
					{#each categories as category}
						<option value={category.id}>{category.name}</option>
					{/each}
				</select>
				<label class="checkbox-label">
					<input type="checkbox" bind:checked={filterLowStock} onchange={handleFilter} />
					{m.inventory_showLowStock()}
				</label>
			</div>
		</div>

		{#if products.length === 0}
			<div class="empty-state card">
				<p>{m.inventory_noProducts()}</p>
			</div>
		{:else}
			<div class="card">
				<div class="table-container">
					<table class="table table-mobile-cards">
						<thead>
							<tr>
								<th>{m.inventory_code()}</th>
								<th>{m.inventory_productName()}</th>
								<th class="hide-mobile">{m.inventory_productType()}</th>
								<th class="hide-mobile">{m.inventory_category()}</th>
								<th class="text-right">{m.inventory_salesPrice()}</th>
								<th class="text-right">{m.inventory_currentStock()}</th>
								<th class="hide-mobile">{m.common_actions()}</th>
							</tr>
						</thead>
						<tbody>
							{#each products as product}
								<tr class:low-stock={isLowStock(product)}>
									<td data-label={m.inventory_code()}>{product.code}</td>
									<td data-label={m.inventory_productName()}>{product.name}</td>
									<td class="hide-mobile" data-label={m.inventory_productType()}>{getProductTypeLabel(product.product_type)}</td>
									<td class="hide-mobile" data-label={m.inventory_category()}>{getCategoryName(product.category_id)}</td>
									<td class="amount text-right" data-label={m.inventory_salesPrice()}>{product.sales_price ? formatCurrency(product.sales_price) : '-'}</td>
									<td class="text-right" data-label={m.inventory_currentStock()}>
										{formatNumber(product.current_stock)}
										{#if isLowStock(product)}
											<span class="badge badge-warning">{m.inventory_lowStock()}</span>
										{/if}
									</td>
									<td class="actions hide-mobile" data-label={m.common_actions()}>
										<button class="btn btn-small" onclick={() => openAdjustStock(product)} title={m.inventory_adjustStock()}>
											{m.inventory_adjustStock()}
										</button>
										{#if warehouses.length >= 2}
											<button class="btn btn-small" onclick={() => openTransferStock(product)} title={m.inventory_transferStock()}>
												{m.inventory_transferStock()}
											</button>
										{/if}
										<button class="btn btn-small" onclick={() => openMovements(product)} title={m.inventory_movements()}>
											{m.inventory_movements()}
										</button>
										<button class="btn btn-small btn-danger" onclick={() => deleteProduct(product.id)} title={m.common_delete()}>
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
	{:else if activeTab === 'warehouses'}
		<div class="page-actions">
			<button class="btn btn-primary" onclick={() => showCreateWarehouse = true}>
				+ {m.inventory_newWarehouse()}
			</button>
		</div>

		{#if warehouses.length === 0}
			<div class="empty-state card">
				<p>{m.inventory_noWarehouses()}</p>
			</div>
		{:else}
			<div class="card">
				<div class="table-container">
					<table class="table table-mobile-cards">
						<thead>
							<tr>
								<th>{m.inventory_warehouseCode()}</th>
								<th>{m.inventory_warehouseName()}</th>
								<th class="hide-mobile">{m.inventory_warehouseAddress()}</th>
								<th>{m.inventory_isDefault()}</th>
								<th>{m.inventory_isActive()}</th>
								<th class="hide-mobile">{m.common_actions()}</th>
							</tr>
						</thead>
						<tbody>
							{#each warehouses as warehouse}
								<tr>
									<td data-label={m.inventory_warehouseCode()}>{warehouse.code}</td>
									<td data-label={m.inventory_warehouseName()}>{warehouse.name}</td>
									<td class="hide-mobile" data-label={m.inventory_warehouseAddress()}>{warehouse.address || '-'}</td>
									<td data-label={m.inventory_isDefault()}>
										{#if warehouse.is_default}
											<span class="badge badge-active">{m.common_yes()}</span>
										{:else}
											-
										{/if}
									</td>
									<td data-label={m.inventory_isActive()}>
										{#if warehouse.is_active}
											<span class="badge badge-active">{m.inventory_statusActive()}</span>
										{:else}
											<span class="badge badge-inactive">{m.inventory_statusInactive()}</span>
										{/if}
									</td>
									<td class="actions hide-mobile" data-label={m.common_actions()}>
										<button class="btn btn-small btn-danger" onclick={() => deleteWarehouse(warehouse.id)} title={m.common_delete()}>
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
	{:else if activeTab === 'categories'}
		<div class="page-actions">
			<button class="btn btn-primary" onclick={() => showCreateCategory = true}>
				+ {m.inventory_newCategory()}
			</button>
		</div>

		{#if categories.length === 0}
			<div class="empty-state card">
				<p>{m.inventory_noCategories()}</p>
			</div>
		{:else}
			<div class="card">
				<div class="table-container">
					<table class="table table-mobile-cards">
						<thead>
							<tr>
								<th>{m.common_name()}</th>
								<th>{m.common_description()}</th>
								<th class="hide-mobile">{m.common_actions()}</th>
							</tr>
						</thead>
						<tbody>
							{#each categories as category}
								<tr>
									<td data-label={m.common_name()}>{category.name}</td>
									<td data-label={m.common_description()}>{category.description || '-'}</td>
									<td class="actions hide-mobile" data-label={m.common_actions()}>
										<button class="btn btn-small btn-danger" onclick={() => deleteCategory(category.id)} title={m.common_delete()}>
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
	{/if}
</div>

{#if showCreateProduct}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => showCreateProduct = false} role="presentation">
		<div class="modal card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="create-product-title" tabindex="-1">
			<h2 id="create-product-title">{m.inventory_newProduct()}</h2>
			<form onsubmit={createProduct}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="product-name">{m.inventory_productName()} *</label>
						<input class="input" type="text" id="product-name" bind:value={newProductName} required />
					</div>
					<div class="form-group">
						<label class="label" for="product-code">{m.inventory_code()}</label>
						<input class="input" type="text" id="product-code" bind:value={newProductCode} placeholder="Auto-generated" />
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="product-type">{m.inventory_productType()}</label>
						<select class="input" id="product-type" bind:value={newProductType}>
							<option value="GOODS">{m.inventory_typeGoods()}</option>
							<option value="SERVICE">{m.inventory_typeService()}</option>
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="product-category">{m.inventory_category()}</label>
						<select class="input" id="product-category" bind:value={newProductCategoryId}>
							<option value="">-</option>
							{#each categories as category}
								<option value={category.id}>{category.name}</option>
							{/each}
						</select>
					</div>
				</div>

				<div class="form-group">
					<label class="label" for="product-description">{m.common_description()}</label>
					<textarea class="input" id="product-description" bind:value={newProductDescription} rows="2"></textarea>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="product-unit">{m.inventory_unit()}</label>
						<input class="input" type="text" id="product-unit" bind:value={newProductUnit} />
					</div>
					<div class="form-group">
						<label class="label" for="product-barcode">{m.inventory_barcode()}</label>
						<input class="input" type="text" id="product-barcode" bind:value={newProductBarcode} />
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="product-purchase-price">{m.inventory_purchasePrice()}</label>
						<input class="input" type="number" step="0.01" min="0" id="product-purchase-price" bind:value={newProductPurchasePrice} />
					</div>
					<div class="form-group">
						<label class="label" for="product-sales-price">{m.inventory_salesPrice()} *</label>
						<input class="input" type="number" step="0.01" min="0" id="product-sales-price" bind:value={newProductSalesPrice} required />
					</div>
					<div class="form-group">
						<label class="label" for="product-vat">{m.inventory_vatRate()} (%)</label>
						<input class="input" type="number" step="0.01" min="0" max="100" id="product-vat" bind:value={newProductVatRate} />
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="product-min-stock">{m.inventory_minStockLevel()}</label>
						<input class="input" type="number" step="0.01" min="0" id="product-min-stock" bind:value={newProductMinStock} />
					</div>
					<div class="form-group">
						<label class="label" for="product-reorder">{m.inventory_reorderPoint()}</label>
						<input class="input" type="number" step="0.01" min="0" id="product-reorder" bind:value={newProductReorderPoint} />
					</div>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={() => showCreateProduct = false}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary">{m.inventory_createProduct()}</button>
				</div>
			</form>
		</div>
	</div>
{/if}

{#if showCreateWarehouse}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => showCreateWarehouse = false} role="presentation">
		<div class="modal card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="create-warehouse-title" tabindex="-1">
			<h2 id="create-warehouse-title">{m.inventory_newWarehouse()}</h2>
			<form onsubmit={createWarehouse}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="warehouse-code">{m.inventory_warehouseCode()} *</label>
						<input class="input" type="text" id="warehouse-code" bind:value={newWarehouseCode} required />
					</div>
					<div class="form-group">
						<label class="label" for="warehouse-name">{m.inventory_warehouseName()} *</label>
						<input class="input" type="text" id="warehouse-name" bind:value={newWarehouseName} required />
					</div>
				</div>

				<div class="form-group">
					<label class="label" for="warehouse-address">{m.inventory_warehouseAddress()}</label>
					<textarea class="input" id="warehouse-address" bind:value={newWarehouseAddress} rows="2"></textarea>
				</div>

				<div class="form-group">
					<label class="checkbox-label">
						<input type="checkbox" bind:checked={newWarehouseIsDefault} />
						{m.inventory_isDefault()}
					</label>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={() => showCreateWarehouse = false}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary">{m.inventory_createWarehouse()}</button>
				</div>
			</form>
		</div>
	</div>
{/if}

{#if showCreateCategory}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => showCreateCategory = false} role="presentation">
		<div class="modal card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="create-category-title" tabindex="-1">
			<h2 id="create-category-title">{m.inventory_newCategory()}</h2>
			<form onsubmit={createCategory}>
				<div class="form-group">
					<label class="label" for="category-name">{m.common_name()} *</label>
					<input class="input" type="text" id="category-name" bind:value={newCategoryName} required />
				</div>

				<div class="form-group">
					<label class="label" for="category-description">{m.common_description()}</label>
					<textarea class="input" id="category-description" bind:value={newCategoryDescription} rows="2"></textarea>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={() => showCreateCategory = false}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary">{m.inventory_createCategory()}</button>
				</div>
			</form>
		</div>
	</div>
{/if}

{#if showAdjustStock && selectedProduct}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => showAdjustStock = false} role="presentation">
		<div class="modal card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="adjust-stock-title" tabindex="-1">
			<h2 id="adjust-stock-title">{m.inventory_adjustStock()}: {selectedProduct.name}</h2>
			<form onsubmit={adjustStock}>
				<div class="form-group">
					<label class="label" for="adjust-warehouse">{m.inventory_warehouses()} *</label>
					<select class="input" id="adjust-warehouse" bind:value={adjustWarehouseId} required>
						{#each warehouses as warehouse}
							<option value={warehouse.id}>{warehouse.name}</option>
						{/each}
					</select>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="adjust-quantity">{m.inventory_quantity()} *</label>
						<input class="input" type="number" step="0.01" id="adjust-quantity" bind:value={adjustQuantity} required placeholder="Positive to add, negative to remove" />
					</div>
					<div class="form-group">
						<label class="label" for="adjust-cost">{m.inventory_unitCost()}</label>
						<input class="input" type="number" step="0.01" min="0" id="adjust-cost" bind:value={adjustUnitCost} />
					</div>
				</div>

				<div class="form-group">
					<label class="label" for="adjust-reason">{m.inventory_reason()}</label>
					<textarea class="input" id="adjust-reason" bind:value={adjustReason} rows="2"></textarea>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={() => showAdjustStock = false}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary">{m.inventory_adjustStock()}</button>
				</div>
			</form>
		</div>
	</div>
{/if}

{#if showTransferStock && selectedProduct}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => showTransferStock = false} role="presentation">
		<div class="modal card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="transfer-stock-title" tabindex="-1">
			<h2 id="transfer-stock-title">{m.inventory_transferStock()}: {selectedProduct.name}</h2>
			<form onsubmit={transferStock}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="transfer-from">{m.inventory_fromWarehouse()} *</label>
						<select class="input" id="transfer-from" bind:value={transferFromWarehouseId} required>
							{#each warehouses as warehouse}
								<option value={warehouse.id}>{warehouse.name}</option>
							{/each}
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="transfer-to">{m.inventory_toWarehouse()} *</label>
						<select class="input" id="transfer-to" bind:value={transferToWarehouseId} required>
							{#each warehouses as warehouse}
								<option value={warehouse.id}>{warehouse.name}</option>
							{/each}
						</select>
					</div>
				</div>

				<div class="form-group">
					<label class="label" for="transfer-quantity">{m.inventory_quantity()} *</label>
					<input class="input" type="number" step="0.01" min="0.01" id="transfer-quantity" bind:value={transferQuantity} required />
				</div>

				<div class="form-group">
					<label class="label" for="transfer-notes">{m.inventory_notes()}</label>
					<textarea class="input" id="transfer-notes" bind:value={transferNotes} rows="2"></textarea>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={() => showTransferStock = false}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary">{m.inventory_transferStock()}</button>
				</div>
			</form>
		</div>
	</div>
{/if}

{#if showMovements && selectedProduct}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => showMovements = false} role="presentation">
		<div class="modal modal-large card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="movements-title" tabindex="-1">
			<h2 id="movements-title">{m.inventory_movements()}: {selectedProduct.name}</h2>

			{#if movements.length === 0}
				<p class="empty-message">{m.inventory_noMovements()}</p>
			{:else}
				<div class="table-container">
					<table class="table">
						<thead>
							<tr>
								<th>{m.inventory_movementDate()}</th>
								<th>{m.inventory_movementType()}</th>
								<th>{m.inventory_warehouses()}</th>
								<th class="text-right">{m.inventory_quantity()}</th>
								<th class="text-right">{m.inventory_unitCost()}</th>
								<th>{m.inventory_reference()}</th>
							</tr>
						</thead>
						<tbody>
							{#each movements as movement}
								<tr>
									<td>{formatDate(movement.movement_date)}</td>
									<td>
										<span class="badge badge-{movement.movement_type.toLowerCase()}">
											{getMovementTypeLabel(movement.movement_type)}
										</span>
									</td>
									<td>{getWarehouseName(movement.warehouse_id)}</td>
									<td class="text-right">{formatNumber(movement.quantity)}</td>
									<td class="text-right">{movement.unit_cost ? formatCurrency(movement.unit_cost) : '-'}</td>
									<td>{movement.reference || '-'}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}

			<div class="modal-actions">
				<button type="button" class="btn btn-secondary" onclick={() => showMovements = false}>
					{m.common_close()}
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.tabs {
		display: flex;
		gap: 0.25rem;
		margin-bottom: 1rem;
		border-bottom: 1px solid var(--border);
		padding-bottom: 0;
	}

	.tab {
		padding: 0.75rem 1.5rem;
		background: none;
		border: none;
		border-bottom: 2px solid transparent;
		cursor: pointer;
		font-weight: 500;
		color: var(--text-secondary);
		transition: all 0.2s;
	}

	.tab:hover {
		color: var(--text);
	}

	.tab.active {
		color: var(--primary);
		border-bottom-color: var(--primary);
	}

	.page-actions {
		display: flex;
		justify-content: flex-end;
		margin-bottom: 1rem;
	}

	.filters {
		margin-bottom: 1rem;
	}

	.filter-row {
		display: flex;
		gap: 1rem;
		flex-wrap: wrap;
		align-items: center;
	}

	.filter-row .input {
		flex: 1;
		min-width: 150px;
	}

	.checkbox-label {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		cursor: pointer;
		white-space: nowrap;
	}

	.low-stock {
		background-color: rgba(255, 193, 7, 0.1);
	}

	.badge-warning {
		background-color: #ffc107;
		color: #000;
	}

	.badge-active {
		background-color: #28a745;
		color: #fff;
	}

	.badge-inactive {
		background-color: #6c757d;
		color: #fff;
	}

	.badge-in {
		background-color: #28a745;
		color: #fff;
	}

	.badge-out {
		background-color: #dc3545;
		color: #fff;
	}

	.badge-adjustment {
		background-color: #ffc107;
		color: #000;
	}

	.badge-transfer {
		background-color: #17a2b8;
		color: #fff;
	}

	.modal-large {
		max-width: 900px;
	}

	.empty-message {
		text-align: center;
		padding: 2rem;
		color: var(--text-secondary);
	}
</style>
