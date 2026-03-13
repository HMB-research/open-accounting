<script lang="ts">
	import { browser } from '$app/environment';
	import { page } from '$app/stores';
	import { api, type Contact, type ContactType, type ImportContactsResult } from '$lib/api';
	import WorkflowHero, { type WorkflowHeroAction, type WorkflowHeroAside, type WorkflowHeroStat } from '$lib/components/WorkflowHero.svelte';
	import * as m from '$lib/paraglide/messages.js';
	import StatusBadge, { type StatusConfig } from '$lib/components/StatusBadge.svelte';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let contacts = $state<Contact[]>([]);
	let isLoading = $state(true);
	let error = $state('');
	let showCreateContact = $state(false);
	let showImportContacts = $state(false);
	let filterType = $state<ContactType | ''>('');
	let searchQuery = $state('');
	let importError = $state('');
	let importFileName = $state('');
	let importCSVContent = $state('');
	let isImporting = $state(false);
	let importResult = $state<ImportContactsResult | null>(null);

	// New contact form
	let newName = $state('');
	let newType = $state<ContactType>('CUSTOMER');
	let newEmail = $state('');
	let newPhone = $state('');
	let newVatNumber = $state('');
	let newAddress = $state('');
	let newCity = $state('');
	let newPostalCode = $state('');
	let newCountry = $state('EE');
	let newPaymentDays = $state(14);

	let totalCustomers = $derived(contacts.filter((contact) => contact.contact_type === 'CUSTOMER' || contact.contact_type === 'BOTH').length);
	let totalSuppliers = $derived(contacts.filter((contact) => contact.contact_type === 'SUPPLIER' || contact.contact_type === 'BOTH').length);
	let averageTerms = $derived(
		contacts.length === 0
			? 0
			: Math.round(
					contacts.reduce((sum, contact) => sum + (contact.payment_terms_days || 0), 0) / contacts.length
				)
	);

	let heroStats = $derived.by<WorkflowHeroStat[]>(() => [
		{
			label: m.common_total(),
			value: String(contacts.length),
			detail: m.contacts_totalContactsHint()
		},
		{
			label: m.contacts_customers(),
			value: String(totalCustomers),
			tone: totalCustomers > 0 ? 'success' : 'default'
		},
		{
			label: m.contacts_suppliers(),
			value: String(totalSuppliers),
			tone: totalSuppliers > 0 ? 'success' : 'default'
		},
		{
			label: m.contacts_paymentTerms(),
			value: contacts.length > 0 ? `${averageTerms} ${m.contacts_days()}` : `14 ${m.contacts_days()}`,
			detail: m.contacts_termsHint()
		}
	]);

	let heroActions = $derived.by<WorkflowHeroAction[]>(() => [
		{
			label: m.contacts_importContacts(),
			variant: 'secondary',
			onclick: openImportModal,
			disabled: !tenantId
		},
		{
			label: m.contacts_newContact(),
			onclick: () => (showCreateContact = true),
			disabled: !tenantId
		}
	]);

	let heroAside = $derived.by<WorkflowHeroAside>(() => {
		if (contacts.length === 0) {
			return {
				kicker: m.dashboard_setupCenter(),
				title: m.contacts_firstImportTitle(),
				body: m.contacts_firstImportDesc(),
				linkLabel: m.contacts_importContacts(),
				href: tenantId ? `/contacts?tenant=${tenantId}` : '/contacts',
				items: [m.contacts_firstImportItemOne(), m.contacts_firstImportItemTwo()]
			};
		}

		return {
			kicker: m.invoices_title(),
			title: m.contacts_readyForBillingTitle(),
			body: m.contacts_readyForBillingDesc(),
			linkLabel: m.invoices_newInvoice(),
			href: tenantId ? `/invoices?tenant=${tenantId}` : '/invoices',
			items: [m.contacts_readyForBillingItemOne(), m.contacts_readyForBillingItemTwo()]
		};
	});

	$effect(() => {
		if (tenantId) {
			loadContacts(tenantId);
		}
	});

	async function loadContacts(tenantId: string) {
		isLoading = true;
		error = '';

		try {
			contacts = await api.listContacts(tenantId, {
				type: filterType || undefined,
				search: searchQuery || undefined,
				active_only: true
			});
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load contacts';
		} finally {
			isLoading = false;
		}
	}

	async function createContact(e: Event) {
		e.preventDefault();
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const contact = await api.createContact(tenantId, {
				name: newName,
				contact_type: newType,
				email: newEmail || undefined,
				phone: newPhone || undefined,
				vat_number: newVatNumber || undefined,
				address_line1: newAddress || undefined,
				city: newCity || undefined,
				postal_code: newPostalCode || undefined,
				country_code: newCountry,
				payment_terms_days: newPaymentDays
			});
			contacts = [...contacts, contact].sort((a, b) => a.name.localeCompare(b.name));
			showCreateContact = false;
			resetForm();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create contact';
		}
	}

	function resetForm() {
		newName = '';
		newType = 'CUSTOMER';
		newEmail = '';
		newPhone = '';
		newVatNumber = '';
		newAddress = '';
		newCity = '';
		newPostalCode = '';
		newCountry = 'EE';
		newPaymentDays = 14;
	}

	function openImportModal() {
		showImportContacts = true;
		importError = '';
		importFileName = '';
		importCSVContent = '';
		importResult = null;
	}

	function closeImportModal() {
		showImportContacts = false;
		importError = '';
		importFileName = '';
		importCSVContent = '';
		importResult = null;
	}

	async function handleImportFileChange(event: Event) {
		const input = event.currentTarget as HTMLInputElement | null;
		const file = input?.files?.[0];

		importResult = null;

		if (!file) {
			importFileName = '';
			importCSVContent = '';
			return;
		}

		importFileName = file.name;
		importCSVContent = await file.text();
		importError = '';
	}

	async function submitImport(event: Event) {
		event.preventDefault();

		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		if (!importCSVContent.trim()) {
			importError = m.contacts_importFileRequired();
			return;
		}

		isImporting = true;
		importError = '';

		try {
			importResult = await api.importContacts(tenantId, {
				file_name: importFileName || undefined,
				csv_content: importCSVContent
			});

			if (importResult.contacts_created > 0) {
				await loadContacts(tenantId);
			}
		} catch (err) {
			importError = err instanceof Error ? err.message : 'Failed to import contacts';
		} finally {
			isImporting = false;
		}
	}

	function downloadImportTemplate() {
		if (!browser) return;

		const template = [
			'name,contact_type,code,reg_code,vat_number,email,phone,address_line1,city,postal_code,country_code,payment_terms_days,credit_limit,notes',
			'Example Customer,CUSTOMER,CUST-001,12345678,EE123456789,customer@example.com,+3725551234,Main Street 1,Tallinn,10111,EE,14,2500.00,Imported from CSV'
		].join('\n');

		const blob = new Blob([template], { type: 'text/csv;charset=utf-8' });
		const url = window.URL.createObjectURL(blob);
		const link = document.createElement('a');
		link.href = url;
		link.download = 'contacts-import-template.csv';
		document.body.appendChild(link);
		link.click();
		document.body.removeChild(link);
		window.URL.revokeObjectURL(url);
	}

	async function handleSearch() {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadContacts(tenantId);
		}
	}

	const typeConfig: Record<ContactType, StatusConfig> = {
		CUSTOMER: { class: 'badge-customer', label: m.contacts_customer() },
		SUPPLIER: { class: 'badge-supplier', label: m.contacts_supplier() },
		BOTH: { class: 'badge-both', label: m.contacts_both() }
	};
</script>

<svelte:head>
	<title>{m.contacts_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<WorkflowHero
		eyebrow={m.dashboard_setupTaskContactsTitle()}
		title={m.contacts_title()}
		description={m.contacts_heroDesc()}
		actions={heroActions}
		stats={heroStats}
		aside={heroAside}
	/>

	<div class="filters card">
		<div class="filter-row">
			<select class="input" bind:value={filterType} onchange={handleSearch}>
				<option value="">{m.contacts_allTypes()}</option>
				<option value="CUSTOMER">{m.contacts_customers()}</option>
				<option value="SUPPLIER">{m.contacts_suppliers()}</option>
				<option value="BOTH">{m.contacts_both()}</option>
			</select>
			<input
				class="input search-input"
				type="text"
				placeholder={m.contacts_searchContacts()}
				bind:value={searchQuery}
				onkeyup={(e) => e.key === 'Enter' && handleSearch()}
			/>
			<button class="btn btn-secondary" onclick={handleSearch}>{m.common_search()}</button>
		</div>
		<p class="filter-hint">{m.contacts_filterHint({ count: String(contacts.length) })}</p>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<p>{m.common_loading()}</p>
	{:else if contacts.length === 0}
		<div class="empty-state card">
			<h2>{m.contacts_noContacts()}</h2>
			<p>{m.contacts_createFirst()}</p>
			<div class="empty-actions">
				<button class="btn btn-secondary" onclick={openImportModal}>
					{m.contacts_importContacts()}
				</button>
				<button class="btn btn-primary" onclick={() => (showCreateContact = true)}>
					{m.contacts_newContact()}
				</button>
			</div>
		</div>
	{:else}
		<div class="card">
			<div class="table-container">
				<table class="table table-mobile-cards">
					<thead>
						<tr>
							<th>{m.common_name()}</th>
							<th>{m.contacts_type()}</th>
							<th>{m.common_email()}</th>
							<th class="hide-mobile">{m.common_phone()}</th>
							<th class="hide-mobile">{m.contacts_vatNumber()}</th>
							<th class="hide-mobile">{m.contacts_paymentTerms()}</th>
						</tr>
					</thead>
					<tbody>
						{#each contacts as contact}
							<tr class:inactive={!contact.is_active}>
								<td class="name" data-label="Name">{contact.name}</td>
								<td data-label="Type">
									<StatusBadge status={contact.contact_type} config={typeConfig} />
								</td>
								<td class="email" data-label="Email">{contact.email || '-'}</td>
								<td class="hide-mobile" data-label="Phone">{contact.phone || '-'}</td>
								<td class="vat hide-mobile" data-label="VAT">{contact.vat_number || '-'}</td>
								<td class="hide-mobile" data-label="Terms">{contact.payment_terms_days} {m.contacts_days()}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	{/if}
	</div>

{#if showImportContacts}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={closeImportModal} role="presentation">
		<div
			class="modal card import-modal"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
			aria-labelledby="import-contacts-title"
			tabindex="-1"
		>
			<h2 id="import-contacts-title">{m.contacts_importContacts()}</h2>
			<p class="import-description">{m.contacts_importDescription()}</p>

			{#if importError}
				<div class="alert alert-error">{importError}</div>
			{/if}

			<form onsubmit={submitImport}>
				<div class="form-group">
					<label class="label" for="contact-import-file">{m.contacts_importChooseFile()}</label>
					<input
						class="input"
						id="contact-import-file"
						type="file"
						accept=".csv,text/csv"
						onchange={handleImportFileChange}
					/>
					<p class="form-hint">{m.contacts_importTemplateHint()}</p>
					{#if importFileName}
						<p class="selected-file">
							{m.contacts_importSelectedFile()}: <span>{importFileName}</span>
						</p>
					{/if}
				</div>

				<div class="import-toolbar">
					<button type="button" class="btn btn-secondary" onclick={downloadImportTemplate}>
						{m.contacts_importTemplate()}
					</button>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={closeImportModal}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary" disabled={isImporting}>
						{isImporting ? m.contacts_importing() : m.contacts_importContacts()}
					</button>
				</div>
			</form>

			{#if importResult}
				<div class="import-summary">
					<h3>{m.contacts_importSummary()}</h3>
					<div class="summary-grid">
						<div class="summary-card">
							<span class="summary-label">{m.contacts_importRowsProcessed()}</span>
							<strong>{importResult.rows_processed}</strong>
						</div>
						<div class="summary-card">
							<span class="summary-label">{m.contacts_importContactsCreated()}</span>
							<strong>{importResult.contacts_created}</strong>
						</div>
						<div class="summary-card">
							<span class="summary-label">{m.contacts_importRowsSkipped()}</span>
							<strong>{importResult.rows_skipped}</strong>
						</div>
					</div>

					{#if importResult.errors?.length}
						<h3>{m.contacts_importErrors()}</h3>
						<div class="table-container import-errors">
							<table class="table table-mobile-cards">
								<thead>
									<tr>
										<th>{m.contacts_importRow()}</th>
										<th>{m.common_name()}</th>
										<th>{m.contacts_importMessage()}</th>
									</tr>
								</thead>
								<tbody>
									{#each importResult.errors as rowError}
										<tr>
											<td data-label={m.contacts_importRow()}>{rowError.row}</td>
											<td data-label={m.common_name()}>{rowError.name || '-'}</td>
											<td data-label={m.contacts_importMessage()}>{rowError.message}</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
					{:else}
						<div class="alert alert-success">
							{m.contacts_importContactsCreated()}: {importResult.contacts_created}
						</div>
					{/if}
				</div>
			{/if}
		</div>
	</div>
{/if}

{#if showCreateContact}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showCreateContact = false)} role="presentation">
		<div class="modal card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="create-contact-title" tabindex="-1">
			<h2 id="create-contact-title">{m.contacts_newContact()}</h2>
			<form onsubmit={createContact}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="name">{m.common_name()} *</label>
						<input
							class="input"
							type="text"
							id="name"
							bind:value={newName}
							required
							placeholder={m.contacts_companyOrPerson()}
						/>
					</div>
					<div class="form-group">
						<label class="label" for="type">{m.contacts_type()}</label>
						<select class="input" id="type" bind:value={newType}>
							<option value="CUSTOMER">{m.contacts_customer()}</option>
							<option value="SUPPLIER">{m.contacts_supplier()}</option>
							<option value="BOTH">{m.contacts_both()}</option>
						</select>
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="email">{m.common_email()}</label>
						<input
							class="input"
							type="email"
							id="email"
							bind:value={newEmail}
							placeholder="email@example.com"
						/>
					</div>
					<div class="form-group">
						<label class="label" for="phone">{m.common_phone()}</label>
						<input
							class="input"
							type="tel"
							id="phone"
							bind:value={newPhone}
							placeholder="+372 5551234"
						/>
					</div>
				</div>

				<div class="form-group">
					<label class="label" for="vat">{m.contacts_vatNumber()}</label>
					<input
						class="input"
						type="text"
						id="vat"
						bind:value={newVatNumber}
						placeholder="EE123456789"
					/>
				</div>

				<div class="form-group">
					<label class="label" for="address">{m.common_address()}</label>
					<input
						class="input"
						type="text"
						id="address"
						bind:value={newAddress}
						placeholder={m.contacts_streetAddress()}
					/>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="city">{m.contacts_city()}</label>
						<input class="input" type="text" id="city" bind:value={newCity} placeholder="Tallinn" />
					</div>
					<div class="form-group">
						<label class="label" for="postal">{m.contacts_postalCode()}</label>
						<input
							class="input"
							type="text"
							id="postal"
							bind:value={newPostalCode}
							placeholder="10111"
						/>
					</div>
					<div class="form-group">
						<label class="label" for="country">{m.contacts_country()}</label>
						<select class="input" id="country" bind:value={newCountry}>
							<option value="EE">Estonia</option>
							<option value="LV">Latvia</option>
							<option value="LT">Lithuania</option>
							<option value="FI">Finland</option>
							<option value="DE">Germany</option>
						</select>
					</div>
				</div>

				<div class="form-group">
					<label class="label" for="payment-days">{m.contacts_paymentTermsDays()}</label>
					<input
						class="input"
						type="number"
						id="payment-days"
						bind:value={newPaymentDays}
						min="0"
						max="365"
					/>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={() => (showCreateContact = false)}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary">{m.common_create()}</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<style>
	.filters {
		margin-bottom: 1.5rem;
		padding: 1rem;
	}

	.filter-hint {
		margin-top: 0.85rem;
		font-size: 0.925rem;
		color: var(--color-text-muted);
	}

	.filter-row {
		display: flex;
		gap: 1rem;
		align-items: center;
		flex-wrap: wrap;
	}

	.filter-row select {
		min-width: 120px;
	}

	.search-input {
		flex: 1;
		min-width: 150px;
	}

	.name {
		font-weight: 500;
	}

	.email {
		color: var(--color-text-muted);
	}

	.vat {
		font-family: var(--font-mono);
		font-size: 0.875rem;
	}

	.inactive {
		opacity: 0.6;
	}

	.empty-state {
		text-align: center;
		padding: 3rem;
		color: var(--color-text-muted);
	}

	.empty-state h2 {
		color: var(--color-text);
		margin-bottom: 0.5rem;
	}

	.empty-actions {
		display: flex;
		justify-content: center;
		gap: 0.75rem;
		flex-wrap: wrap;
		margin-top: 1.25rem;
	}

	.modal-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.5);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 100;
		padding: 1rem;
	}

	.modal {
		width: 100%;
		max-width: 600px;
		max-height: 90vh;
		overflow-y: auto;
	}

	.modal h2 {
		margin-bottom: 1.5rem;
	}

	.import-modal {
		max-width: 760px;
	}

	.form-row {
		display: flex;
		gap: 1rem;
		flex-wrap: wrap;
	}

	.form-row .form-group {
		flex: 1;
		min-width: 150px;
	}

	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1.5rem;
	}

	.import-description,
	.form-hint,
	.selected-file,
	.summary-label {
		color: var(--color-text-muted);
	}

	.form-hint,
	.selected-file {
		margin-top: 0.5rem;
		font-size: 0.925rem;
	}

	.selected-file span {
		color: var(--color-text);
		font-weight: 500;
	}

	.import-toolbar {
		display: flex;
		justify-content: flex-start;
		margin-top: 1rem;
	}

	.import-summary {
		margin-top: 2rem;
		padding-top: 1.5rem;
		border-top: 1px solid var(--color-border);
	}

	.import-summary h3 {
		margin-bottom: 1rem;
	}

	.summary-grid {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 0.75rem;
		margin-bottom: 1.5rem;
	}

	.summary-card {
		padding: 1rem;
		border: 1px solid var(--color-border);
		border-radius: 0.75rem;
		background: var(--color-bg);
	}

	.summary-card strong {
		display: block;
		font-size: 1.5rem;
		margin-top: 0.35rem;
	}

	.import-errors {
		margin-top: 1rem;
	}

	/* Mobile responsive */
	@media (max-width: 768px) {
		.filters {
			padding: 0.75rem;
		}

		.filter-row {
			flex-direction: column;
			gap: 0.75rem;
		}

		.filter-row select,
		.search-input {
			width: 100%;
			min-width: unset;
			min-height: 44px;
		}

		.filter-row .btn {
			width: 100%;
			min-height: 44px;
			justify-content: center;
		}

		.modal-backdrop {
			padding: 0;
			align-items: flex-end;
		}

		.modal {
			max-width: 100%;
			max-height: 95vh;
			border-radius: 1rem 1rem 0 0;
		}

		.modal h2 {
			font-size: 1.25rem;
		}

		.form-row {
			flex-direction: column;
			gap: 0;
		}

		.form-row .form-group {
			width: 100%;
			min-width: unset;
		}

		.modal-actions {
			flex-direction: column-reverse;
		}

		.modal-actions button {
			width: 100%;
			min-height: 44px;
		}

		.summary-grid {
			grid-template-columns: 1fr;
		}

		.empty-state {
			padding: 2rem 1rem;
		}

		.empty-actions {
			flex-direction: column;
		}

		.empty-actions .btn {
			width: 100%;
		}
	}
</style>
