<script lang="ts">
	import { page } from '$app/stores';
	import { api, type Payment, type PaymentType, type Contact, type Invoice } from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';
	import DateRangeFilter from '$lib/components/DateRangeFilter.svelte';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import StatusBadge, { type StatusConfig } from '$lib/components/StatusBadge.svelte';
	import { formatCurrency, formatDate } from '$lib/utils/formatting';
	import { requireTenantId, parseApiError } from '$lib/utils/tenant';

	let payments = $state<Payment[]>([]);
	let contacts = $state<Contact[]>([]);
	let unpaidInvoices = $state<Invoice[]>([]);
	let isLoading = $state(true);
	let error = $state('');
	let success = $state('');
	let actionLoading = $state(false);
	let showCreatePayment = $state(false);
	let filterType = $state<PaymentType | ''>('');
	let filterFromDate = $state('');
	let filterToDate = $state('');

	// New payment form
	let newType = $state<PaymentType>('RECEIVED');
	let newContactId = $state('');
	let newPaymentDate = $state(new Date().toISOString().split('T')[0]);
	let newAmount = $state('0');
	let newMethod = $state('BANK_TRANSFER');
	let newReference = $state('');
	let newNotes = $state('');
	let selectedInvoices = $state<{ invoice_id: string; amount: string }[]>([]);

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
			const [paymentData, contactData, invoiceData] = await Promise.all([
				api.listPayments(tenantId, {
					type: filterType || undefined,
					from_date: filterFromDate || undefined,
					to_date: filterToDate || undefined
				}),
				api.listContacts(tenantId, { active_only: true }),
				api.listInvoices(tenantId, { status: 'SENT' })
			]);
			payments = paymentData;
			contacts = contactData;
			unpaidInvoices = invoiceData;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load data';
		} finally {
			isLoading = false;
		}
	}

	async function createPayment(e: Event) {
		e.preventDefault();
		const tenantId = requireTenantId($page, (err) => (error = err));
		if (!tenantId) return;

		actionLoading = true;
		error = '';
		try {
			const payment = await api.createPayment(tenantId, {
				payment_type: newType,
				contact_id: newContactId || undefined,
				payment_date: newPaymentDate,
				amount: newAmount,
				payment_method: newMethod,
				reference: newReference || undefined,
				notes: newNotes || undefined,
				allocations: selectedInvoices.filter((i) => i.invoice_id && parseFloat(i.amount) > 0)
			});
			payments = [payment, ...payments];
			showCreatePayment = false;
			resetForm();
			success = m.payments_recordPayment();
			setTimeout(() => (success = ''), 3000);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionLoading = false;
		}
	}

	function resetForm() {
		newType = 'RECEIVED';
		newContactId = '';
		newPaymentDate = new Date().toISOString().split('T')[0];
		newAmount = '0';
		newMethod = 'BANK_TRANSFER';
		newReference = '';
		newNotes = '';
		selectedInvoices = [];
	}

	function addInvoiceAllocation() {
		selectedInvoices = [...selectedInvoices, { invoice_id: '', amount: '0' }];
	}

	function removeInvoiceAllocation(index: number) {
		selectedInvoices = selectedInvoices.filter((_, i) => i !== index);
	}

	async function handleFilter() {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadData(tenantId);
		}
	}

	const typeConfig: Record<PaymentType, StatusConfig> = {
		RECEIVED: { class: 'badge-received', label: m.payments_paymentReceived() },
		MADE: { class: 'badge-made', label: m.payments_paymentMade() }
	};

	function getMethodLabel(method: string): string {
		switch (method) {
			case 'BANK_TRANSFER': return m.payments_bankTransfer();
			case 'CASH': return m.payments_cash();
			case 'CARD': return m.payments_card();
			default: return m.payments_other();
		}
	}

	function getContactName(contactId: string | undefined): string {
		if (!contactId) return '-';
		const contact = contacts.find((c) => c.id === contactId);
		return contact?.name || '-';
	}

	function getUnallocatedAmount(payment: Payment): Decimal {
		const total = payment.amount;
		const allocations = payment.allocations || [];
		const allocated = allocations.reduce(
			(sum, a) => sum.plus(a.amount),
			new Decimal(0)
		);
		return new Decimal(total).minus(allocated);
	}
</script>

<svelte:head>
	<title>{m.payments_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="page-header">
		<h1>{m.payments_title()}</h1>
		<div class="page-actions">
			<button class="btn btn-primary" onclick={() => (showCreatePayment = true)}>
				+ {m.payments_newPayment()}
			</button>
		</div>
	</div>

	<div class="filters card">
		<div class="filter-row">
			<select class="input" bind:value={filterType} onchange={handleFilter}>
				<option value="">{m.payments_allPayments()}</option>
				<option value="RECEIVED">{m.payments_received()}</option>
				<option value="MADE">{m.payments_made()}</option>
			</select>
			<DateRangeFilter
				bind:fromDate={filterFromDate}
				bind:toDate={filterToDate}
				onchange={handleFilter}
			/>
		</div>
	</div>

	{#if success}
		<ErrorAlert message={success} type="success" onDismiss={() => (success = '')} />
	{/if}

	{#if error}
		<ErrorAlert message={error} type="error" onDismiss={() => (error = '')} />
	{/if}

	{#if isLoading}
		<p>{m.common_loading()}</p>
	{:else if payments.length === 0}
		<div class="empty-state card">
			<p>{m.payments_noPayments()} {m.payments_createFirst()}</p>
		</div>
	{:else}
		<div class="card">
			<div class="table-container">
				<table class="table table-mobile-cards">
					<thead>
						<tr>
							<th>{m.payments_number()}</th>
							<th>{m.accounts_accountType()}</th>
							<th class="hide-mobile">{m.payments_contact()}</th>
							<th>{m.common_date()}</th>
							<th class="hide-mobile">{m.payments_method()}</th>
							<th>{m.common_amount()}</th>
							<th class="hide-mobile">{m.payments_unallocated()}</th>
							<th class="hide-mobile">{m.payments_reference()}</th>
						</tr>
					</thead>
					<tbody>
						{#each payments as payment}
							{@const unallocated = getUnallocatedAmount(payment)}
							<tr>
								<td class="number" data-label="Number">{payment.payment_number}</td>
								<td data-label="Type">
									<StatusBadge status={payment.payment_type} config={typeConfig} />
								</td>
								<td class="hide-mobile" data-label="Contact">{getContactName(payment.contact_id)}</td>
								<td data-label="Date">{formatDate(payment.payment_date)}</td>
								<td class="hide-mobile" data-label="Method">{getMethodLabel(payment.payment_method || 'OTHER')}</td>
								<td class="amount" data-label="Amount">{formatCurrency(payment.amount)}</td>
								<td class="amount hide-mobile" class:unallocated-warning={unallocated.greaterThan(0)} data-label="Unallocated">
									{formatCurrency(unallocated)}
								</td>
								<td class="reference hide-mobile" data-label="Reference">{payment.reference || '-'}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	{/if}
</div>

{#if showCreatePayment}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showCreatePayment = false)} role="presentation">
		<div class="modal card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="create-payment-title" tabindex="-1">
			<h2 id="create-payment-title">{m.payments_recordPayment()}</h2>
			<form onsubmit={createPayment}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="type">{m.payments_paymentType()}</label>
						<select class="input" id="type" bind:value={newType}>
							<option value="RECEIVED">{m.payments_paymentReceived()}</option>
							<option value="MADE">{m.payments_paymentMade()}</option>
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="contact">{m.payments_contact()}</label>
						<select class="input" id="contact" bind:value={newContactId}>
							<option value="">{m.payments_noContact()}</option>
							{#each contacts as contact}
								<option value={contact.id}>{contact.name}</option>
							{/each}
						</select>
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="date">{m.payments_paymentDate()}</label>
						<input class="input" type="date" id="date" bind:value={newPaymentDate} required />
					</div>
					<div class="form-group">
						<label class="label" for="amount">{m.common_amount()} *</label>
						<input
							class="input"
							type="number"
							step="0.01"
							min="0.01"
							id="amount"
							bind:value={newAmount}
							required
						/>
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="method">{m.payments_method()}</label>
						<select class="input" id="method" bind:value={newMethod}>
							<option value="BANK_TRANSFER">{m.payments_bankTransfer()}</option>
							<option value="CASH">{m.payments_cash()}</option>
							<option value="CARD">{m.payments_card()}</option>
							<option value="OTHER">{m.payments_other()}</option>
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="reference">{m.payments_reference()}</label>
						<input
							class="input"
							type="text"
							id="reference"
							bind:value={newReference}
							placeholder={m.payments_bankReference()}
						/>
					</div>
				</div>

				{#if unpaidInvoices.length > 0}
					<div class="allocations-section">
						<h3>{m.payments_allocateToInvoices()}</h3>
						{#each selectedInvoices as allocation, i}
							<div class="allocation-row">
								<select class="input" bind:value={allocation.invoice_id}>
									<option value="">{m.payments_selectInvoice()}</option>
									{#each unpaidInvoices as invoice}
										<option value={invoice.id}>
											{invoice.invoice_number} - {formatCurrency(invoice.total)}
										</option>
									{/each}
								</select>
								<input
									class="input input-small"
									type="number"
									step="0.01"
									min="0"
									bind:value={allocation.amount}
									placeholder={m.common_amount()}
								/>
								<button
									type="button"
									class="btn btn-small btn-danger"
									onclick={() => removeInvoiceAllocation(i)}
								>
									&times;
								</button>
							</div>
						{/each}
						<button type="button" class="btn btn-secondary btn-small" onclick={addInvoiceAllocation}>
							+ {m.payments_addInvoice()}
						</button>
					</div>
				{/if}

				<div class="form-group">
					<label class="label" for="notes">{m.invoices_notes()}</label>
					<textarea
						class="input"
						id="notes"
						bind:value={newNotes}
						rows="2"
						placeholder={m.invoices_additionalNotes()}
					></textarea>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={() => (showCreatePayment = false)}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary">{m.payments_recordPayment()}</button>
				</div>
			</form>
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
	}

	.number {
		font-family: var(--font-mono);
		font-weight: 500;
	}

	.amount {
		font-family: var(--font-mono);
		text-align: right;
	}

	.reference {
		font-family: var(--font-mono);
		font-size: 0.875rem;
		color: var(--color-text-muted);
	}

	.unallocated-warning {
		color: #f59e0b;
		font-weight: 500;
	}

	.btn-small {
		padding: 0.25rem 0.5rem;
		font-size: 0.75rem;
	}

	.btn-danger {
		background: #dc2626;
		color: white;
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

	.allocations-section {
		margin: 1.5rem 0;
		padding: 1rem;
		background: var(--color-bg-secondary, #f9fafb);
		border-radius: 0.5rem;
	}

	.allocations-section h3 {
		font-size: 0.875rem;
		margin-bottom: 0.75rem;
		color: var(--color-text-muted);
	}

	.allocation-row {
		display: flex;
		gap: 0.5rem;
		margin-bottom: 0.5rem;
		align-items: center;
	}

	.allocation-row select {
		flex: 2;
	}

	.input-small {
		width: 100px;
	}

	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1.5rem;
	}
</style>
