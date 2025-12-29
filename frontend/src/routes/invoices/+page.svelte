<script lang="ts">
	import { page } from '$app/stores';
	import { api, type Invoice, type InvoiceType, type InvoiceStatus, type Contact } from '$lib/api';
	import Decimal from 'decimal.js';

	let invoices = $state<Invoice[]>([]);
	let contacts = $state<Contact[]>([]);
	let isLoading = $state(true);
	let error = $state('');
	let showCreateInvoice = $state(false);
	let filterType = $state<InvoiceType | ''>('');
	let filterStatus = $state<InvoiceStatus | ''>('');

	// New invoice form
	let newType = $state<InvoiceType>('SALES');
	let newContactId = $state('');
	let newIssueDate = $state(new Date().toISOString().split('T')[0]);
	let newDueDate = $state(new Date(Date.now() + 14 * 24 * 60 * 60 * 1000).toISOString().split('T')[0]);
	let newReference = $state('');
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
			const [invoiceData, contactData] = await Promise.all([
				api.listInvoices(tenantId, {
					type: filterType || undefined,
					status: filterStatus || undefined
				}),
				api.listContacts(tenantId, { active_only: true })
			]);
			invoices = invoiceData;
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

	function calculateTotal(): Decimal {
		return newLines.reduce((sum, line) => sum.plus(calculateLineTotal(line)), new Decimal(0));
	}

	async function createInvoice(e: Event) {
		e.preventDefault();
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const invoice = await api.createInvoice(tenantId, {
				invoice_type: newType,
				contact_id: newContactId,
				issue_date: newIssueDate,
				due_date: newDueDate,
				reference: newReference || undefined,
				notes: newNotes || undefined,
				lines: newLines.map((line) => ({
					description: line.description,
					quantity: line.quantity,
					unit_price: line.unit_price,
					vat_rate: line.vat_rate,
					discount_percent: line.discount_percent || '0'
				}))
			});
			invoices = [invoice, ...invoices];
			showCreateInvoice = false;
			resetForm();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create invoice';
		}
	}

	function resetForm() {
		newType = 'SALES';
		newContactId = '';
		newIssueDate = new Date().toISOString().split('T')[0];
		newDueDate = new Date(Date.now() + 14 * 24 * 60 * 60 * 1000).toISOString().split('T')[0];
		newReference = '';
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

	async function sendInvoice(invoiceId: string) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			await api.sendInvoice(tenantId, invoiceId);
			loadData(tenantId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to send invoice';
		}
	}

	async function downloadPDF(invoiceId: string, invoiceNumber: string) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			await api.downloadInvoicePDF(tenantId, invoiceId, invoiceNumber);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to download PDF';
		}
	}

	const typeLabels: Record<InvoiceType, string> = {
		SALES: 'Sales Invoice',
		PURCHASE: 'Purchase Invoice',
		CREDIT_NOTE: 'Credit Note'
	};

	const statusLabels: Record<InvoiceStatus, string> = {
		DRAFT: 'Draft',
		SENT: 'Sent',
		PARTIALLY_PAID: 'Partial',
		PAID: 'Paid',
		OVERDUE: 'Overdue',
		VOIDED: 'Voided'
	};

	const statusClass: Record<InvoiceStatus, string> = {
		DRAFT: 'badge-draft',
		SENT: 'badge-sent',
		PARTIALLY_PAID: 'badge-partial',
		PAID: 'badge-paid',
		OVERDUE: 'badge-overdue',
		VOIDED: 'badge-voided'
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
</script>

<svelte:head>
	<title>Invoices - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="page-header">
		<h1>Invoices</h1>
		<div class="page-actions">
			<button class="btn btn-primary" onclick={() => (showCreateInvoice = true)}>
				+ New Invoice
			</button>
		</div>
	</div>

	<div class="filters card">
		<div class="filter-row">
			<select class="input" bind:value={filterType} onchange={handleFilter}>
				<option value="">All Types</option>
				<option value="SALES">Sales Invoices</option>
				<option value="PURCHASE">Purchase Invoices</option>
				<option value="CREDIT_NOTE">Credit Notes</option>
			</select>
			<select class="input" bind:value={filterStatus} onchange={handleFilter}>
				<option value="">All Statuses</option>
				<option value="DRAFT">Draft</option>
				<option value="SENT">Sent</option>
				<option value="PARTIALLY_PAID">Partially Paid</option>
				<option value="PAID">Paid</option>
				<option value="OVERDUE">Overdue</option>
			</select>
		</div>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<p>Loading invoices...</p>
	{:else if invoices.length === 0}
		<div class="empty-state card">
			<p>No invoices found. Create your first invoice to get started.</p>
		</div>
	{:else}
		<div class="card">
			<div class="table-container">
				<table class="table table-mobile-cards">
					<thead>
						<tr>
							<th>Number</th>
							<th class="hide-mobile">Type</th>
							<th>Contact</th>
							<th class="hide-mobile">Issue Date</th>
							<th>Due Date</th>
							<th>Total</th>
							<th>Status</th>
							<th>Actions</th>
						</tr>
					</thead>
					<tbody>
						{#each invoices as invoice}
							<tr>
								<td class="number" data-label="Number">{invoice.invoice_number}</td>
								<td class="hide-mobile" data-label="Type">{typeLabels[invoice.invoice_type]}</td>
								<td data-label="Contact">{invoice.contact?.name || '-'}</td>
								<td class="hide-mobile" data-label="Issue Date">{formatDate(invoice.issue_date)}</td>
								<td data-label="Due Date">{formatDate(invoice.due_date)}</td>
								<td class="amount" data-label="Total">{formatCurrency(invoice.total)}</td>
								<td data-label="Status">
									<span class="badge {statusClass[invoice.status]}">
										{statusLabels[invoice.status]}
									</span>
								</td>
								<td class="actions actions-cell">
									<button
										class="btn btn-small btn-secondary"
										onclick={() => downloadPDF(invoice.id, invoice.invoice_number)}
										title="Download PDF"
									>
										PDF
									</button>
									{#if invoice.status === 'DRAFT'}
										<button class="btn btn-small" onclick={() => sendInvoice(invoice.id)}>
											Send
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

{#if showCreateInvoice}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showCreateInvoice = false)} role="presentation">
		<div class="modal modal-large card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="create-invoice-title" tabindex="-1">
			<h2 id="create-invoice-title">Create Invoice</h2>
			<form onsubmit={createInvoice}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="type">Invoice Type</label>
						<select class="input" id="type" bind:value={newType}>
							<option value="SALES">Sales Invoice</option>
							<option value="PURCHASE">Purchase Invoice</option>
							<option value="CREDIT_NOTE">Credit Note</option>
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="contact">Contact *</label>
						<select class="input" id="contact" bind:value={newContactId} required>
							<option value="">Select contact...</option>
							{#each contacts as contact}
								<option value={contact.id}>{contact.name}</option>
							{/each}
						</select>
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="issue-date">Issue Date</label>
						<input class="input" type="date" id="issue-date" bind:value={newIssueDate} required />
					</div>
					<div class="form-group">
						<label class="label" for="due-date">Due Date</label>
						<input class="input" type="date" id="due-date" bind:value={newDueDate} required />
					</div>
					<div class="form-group">
						<label class="label" for="reference">Reference</label>
						<input
							class="input"
							type="text"
							id="reference"
							bind:value={newReference}
							placeholder="PO-12345"
						/>
					</div>
				</div>

				<div class="lines-section">
					<h3>Invoice Lines</h3>
					<table class="lines-table">
						<thead>
							<tr>
								<th>Description</th>
								<th>Qty</th>
								<th>Unit Price</th>
								<th>VAT %</th>
								<th>Discount %</th>
								<th>Total</th>
								<th></th>
							</tr>
						</thead>
						<tbody>
							{#each newLines as line, i}
								<tr>
									<td>
										<input
											class="input"
											type="text"
											bind:value={line.description}
											required
											placeholder="Product or service"
										/>
									</td>
									<td>
										<input
											class="input input-small"
											type="number"
											step="0.01"
											min="0.01"
											bind:value={line.quantity}
											required
										/>
									</td>
									<td>
										<input
											class="input input-small"
											type="number"
											step="0.01"
											min="0"
											bind:value={line.unit_price}
											required
										/>
									</td>
									<td>
										<select class="input input-small" bind:value={line.vat_rate}>
											<option value="22">22%</option>
											<option value="9">9%</option>
											<option value="0">0%</option>
										</select>
									</td>
									<td>
										<input
											class="input input-small"
											type="number"
											step="0.1"
											min="0"
											max="100"
											bind:value={line.discount_percent}
										/>
									</td>
									<td class="amount">{formatCurrency(calculateLineTotal(line))}</td>
									<td>
										{#if newLines.length > 1}
											<button
												type="button"
												class="btn btn-small btn-danger"
												onclick={() => removeLine(i)}
											>
												&times;
											</button>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
						<tfoot>
							<tr>
								<td colspan="5" style="text-align: right;"><strong>Total:</strong></td>
								<td class="amount"><strong>{formatCurrency(calculateTotal())}</strong></td>
								<td></td>
							</tr>
						</tfoot>
					</table>
					<button type="button" class="btn btn-secondary" onclick={addLine}>+ Add Line</button>
				</div>

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
					<button type="button" class="btn btn-secondary" onclick={() => (showCreateInvoice = false)}>
						Cancel
					</button>
					<button type="submit" class="btn btn-primary">Create Invoice</button>
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
		flex-wrap: wrap;
	}

	.filter-row select {
		flex: 1;
		min-width: 150px;
	}

	.number {
		font-family: var(--font-mono);
		font-weight: 500;
	}

	.amount {
		font-family: var(--font-mono);
		text-align: right;
	}

	.badge-draft {
		background: #f3f4f6;
		color: #6b7280;
	}

	.badge-sent {
		background: #dbeafe;
		color: #1d4ed8;
	}

	.badge-partial {
		background: #fef3c7;
		color: #92400e;
	}

	.badge-paid {
		background: #dcfce7;
		color: #166534;
	}

	.badge-overdue {
		background: #fee2e2;
		color: #dc2626;
	}

	.badge-voided {
		background: #f3f4f6;
		color: #9ca3af;
	}

	.btn-small {
		padding: 0.25rem 0.5rem;
		font-size: 0.75rem;
	}

	.btn-danger {
		background: #dc2626;
		color: white;
	}

	.btn-secondary {
		background: #6b7280;
		color: white;
	}

	.btn-secondary:hover {
		background: #4b5563;
	}

	.actions {
		display: flex;
		gap: 0.25rem;
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

	.modal-large {
		max-width: 900px;
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

	.lines-section {
		margin: 1.5rem 0;
	}

	.lines-section h3 {
		font-size: 1rem;
		margin-bottom: 0.75rem;
	}

	.lines-table {
		width: 100%;
		margin-bottom: 0.75rem;
	}

	.lines-table th,
	.lines-table td {
		padding: 0.5rem;
	}

	.lines-table th {
		text-align: left;
		font-weight: 500;
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}

	.input-small {
		width: 80px;
	}

	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1.5rem;
	}

	/* Mobile responsive styles */
	@media (max-width: 768px) {
		h1 {
			font-size: 1.5rem;
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

		.lines-section {
			overflow-x: auto;
		}

		.lines-table {
			min-width: 600px;
		}

		.input-small {
			width: 70px;
		}

		.modal-actions {
			flex-direction: column-reverse;
		}

		.modal-actions button {
			width: 100%;
		}
	}
</style>
