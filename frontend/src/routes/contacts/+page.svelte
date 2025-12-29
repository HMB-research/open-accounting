<script lang="ts">
	import { page } from '$app/stores';
	import { api, type Contact, type ContactType } from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';

	let contacts = $state<Contact[]>([]);
	let isLoading = $state(true);
	let error = $state('');
	let showCreateContact = $state(false);
	let filterType = $state<ContactType | ''>('');
	let searchQuery = $state('');

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

	$effect(() => {
		const tenantId = $page.url.searchParams.get('tenant');
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

	async function handleSearch() {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadContacts(tenantId);
		}
	}

	function getTypeLabel(type: ContactType): string {
		switch (type) {
			case 'CUSTOMER': return m.contacts_customer();
			case 'SUPPLIER': return m.contacts_supplier();
			case 'BOTH': return m.contacts_both();
		}
	}

	const typeBadgeClass: Record<ContactType, string> = {
		CUSTOMER: 'badge-customer',
		SUPPLIER: 'badge-supplier',
		BOTH: 'badge-both'
	};
</script>

<svelte:head>
	<title>{m.contacts_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="page-header">
		<h1>{m.contacts_title()}</h1>
		<div class="page-actions">
			<button class="btn btn-primary" onclick={() => (showCreateContact = true)}>
				+ {m.contacts_newContact()}
			</button>
		</div>
	</div>

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
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<p>{m.common_loading()}</p>
	{:else if contacts.length === 0}
		<div class="empty-state card">
			<p>{m.contacts_noContacts()} {m.contacts_createFirst()}</p>
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
									<span class="badge {typeBadgeClass[contact.contact_type]}">
										{getTypeLabel(contact.contact_type)}
									</span>
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

	.badge-customer {
		background: #dcfce7;
		color: #166534;
	}

	.badge-supplier {
		background: #fef3c7;
		color: #92400e;
	}

	.badge-both {
		background: #e0e7ff;
		color: #3730a3;
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

	/* Mobile responsive */
	@media (max-width: 768px) {
		.modal-backdrop {
			padding: 0;
			align-items: flex-end;
		}

		.modal {
			max-width: 100%;
			max-height: 95vh;
			border-radius: 1rem 1rem 0 0;
		}

		.modal-actions {
			flex-direction: column-reverse;
		}

		.modal-actions button {
			width: 100%;
		}
	}
</style>
