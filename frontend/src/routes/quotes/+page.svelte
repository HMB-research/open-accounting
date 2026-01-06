<script lang="ts">
	import { page } from '$app/stores';
	import { api, type Quote, type QuoteStatus, type Contact } from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';

	let quotes = $state<Quote[]>([]);
	let contacts = $state<Contact[]>([]);
	let isLoading = $state(true);
	let error = $state('');
	let showCreateQuote = $state(false);
	let filterStatus = $state<QuoteStatus | ''>('');

	// New quote form
	let newContactId = $state('');
	let newQuoteDate = $state(new Date().toISOString().split('T')[0]);
	let newValidUntil = $state(new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0]);
	let newNotes = $state('');
	let newLines = $state([
		{ description: '', quantity: '1', unit_price: '0', vat_rate: '22', discount_percent: '0' }
	]);

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
			const [quoteData, contactData] = await Promise.all([
				api.listQuotes(tenantId, {
					status: filterStatus || undefined
				}),
				api.listContacts(tenantId, { active_only: true })
			]);
			quotes = quoteData;
			contacts = contactData;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load data';
		} finally {
			isLoading = false;
		}
	}

	function addLine() {
		newLines = [
			...newLines,
			{ description: '', quantity: '1', unit_price: '0', vat_rate: '22', discount_percent: '0' }
		];
	}

	function removeLine(index: number) {
		if (newLines.length > 1) {
			newLines = newLines.filter((_, i) => i !== index);
		}
	}

	function calculateLineTotal(line: typeof newLines[0]): Decimal {
		const qty = new Decimal(line.quantity || 0);
		const price = new Decimal(line.unit_price || 0);
		const discount = new Decimal(line.discount_percent || 0);
		const vat = new Decimal(line.vat_rate || 0);

		const gross = qty.mul(price);
		const discountAmt = gross.mul(discount).div(100);
		const subtotal = gross.minus(discountAmt);
		const vatAmt = subtotal.mul(vat).div(100);
		return subtotal.plus(vatAmt);
	}

	let invoiceTotal = $derived.by(() => {
		return newLines.reduce((sum, line) => sum.plus(calculateLineTotal(line)), new Decimal(0));
	});

	async function createQuote(e: Event) {
		e.preventDefault();
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const quote = await api.createQuote(tenantId, {
				contact_id: newContactId,
				quote_date: newQuoteDate,
				valid_until: newValidUntil || undefined,
				notes: newNotes || undefined,
				lines: newLines.map((line) => ({
					description: line.description,
					quantity: line.quantity,
					unit_price: line.unit_price,
					vat_rate: line.vat_rate,
					discount_percent: line.discount_percent || '0'
				}))
			});
			quotes = [quote, ...quotes];
			showCreateQuote = false;
			resetForm();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create quote';
		}
	}

	function resetForm() {
		newContactId = '';
		newQuoteDate = new Date().toISOString().split('T')[0];
		newValidUntil = new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0];
		newNotes = '';
		newLines = [
			{ description: '', quantity: '1', unit_price: '0', vat_rate: '22', discount_percent: '0' }
		];
	}

	async function handleFilter() {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadData(tenantId);
		}
	}

	async function sendQuote(quoteId: string) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			await api.sendQuote(tenantId, quoteId);
			loadData(tenantId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to send quote';
		}
	}

	async function acceptQuote(quoteId: string) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			await api.acceptQuote(tenantId, quoteId);
			loadData(tenantId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to accept quote';
		}
	}

	async function rejectQuote(quoteId: string) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			await api.rejectQuote(tenantId, quoteId);
			loadData(tenantId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to reject quote';
		}
	}

	async function deleteQuote(quoteId: string) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		if (!confirm(m.quotes_confirmDelete())) return;

		try {
			await api.deleteQuote(tenantId, quoteId);
			quotes = quotes.filter(q => q.id !== quoteId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete quote';
		}
	}

	function getStatusLabel(status: QuoteStatus): string {
		switch (status) {
			case 'DRAFT': return m.quotes_statusDraft();
			case 'SENT': return m.quotes_statusSent();
			case 'ACCEPTED': return m.quotes_statusAccepted();
			case 'REJECTED': return m.quotes_statusRejected();
			case 'EXPIRED': return m.quotes_statusExpired();
			case 'CONVERTED': return m.quotes_statusConverted();
		}
	}

	const statusBadgeClass: Record<QuoteStatus, string> = {
		DRAFT: 'badge-draft',
		SENT: 'badge-sent',
		ACCEPTED: 'badge-accepted',
		REJECTED: 'badge-rejected',
		EXPIRED: 'badge-expired',
		CONVERTED: 'badge-converted'
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

	function getContactName(contactId: string): string {
		const contact = contacts.find((c) => c.id === contactId);
		return contact?.name || '-';
	}
</script>

<svelte:head>
	<title>{m.quotes_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="page-header">
		<h1>{m.quotes_title()}</h1>
		<div class="page-actions">
			<button class="btn btn-primary" onclick={() => (showCreateQuote = true)}>
				+ {m.quotes_newQuote()}
			</button>
		</div>
	</div>

	<div class="filters card">
		<div class="filter-row">
			<select class="input" bind:value={filterStatus} onchange={handleFilter}>
				<option value="">{m.quotes_allQuotes()}</option>
				<option value="DRAFT">{m.quotes_statusDraft()}</option>
				<option value="SENT">{m.quotes_statusSent()}</option>
				<option value="ACCEPTED">{m.quotes_statusAccepted()}</option>
				<option value="REJECTED">{m.quotes_statusRejected()}</option>
				<option value="EXPIRED">{m.quotes_statusExpired()}</option>
				<option value="CONVERTED">{m.quotes_statusConverted()}</option>
			</select>
		</div>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<p>{m.common_loading()}</p>
	{:else if quotes.length === 0}
		<div class="empty-state card">
			<p>{m.quotes_noQuotes()}</p>
		</div>
	{:else}
		<div class="card">
			<div class="table-container">
				<table class="table table-mobile-cards">
					<thead>
						<tr>
							<th>{m.quotes_number()}</th>
							<th>{m.invoices_status()}</th>
							<th class="hide-mobile">{m.invoices_customer()}</th>
							<th>{m.common_date()}</th>
							<th class="hide-mobile">{m.quotes_validUntil()}</th>
							<th class="text-right">{m.common_total()}</th>
							<th class="hide-mobile">{m.common_actions()}</th>
						</tr>
					</thead>
					<tbody>
						{#each quotes as quote}
							<tr>
								<td class="number" data-label={m.quotes_number()}>{quote.quote_number}</td>
								<td data-label={m.invoices_status()}>
									<span class="badge {statusBadgeClass[quote.status]}">
										{getStatusLabel(quote.status)}
									</span>
								</td>
								<td class="hide-mobile" data-label={m.invoices_customer()}>{getContactName(quote.contact_id)}</td>
								<td data-label={m.common_date()}>{formatDate(quote.quote_date)}</td>
								<td class="hide-mobile" data-label={m.quotes_validUntil()}>
									{quote.valid_until ? formatDate(quote.valid_until) : '-'}
								</td>
								<td class="amount text-right" data-label={m.common_total()}>{formatCurrency(quote.total)}</td>
								<td class="actions hide-mobile" data-label={m.common_actions()}>
									{#if quote.status === 'DRAFT'}
										<button class="btn btn-small" onclick={() => sendQuote(quote.id)} title={m.quotes_send()}>
											{m.quotes_send()}
										</button>
										<button class="btn btn-small btn-danger" onclick={() => deleteQuote(quote.id)} title={m.common_delete()}>
											{m.common_delete()}
										</button>
									{:else if quote.status === 'SENT'}
										<button class="btn btn-small btn-success" onclick={() => acceptQuote(quote.id)} title={m.quotes_accept()}>
											{m.quotes_accept()}
										</button>
										<button class="btn btn-small btn-danger" onclick={() => rejectQuote(quote.id)} title={m.quotes_reject()}>
											{m.quotes_reject()}
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

{#if showCreateQuote}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showCreateQuote = false)} role="presentation">
		<div class="modal card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="create-quote-title" tabindex="-1">
			<h2 id="create-quote-title">{m.quotes_newQuote()}</h2>
			<form onsubmit={createQuote}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="contact">{m.invoices_customer()} *</label>
						<select class="input" id="contact" bind:value={newContactId} required>
							<option value="">{m.invoices_selectContact()}</option>
							{#each contacts as contact}
								<option value={contact.id}>{contact.name}</option>
							{/each}
						</select>
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="quote-date">{m.quotes_quoteDate()}</label>
						<input class="input" type="date" id="quote-date" bind:value={newQuoteDate} required />
					</div>
					<div class="form-group">
						<label class="label" for="valid-until">{m.quotes_validUntil()}</label>
						<input class="input" type="date" id="valid-until" bind:value={newValidUntil} />
					</div>
				</div>

				<div class="form-group">
					<label class="label">{m.invoices_lineItems()}</label>
					<div class="lines-container">
						{#each newLines as line, i}
							<div class="line-row">
								<input
									class="input line-description"
									placeholder={m.invoices_productOrService()}
									bind:value={line.description}
									required
								/>
								<input
									class="input line-qty"
									type="number"
									step="0.01"
									min="0.01"
									placeholder={m.invoices_qty()}
									bind:value={line.quantity}
									required
								/>
								<input
									class="input line-price"
									type="number"
									step="0.01"
									min="0"
									placeholder={m.invoices_unitPrice()}
									bind:value={line.unit_price}
									required
								/>
								<input
									class="input line-vat"
									type="number"
									step="0.01"
									min="0"
									placeholder={m.invoices_vat()}
									bind:value={line.vat_rate}
									required
								/>
								<span class="line-total">{formatCurrency(calculateLineTotal(line))}</span>
								{#if newLines.length > 1}
									<button type="button" class="btn btn-small btn-danger" onclick={() => removeLine(i)}>
										&times;
									</button>
								{/if}
							</div>
						{/each}
					</div>
					<button type="button" class="btn btn-secondary btn-small" onclick={addLine}>
						+ {m.invoices_addLine()}
					</button>
				</div>

				<div class="invoice-totals">
					<div class="total-row">
						<span>{m.common_total()}:</span>
						<span class="total-value">{formatCurrency(invoiceTotal)}</span>
					</div>
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
					<button type="button" class="btn btn-secondary" onclick={() => (showCreateQuote = false)}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary">{m.quotes_createQuote()}</button>
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
		background: #e5e7eb;
		color: #374151;
	}

	.badge-sent {
		background: #dbeafe;
		color: #1d4ed8;
	}

	.badge-accepted {
		background: #dcfce7;
		color: #166534;
	}

	.badge-rejected {
		background: #fee2e2;
		color: #dc2626;
	}

	.badge-expired {
		background: #fef3c7;
		color: #b45309;
	}

	.badge-converted {
		background: #f3e8ff;
		color: #7c3aed;
	}

	.actions {
		display: flex;
		gap: 0.5rem;
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
		max-width: 700px;
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

	.lines-container {
		margin-bottom: 0.5rem;
	}

	.line-row {
		display: flex;
		gap: 0.5rem;
		margin-bottom: 0.5rem;
		align-items: center;
	}

	.line-description {
		flex: 2;
	}

	.line-qty,
	.line-vat {
		width: 70px;
	}

	.line-price {
		width: 100px;
	}

	.line-total {
		width: 100px;
		text-align: right;
		font-family: var(--font-mono);
	}

	.invoice-totals {
		background: var(--color-bg-secondary);
		padding: 1rem;
		border-radius: 0.5rem;
		margin: 1rem 0;
	}

	.total-row {
		display: flex;
		justify-content: space-between;
		font-weight: 600;
	}

	.total-value {
		font-family: var(--font-mono);
	}

	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1.5rem;
	}

	@media (max-width: 768px) {
		.line-row {
			flex-wrap: wrap;
		}

		.line-description {
			width: 100%;
			flex: none;
		}
	}
</style>
