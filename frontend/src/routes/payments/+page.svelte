<script lang="ts">
	import { page } from '$app/stores';
	import { api, type Payment, type PaymentType, type Contact, type Invoice } from '$lib/api';
	import Decimal from 'decimal.js';

	let payments = $state<Payment[]>([]);
	let contacts = $state<Contact[]>([]);
	let unpaidInvoices = $state<Invoice[]>([]);
	let isLoading = $state(true);
	let error = $state('');
	let showCreatePayment = $state(false);
	let filterType = $state<PaymentType | ''>('');

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
					type: filterType || undefined
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
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

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
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create payment';
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

	const typeLabels: Record<PaymentType, string> = {
		RECEIVED: 'Payment Received',
		MADE: 'Payment Made'
	};

	const typeBadgeClass: Record<PaymentType, string> = {
		RECEIVED: 'badge-received',
		MADE: 'badge-made'
	};

	const methodLabels: Record<string, string> = {
		BANK_TRANSFER: 'Bank Transfer',
		CASH: 'Cash',
		CARD: 'Card',
		OTHER: 'Other'
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

	function getContactName(contactId: string | undefined): string {
		if (!contactId) return '-';
		const contact = contacts.find((c) => c.id === contactId);
		return contact?.name || '-';
	}

	function getUnallocatedAmount(payment: Payment): Decimal {
		const total = payment.amount;
		const allocated = payment.allocations.reduce(
			(sum, a) => sum.plus(a.amount),
			new Decimal(0)
		);
		return new Decimal(total).minus(allocated);
	}
</script>

<svelte:head>
	<title>Payments - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="page-header">
		<h1>Payments</h1>
		<div class="page-actions">
			<button class="btn btn-primary" onclick={() => (showCreatePayment = true)}>
				+ New Payment
			</button>
		</div>
	</div>

	<div class="filters card">
		<div class="filter-row">
			<select class="input" bind:value={filterType} onchange={handleFilter}>
				<option value="">All Payments</option>
				<option value="RECEIVED">Received</option>
				<option value="MADE">Made</option>
			</select>
		</div>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<p>Loading payments...</p>
	{:else if payments.length === 0}
		<div class="empty-state card">
			<p>No payments found. Record your first payment to get started.</p>
		</div>
	{:else}
		<div class="card">
			<div class="table-container">
				<table class="table table-mobile-cards">
					<thead>
						<tr>
							<th>Number</th>
							<th>Type</th>
							<th class="hide-mobile">Contact</th>
							<th>Date</th>
							<th class="hide-mobile">Method</th>
							<th>Amount</th>
							<th class="hide-mobile">Unallocated</th>
							<th class="hide-mobile">Reference</th>
						</tr>
					</thead>
					<tbody>
						{#each payments as payment}
							{@const unallocated = getUnallocatedAmount(payment)}
							<tr>
								<td class="number" data-label="Number">{payment.payment_number}</td>
								<td data-label="Type">
									<span class="badge {typeBadgeClass[payment.payment_type]}">
										{typeLabels[payment.payment_type]}
									</span>
								</td>
								<td class="hide-mobile" data-label="Contact">{getContactName(payment.contact_id)}</td>
								<td data-label="Date">{formatDate(payment.payment_date)}</td>
								<td class="hide-mobile" data-label="Method">{methodLabels[payment.payment_method || 'OTHER'] || payment.payment_method}</td>
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
			<h2 id="create-payment-title">Record Payment</h2>
			<form onsubmit={createPayment}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="type">Payment Type</label>
						<select class="input" id="type" bind:value={newType}>
							<option value="RECEIVED">Payment Received</option>
							<option value="MADE">Payment Made</option>
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="contact">Contact</label>
						<select class="input" id="contact" bind:value={newContactId}>
							<option value="">No contact</option>
							{#each contacts as contact}
								<option value={contact.id}>{contact.name}</option>
							{/each}
						</select>
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="date">Payment Date</label>
						<input class="input" type="date" id="date" bind:value={newPaymentDate} required />
					</div>
					<div class="form-group">
						<label class="label" for="amount">Amount *</label>
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
						<label class="label" for="method">Payment Method</label>
						<select class="input" id="method" bind:value={newMethod}>
							<option value="BANK_TRANSFER">Bank Transfer</option>
							<option value="CASH">Cash</option>
							<option value="CARD">Card</option>
							<option value="OTHER">Other</option>
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="reference">Reference</label>
						<input
							class="input"
							type="text"
							id="reference"
							bind:value={newReference}
							placeholder="Bank reference"
						/>
					</div>
				</div>

				{#if unpaidInvoices.length > 0}
					<div class="allocations-section">
						<h3>Allocate to Invoices</h3>
						{#each selectedInvoices as allocation, i}
							<div class="allocation-row">
								<select class="input" bind:value={allocation.invoice_id}>
									<option value="">Select invoice...</option>
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
									placeholder="Amount"
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
							+ Add Invoice
						</button>
					</div>
				{/if}

				<div class="form-group">
					<label class="label" for="notes">Notes</label>
					<textarea
						class="input"
						id="notes"
						bind:value={newNotes}
						rows="2"
						placeholder="Additional notes..."
					></textarea>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={() => (showCreatePayment = false)}>
						Cancel
					</button>
					<button type="submit" class="btn btn-primary">Record Payment</button>
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

	.badge-received {
		background: #dcfce7;
		color: #166534;
	}

	.badge-made {
		background: #fee2e2;
		color: #dc2626;
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
