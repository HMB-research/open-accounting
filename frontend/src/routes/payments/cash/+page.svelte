<script lang="ts">
	import { page } from '$app/stores';
	import { api, type Payment, type PaymentType, type Contact } from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import StatusBadge, { type StatusConfig } from '$lib/components/StatusBadge.svelte';
	import { dateInputToApiTimestamp } from '$lib/utils/dates';
	import { formatCurrency, formatDate, formStringValue } from '$lib/utils/formatting';
	import { requireTenantId, parseApiError } from '$lib/utils/tenant';

	let payments = $state<Payment[]>([]);
	let contacts = $state<Contact[]>([]);
	let isLoading = $state(true);
	let error = $state('');
	let success = $state('');
	let actionLoading = $state(false);
	let showCreatePayment = $state(false);
	let showReversePayment = $state(false);
	let selectedPaymentForReversal = $state<Payment | null>(null);
	let filterType = $state<PaymentType | ''>('');

	// New payment form
	let newType = $state<PaymentType>('RECEIVED');
	let newContactId = $state('');
	let newPaymentDate = $state(new Date().toISOString().split('T')[0]);
	let newAmount = $state('0');
	let newReference = $state('');
	let newNotes = $state('');

	// Payment reversal form
	let reversalDate = $state(new Date().toISOString().split('T')[0]);
	let reversalReason = $state('');
	let reversalReference = $state('');
	let reversalNotes = $state('');

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
			const [paymentData, contactData] = await Promise.all([
				api.listPayments(tenantId, {
					type: filterType || undefined,
					method: 'CASH'
				}),
				api.listContacts(tenantId, { active_only: true })
			]);
			payments = paymentData;
			contacts = contactData;
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
				payment_date: dateInputToApiTimestamp(newPaymentDate),
				amount: formStringValue(newAmount),
				payment_method: 'CASH',
				reference: newReference ? formStringValue(newReference) : undefined,
				notes: newNotes ? formStringValue(newNotes) : undefined,
				allocations: []
			});
			payments = paymentMatchesFilter(payment) ? [payment, ...payments] : payments;
			closeCreatePayment();
			resetForm();
			success = m.cashPayments_recordPayment();
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
		newReference = '';
		newNotes = '';
	}

	async function submitPaymentReversal(e: Event) {
		e.preventDefault();
		const tenantId = requireTenantId($page, (err) => (error = err));
		if (!tenantId || !selectedPaymentForReversal) return;

		actionLoading = true;
		error = '';
		try {
			const result = await api.reversePayment(tenantId, selectedPaymentForReversal.id, {
				payment_date: dateInputToApiTimestamp(reversalDate),
				reason: formStringValue(reversalReason),
				reference: reversalReference ? formStringValue(reversalReference) : undefined,
				notes: reversalNotes ? formStringValue(reversalNotes) : undefined
			});

			const originalID = result.original_payment.id;
			const reversalID = result.reversal_payment.id;
			const updatedPayments = payments
				.map((payment) => (payment.id === originalID ? result.original_payment : payment))
				.filter((payment) => payment.id !== reversalID && paymentMatchesFilter(payment));
			payments = paymentMatchesFilter(result.reversal_payment)
				? [result.reversal_payment, ...updatedPayments]
				: updatedPayments;
			closeReversePayment();
			success = m.payments_reverseSuccess({
				original: result.original_payment.payment_number,
				reversal: result.reversal_payment.payment_number
			});
			setTimeout(() => (success = ''), 3000);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionLoading = false;
		}
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

	function getContactName(contactId: string | undefined): string {
		if (!contactId) return '-';
		const contact = contacts.find((c) => c.id === contactId);
		return contact?.name || '-';
	}

	function paymentMatchesFilter(payment: Payment): boolean {
		return payment.payment_method === 'CASH' && (!filterType || payment.payment_type === filterType);
	}

	function openReversePayment(payment: Payment) {
		selectedPaymentForReversal = payment;
		reversalDate = new Date().toISOString().split('T')[0];
		reversalReason = '';
		reversalReference = `REVERSAL-${payment.payment_number}`;
		reversalNotes = '';
		showReversePayment = true;
	}

	function closeReversePayment() {
		showReversePayment = false;
		selectedPaymentForReversal = null;
		reversalDate = new Date().toISOString().split('T')[0];
		reversalReason = '';
		reversalReference = '';
		reversalNotes = '';
	}

	function closeCreatePayment() {
		showCreatePayment = false;
	}

	function handleCreatePaymentBackdropKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			closeCreatePayment();
		}
	}

	function handleReversePaymentBackdropKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			closeReversePayment();
		}
	}

	function handleModalKeydown() {
		// Let keyboard events bubble to the backdrop so Escape still closes the modal.
	}

	function isReversalPayment(payment: Payment): boolean {
		return !!payment.reversal_of_payment_id;
	}

	function isReversedPayment(payment: Payment): boolean {
		return !!payment.reversed_by_payment_id;
	}

	function canReversePayment(payment: Payment): boolean {
		return !isReversalPayment(payment) && !isReversedPayment(payment);
	}

	// Calculate totals as derived state
	let totals = $derived.by(() => {
		const received = payments
			.filter(p => p.payment_type === 'RECEIVED')
			.reduce((sum, p) => sum.plus(p.amount), new Decimal(0));
		const made = payments
			.filter(p => p.payment_type === 'MADE')
			.reduce((sum, p) => sum.plus(p.amount), new Decimal(0));
		return { received, made, balance: received.minus(made) };
	});
</script>

<svelte:head>
	<title>{m.cashPayments_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="page-header">
		<h1>{m.cashPayments_title()}</h1>
		<div class="page-actions">
			<button class="btn btn-primary" onclick={() => (showCreatePayment = true)}>
				+ {m.cashPayments_newPayment()}
			</button>
		</div>
	</div>

	<!-- Cash Summary Card -->
	<div class="summary-cards">
		<div class="summary-card received">
			<span class="summary-label">{m.cashPayments_totalReceived()}</span>
			<span class="summary-value">{formatCurrency(totals.received)}</span>
		</div>
		<div class="summary-card made">
			<span class="summary-label">{m.cashPayments_totalPaid()}</span>
			<span class="summary-value">{formatCurrency(totals.made)}</span>
		</div>
		<div class="summary-card balance">
			<span class="summary-label">{m.cashPayments_cashBalance()}</span>
			<span class="summary-value" class:negative={totals.balance.lessThan(0)}>
				{formatCurrency(totals.balance)}
			</span>
		</div>
	</div>

	<div class="filters card">
		<div class="filter-row">
			<select class="input" bind:value={filterType} onchange={handleFilter}>
				<option value="">{m.payments_allPayments()}</option>
				<option value="RECEIVED">{m.payments_received()}</option>
				<option value="MADE">{m.payments_made()}</option>
			</select>
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
			<p>{m.cashPayments_noPayments()}</p>
		</div>
	{:else}
		<div class="card data-table-card">
			<div class="table-container">
				<table class="table table-mobile-cards readable-table cash-payments-table">
					<colgroup>
						<col class="col-number" />
						<col class="col-type" />
						<col class="col-contact" />
						<col class="col-date" />
						<col class="col-amount" />
						<col class="col-reference" />
						<col class="col-actions" />
					</colgroup>
					<thead>
						<tr>
							<th>{m.payments_number()}</th>
							<th>{m.accounts_accountType()}</th>
							<th class="hide-mobile">{m.payments_contact()}</th>
							<th>{m.common_date()}</th>
							<th class="amount-heading">{m.common_amount()}</th>
							<th class="hide-mobile">{m.payments_reference()}</th>
							<th class="actions-heading">{m.common_actions()}</th>
						</tr>
					</thead>
					<tbody>
						{#each payments as payment (payment.id)}
							<tr>
								<td class="number" data-label={m.payments_number()}>
									<div class="cell-stack">
										<span class="cell-primary">{payment.payment_number}</span>
										{#if isReversalPayment(payment)}
											<span class="table-state-badge">{m.payments_reversal()}</span>
										{:else if isReversedPayment(payment)}
											<span class="table-state-badge">{m.payments_reversed()}</span>
										{/if}
									</div>
								</td>
								<td class="payment-type" data-label={m.accounts_accountType()}>
									<StatusBadge status={payment.payment_type} config={typeConfig} />
								</td>
								<td class="cell-muted" data-label={m.payments_contact()}>{getContactName(payment.contact_id)}</td>
								<td class="date" data-label={m.common_date()}>{formatDate(payment.payment_date)}</td>
								<td class="amount" data-label={m.common_amount()}>{formatCurrency(payment.amount)}</td>
								<td class="reference" data-label={m.payments_reference()}>
									<span class="cell-ellipsis" title={payment.reference || undefined}>{payment.reference || '-'}</span>
								</td>
								<td class="actions actions-cell" data-label={m.common_actions()}>
									<div class="actions-stack">
										{#if canReversePayment(payment)}
											<button type="button" class="btn btn-secondary btn-small" onclick={() => openReversePayment(payment)}>
												{m.payments_reverse()}
											</button>
										{/if}
									</div>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	{/if}
</div>

{#if showCreatePayment}
	<div
		class="modal-backdrop"
		onclick={closeCreatePayment}
		onkeydown={handleCreatePaymentBackdropKeydown}
		role="presentation"
	>
		<div
			class="modal card"
			onclick={(e) => e.stopPropagation()}
			onkeydown={handleModalKeydown}
			role="dialog"
			aria-modal="true"
			aria-labelledby="create-payment-title"
			tabindex="-1"
		>
			<h2 id="create-payment-title">{m.cashPayments_recordPayment()}</h2>
			<form onsubmit={createPayment}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="type">{m.payments_paymentType()}</label>
						<select class="input" id="type" bind:value={newType}>
							<option value="RECEIVED">{m.cashPayments_cashIn()}</option>
							<option value="MADE">{m.cashPayments_cashOut()}</option>
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="contact">{m.payments_contact()}</label>
						<select class="input" id="contact" bind:value={newContactId}>
							<option value="">{m.payments_noContact()}</option>
							{#each contacts as contact (contact.id)}
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

				<div class="form-group">
					<label class="label" for="reference">{m.payments_reference()}</label>
					<input
						class="input"
						type="text"
						id="reference"
						bind:value={newReference}
						placeholder={m.cashPayments_receiptNumber()}
					/>
				</div>

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
					<button type="button" class="btn btn-secondary" onclick={closeCreatePayment}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary" disabled={actionLoading}>
						{m.cashPayments_recordPayment()}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}

{#if showReversePayment && selectedPaymentForReversal}
	<div
		class="modal-backdrop"
		onclick={closeReversePayment}
		onkeydown={handleReversePaymentBackdropKeydown}
		role="presentation"
	>
		<div
			class="modal card"
			onclick={(e) => e.stopPropagation()}
			onkeydown={handleModalKeydown}
			role="dialog"
			aria-modal="true"
			aria-labelledby="reverse-payment-title"
			tabindex="-1"
		>
			<h2 id="reverse-payment-title">{m.payments_reversePayment()}</h2>
			<form onsubmit={submitPaymentReversal}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="reversal-original">{m.payments_originalPayment()}</label>
						<input
							class="input"
							id="reversal-original"
							value={selectedPaymentForReversal.payment_number}
							readonly
						/>
					</div>
					<div class="form-group">
						<label class="label" for="reversal-date">{m.payments_reversalDate()}</label>
						<input class="input" type="date" id="reversal-date" bind:value={reversalDate} required />
					</div>
				</div>

				<div class="form-group">
					<label class="label" for="reversal-reason">{m.payments_reversalReason()} *</label>
					<input
						class="input"
						type="text"
						id="reversal-reason"
						bind:value={reversalReason}
						placeholder={m.payments_reversalReasonPlaceholder()}
						required
					/>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="reversal-reference">{m.payments_reversalReference()}</label>
						<input
							class="input"
							type="text"
							id="reversal-reference"
							bind:value={reversalReference}
						/>
					</div>
					<div class="form-group">
						<label class="label" for="reversal-notes">{m.payments_reversalNotes()}</label>
						<input
							class="input"
							type="text"
							id="reversal-notes"
							bind:value={reversalNotes}
						/>
					</div>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={closeReversePayment}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-danger" disabled={actionLoading}>
						{m.payments_reverse()}
					</button>
				</div>
			</form>
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

	.summary-cards {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
		gap: 1rem;
		margin-bottom: 1.5rem;
	}

	.summary-card {
		background: white;
		border-radius: 0.5rem;
		padding: 1.25rem;
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.summary-card.received {
		border-left: 4px solid #22c55e;
	}

	.summary-card.made {
		border-left: 4px solid #ef4444;
	}

	.summary-card.balance {
		border-left: 4px solid #3b82f6;
	}

	.summary-label {
		font-size: 0.875rem;
		color: var(--color-text-muted);
	}

	.summary-value {
		font-size: 1.5rem;
		font-weight: 600;
		font-family: var(--font-mono);
	}

	.summary-value.negative {
		color: #ef4444;
	}

	.filters {
		margin-bottom: 1.5rem;
		padding: 1rem;
	}

	.filter-row {
		display: flex;
		gap: 1rem;
	}

	.cash-payments-table {
		min-width: 1000px;
	}

	.cash-payments-table .col-number {
		width: 13%;
	}

	.cash-payments-table .col-type {
		width: 14%;
	}

	.cash-payments-table .col-contact {
		width: 17%;
	}

	.cash-payments-table .col-date {
		width: 10%;
	}

	.cash-payments-table .col-amount {
		width: 12%;
	}

	.cash-payments-table .col-reference {
		width: 23%;
	}

	.cash-payments-table .col-actions {
		width: 11%;
	}

	.cash-payments-table th,
	.cash-payments-table td {
		padding-inline: 0.9rem;
	}

	.cash-payments-table tbody tr {
		height: 4.15rem;
	}

	.cash-payments-table .number .cell-primary,
	.cash-payments-table .reference .cell-ellipsis,
	.cash-payments-table .date,
	.cash-payments-table .amount {
		font-size: 0.86rem;
	}

	.cash-payments-table .number .cell-primary {
		white-space: nowrap;
	}

	.cash-payments-table .cell-muted {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.cash-payments-table .payment-type :global(.badge) {
		min-width: 7.2rem;
		justify-content: center;
		font-weight: 650;
	}

	.cash-payments-table .reference .cell-ellipsis {
		max-width: 100%;
		color: #334155;
	}

	.cash-payments-table .actions-stack {
		justify-content: flex-end;
	}

	.cash-payments-table .btn-small {
		min-width: 4.2rem;
		justify-content: center;
	}

	@media (max-width: 768px) {
		.cash-payments-table {
			min-width: 0;
		}

		.cash-payments-table tbody tr {
			height: auto;
		}

		.cash-payments-table th,
		.cash-payments-table td {
			padding-inline: 0;
		}

		.cash-payments-table .payment-type :global(.badge) {
			min-width: 0;
		}

		.cash-payments-table .cell-muted {
			max-width: 62%;
			text-align: right;
			white-space: normal;
			overflow-wrap: anywhere;
		}
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
		max-width: 500px;
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

	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1.5rem;
	}
</style>
