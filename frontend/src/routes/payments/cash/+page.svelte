<script lang="ts">
	import { page } from '$app/stores';
	import { api, type Payment, type PaymentType, type Contact } from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';
	import StatusBadge, { type StatusConfig } from '$lib/components/StatusBadge.svelte';
	import { formatCurrency, formatDate } from '$lib/utils/formatting';

	let payments = $state<Payment[]>([]);
	let contacts = $state<Contact[]>([]);
	let isLoading = $state(true);
	let error = $state('');
	let showCreatePayment = $state(false);
	let filterType = $state<PaymentType | ''>('');

	// New payment form
	let newType = $state<PaymentType>('RECEIVED');
	let newContactId = $state('');
	let newPaymentDate = $state(new Date().toISOString().split('T')[0]);
	let newAmount = $state('0');
	let newReference = $state('');
	let newNotes = $state('');

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
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const payment = await api.createPayment(tenantId, {
				payment_type: newType,
				contact_id: newContactId || undefined,
				payment_date: newPaymentDate,
				amount: newAmount,
				payment_method: 'CASH',
				reference: newReference || undefined,
				notes: newNotes || undefined,
				allocations: []
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
		newReference = '';
		newNotes = '';
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

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<p>{m.common_loading()}</p>
	{:else if payments.length === 0}
		<div class="empty-state card">
			<p>{m.cashPayments_noPayments()}</p>
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
							<th>{m.common_amount()}</th>
							<th class="hide-mobile">{m.payments_reference()}</th>
						</tr>
					</thead>
					<tbody>
						{#each payments as payment}
							<tr>
								<td class="number" data-label="Number">{payment.payment_number}</td>
								<td data-label="Type">
									<StatusBadge status={payment.payment_type} config={typeConfig} />
								</td>
								<td class="hide-mobile" data-label="Contact">{getContactName(payment.contact_id)}</td>
								<td data-label="Date">{formatDate(payment.payment_date)}</td>
								<td class="amount" data-label="Amount">{formatCurrency(payment.amount)}</td>
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
					<button type="button" class="btn btn-secondary" onclick={() => (showCreatePayment = false)}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary">{m.cashPayments_recordPayment()}</button>
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
