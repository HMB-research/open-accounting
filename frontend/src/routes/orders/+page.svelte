<script lang="ts">
	import { page } from '$app/stores';
	import { api, type Order, type OrderStatus, type Contact } from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';
	import DateRangeFilter from '$lib/components/DateRangeFilter.svelte';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import StatusBadge, { type StatusConfig } from '$lib/components/StatusBadge.svelte';
	import { dateInputToApiTimestamp } from '$lib/utils/dates';
	import { requireTenantId, parseApiError } from '$lib/utils/tenant';
	import {
		formatCurrency,
		formatDate,
		formStringValue,
		calculateLineTotal as calcLineTotal,
		calculateLinesTotal,
		createEmptyLine,
		type LineItem
	} from '$lib/utils/formatting';

	let orders = $state<Order[]>([]);
	let contacts = $state<Contact[]>([]);
	let isLoading = $state(true);
	let error = $state('');
	let success = $state('');
	let actionLoading = $state(false);
	let showCreateOrder = $state(false);
	let filterStatus = $state<OrderStatus | ''>('');
	let filterFromDate = $state('');
	let filterToDate = $state('');
	let emailOrderTarget = $state<Order | null>(null);
	let emailRecipientEmail = $state('');
	let emailRecipientName = $state('');
	let emailSubject = $state('');
	let emailMessage = $state('');
	let emailAttachPDF = $state(true);
	let emailRequireApprovedEvidence = $state(false);

	// New order form
	let newContactId = $state('');
	let newOrderDate = $state(new Date().toISOString().split('T')[0]);
	let newExpectedDelivery = $state('');
	let newNotes = $state('');
	let newLines = $state<LineItem[]>([createEmptyLine()]);

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
			const [orderData, contactData] = await Promise.all([
				api.listOrders(tenantId, {
					status: filterStatus || undefined,
					from_date: filterFromDate || undefined,
					to_date: filterToDate || undefined
				}),
				api.listContacts(tenantId, { active_only: true })
			]);
			orders = orderData;
			contacts = contactData;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load data';
		} finally {
			isLoading = false;
		}
	}

	function addLine() {
		newLines = [...newLines, createEmptyLine()];
	}

	function removeLine(index: number) {
		if (newLines.length > 1) {
			newLines = newLines.filter((_, i) => i !== index);
		}
	}

	let orderTotal = $derived(calculateLinesTotal(newLines));

	async function createOrder(e: Event) {
		e.preventDefault();
		const tenantId = requireTenantId($page, (err) => (error = err));
		if (!tenantId) return;

		actionLoading = true;
		error = '';
		try {
			const order = await api.createOrder(tenantId, {
				contact_id: newContactId,
				order_date: dateInputToApiTimestamp(newOrderDate),
				expected_delivery: newExpectedDelivery ? dateInputToApiTimestamp(newExpectedDelivery) : undefined,
				notes: newNotes || undefined,
				lines: newLines.map((line) => ({
					description: line.description,
					quantity: formStringValue(line.quantity),
					unit_price: formStringValue(line.unit_price),
					vat_rate: formStringValue(line.vat_rate),
					discount_percent: formStringValue(line.discount_percent || '0')
				}))
			});
			orders = [order, ...orders];
			showCreateOrder = false;
			resetForm();
			success = m.orders_orderCreated();
			setTimeout(() => (success = ''), 3000);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionLoading = false;
		}
	}

	function resetForm() {
		newContactId = '';
		newOrderDate = new Date().toISOString().split('T')[0];
		newExpectedDelivery = '';
		newNotes = '';
		newLines = [createEmptyLine()];
	}

	async function handleFilter() {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadData(tenantId);
		}
	}

	async function confirmOrder(orderId: string) {
		const tenantId = requireTenantId($page, (err) => (error = err));
		if (!tenantId) return;

		actionLoading = true;
		error = '';
		try {
			await api.confirmOrder(tenantId, orderId);
			success = m.orders_statusConfirmed();
			setTimeout(() => (success = ''), 3000);
			loadData(tenantId);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionLoading = false;
		}
	}

	async function downloadOrderPDF(order: Order) {
		const tenantId = requireTenantId($page, (err) => (error = err));
		if (!tenantId) return;

		actionLoading = true;
		error = '';
		try {
			await api.downloadOrderPDF(tenantId, order.id, order.order_number);
			success = m.orders_pdfDownloaded();
			setTimeout(() => (success = ''), 3000);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionLoading = false;
		}
	}

	function openEmailOrder(order: Order) {
		const contact = getContact(order.contact_id);
		emailOrderTarget = order;
		emailRecipientEmail = contact?.email || '';
		emailRecipientName = contact?.name || '';
		emailSubject = `Order ${order.order_number}`;
		emailMessage = '';
		emailAttachPDF = true;
		emailRequireApprovedEvidence = false;
	}

	async function sendOrderEmail(e: Event) {
		e.preventDefault();
		const tenantId = requireTenantId($page, (err) => (error = err));
		if (!tenantId || !emailOrderTarget) return;

		actionLoading = true;
		error = '';
		try {
			await api.emailOrder(tenantId, emailOrderTarget.id, {
				recipient_email: emailRecipientEmail.trim(),
				recipient_name: emailRecipientName.trim() || undefined,
				subject: emailSubject.trim() || undefined,
				message: emailMessage.trim() || undefined,
				attach_pdf: emailAttachPDF,
				require_approved_evidence: emailRequireApprovedEvidence
			});
			emailOrderTarget = null;
			success = m.orders_emailSent();
			setTimeout(() => (success = ''), 3000);
			loadData(tenantId);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionLoading = false;
		}
	}

	async function processOrder(orderId: string) {
		const tenantId = requireTenantId($page, (err) => (error = err));
		if (!tenantId) return;

		actionLoading = true;
		error = '';
		try {
			await api.processOrder(tenantId, orderId);
			success = m.orders_statusProcessing();
			setTimeout(() => (success = ''), 3000);
			loadData(tenantId);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionLoading = false;
		}
	}

	async function shipOrder(orderId: string) {
		const tenantId = requireTenantId($page, (err) => (error = err));
		if (!tenantId) return;

		actionLoading = true;
		error = '';
		try {
			await api.shipOrder(tenantId, orderId);
			success = m.orders_statusShipped();
			setTimeout(() => (success = ''), 3000);
			loadData(tenantId);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionLoading = false;
		}
	}

	async function deliverOrder(orderId: string) {
		const tenantId = requireTenantId($page, (err) => (error = err));
		if (!tenantId) return;

		actionLoading = true;
		error = '';
		try {
			await api.deliverOrder(tenantId, orderId);
			success = m.orders_statusDelivered();
			setTimeout(() => (success = ''), 3000);
			loadData(tenantId);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionLoading = false;
		}
	}

	async function convertOrderToInvoice(orderId: string) {
		const tenantId = requireTenantId($page, (err) => (error = err));
		if (!tenantId) return;

		actionLoading = true;
		error = '';
		try {
			const result = await api.convertOrderToInvoice(tenantId, orderId);
			success = m.orders_convertedToInvoice({ number: result.invoice.invoice_number });
			setTimeout(() => (success = ''), 3000);
			loadData(tenantId);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionLoading = false;
		}
	}

	async function cancelOrder(orderId: string) {
		const tenantId = requireTenantId($page, (err) => (error = err));
		if (!tenantId) return;

		if (!confirm(m.orders_confirmCancel())) return;

		actionLoading = true;
		error = '';
		try {
			await api.cancelOrder(tenantId, orderId);
			success = m.orders_statusCancelled();
			setTimeout(() => (success = ''), 3000);
			loadData(tenantId);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionLoading = false;
		}
	}

	async function deleteOrder(orderId: string) {
		const tenantId = requireTenantId($page, (err) => (error = err));
		if (!tenantId) return;

		if (!confirm(m.orders_confirmDelete())) return;

		actionLoading = true;
		error = '';
		try {
			await api.deleteOrder(tenantId, orderId);
			orders = orders.filter(o => o.id !== orderId);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionLoading = false;
		}
	}

	const statusConfig: Record<OrderStatus, StatusConfig> = {
		PENDING: { class: 'badge-pending', label: m.orders_statusPending() },
		CONFIRMED: { class: 'badge-confirmed', label: m.orders_statusConfirmed() },
		PROCESSING: { class: 'badge-processing', label: m.orders_statusProcessing() },
		SHIPPED: { class: 'badge-shipped', label: m.orders_statusShipped() },
		DELIVERED: { class: 'badge-delivered', label: m.orders_statusDelivered() },
		CANCELED: { class: 'badge-cancelled', label: m.orders_statusCancelled() }
	};

	function getContact(contactId: string): Contact | undefined {
		return contacts.find((c) => c.id === contactId);
	}

	function getContactName(contactId: string): string {
		const contact = getContact(contactId);
		return contact?.name || '-';
	}
</script>

<svelte:head>
	<title>{m.orders_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="page-header">
		<h1>{m.orders_title()}</h1>
		<div class="page-actions">
			<button class="btn btn-primary" onclick={() => (showCreateOrder = true)}>
				+ {m.orders_newOrder()}
			</button>
		</div>
	</div>

	<div class="filters card">
		<div class="filter-row">
			<select class="input" bind:value={filterStatus} onchange={handleFilter}>
				<option value="">{m.orders_allOrders()}</option>
				<option value="PENDING">{m.orders_statusPending()}</option>
				<option value="CONFIRMED">{m.orders_statusConfirmed()}</option>
				<option value="PROCESSING">{m.orders_statusProcessing()}</option>
				<option value="SHIPPED">{m.orders_statusShipped()}</option>
				<option value="DELIVERED">{m.orders_statusDelivered()}</option>
				<option value="CANCELED">{m.orders_statusCancelled()}</option>
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
	{:else if orders.length === 0}
		<div class="empty-state card">
			<p>{m.orders_noOrders()}</p>
		</div>
	{:else}
		<div class="card">
			<div class="table-container">
				<table class="table table-mobile-cards">
					<thead>
						<tr>
							<th>{m.orders_number()}</th>
							<th>{m.invoices_status()}</th>
							<th class="hide-mobile">{m.invoices_customer()}</th>
							<th>{m.common_date()}</th>
							<th class="hide-mobile">{m.orders_expectedDelivery()}</th>
							<th class="text-right">{m.common_total()}</th>
							<th class="hide-mobile">{m.common_actions()}</th>
						</tr>
					</thead>
					<tbody>
						{#each orders as order (order.id)}
							<tr>
								<td class="number" data-label={m.orders_number()}>{order.order_number}</td>
								<td data-label={m.invoices_status()}>
									<StatusBadge status={order.status} config={statusConfig} />
								</td>
								<td class="hide-mobile" data-label={m.invoices_customer()}>{getContactName(order.contact_id)}</td>
								<td data-label={m.common_date()}>{formatDate(order.order_date)}</td>
								<td class="hide-mobile" data-label={m.orders_expectedDelivery()}>
									{order.expected_delivery ? formatDate(order.expected_delivery) : '-'}
								</td>
								<td class="amount text-right" data-label={m.common_total()}>{formatCurrency(order.total)}</td>
								<td class="actions hide-mobile" data-label={m.common_actions()}>
									<button class="btn btn-small" onclick={() => downloadOrderPDF(order)} disabled={actionLoading} title={m.invoices_downloadPdf()}>
										PDF
									</button>
									<button class="btn btn-small" onclick={() => openEmailOrder(order)} disabled={actionLoading} title={m.orders_emailOrder()}>
										{m.orders_email()}
									</button>
									{#if order.status === 'PENDING'}
										<button class="btn btn-small btn-success" onclick={() => confirmOrder(order.id)} disabled={actionLoading} title={m.orders_confirm()}>
											{m.orders_confirm()}
										</button>
										<button class="btn btn-small btn-danger" onclick={() => deleteOrder(order.id)} disabled={actionLoading} title={m.common_delete()}>
											{m.common_delete()}
										</button>
									{:else if order.status === 'CONFIRMED'}
										<button class="btn btn-small" onclick={() => processOrder(order.id)} disabled={actionLoading} title={m.orders_process()}>
											{m.orders_process()}
										</button>
										<button class="btn btn-small" onclick={() => shipOrder(order.id)} disabled={actionLoading} title={m.orders_ship()}>
											{m.orders_ship()}
										</button>
										<button class="btn btn-small btn-danger" onclick={() => cancelOrder(order.id)} disabled={actionLoading} title={m.orders_cancel()}>
											{m.orders_cancel()}
										</button>
									{:else if order.status === 'PROCESSING'}
										<button class="btn btn-small" onclick={() => shipOrder(order.id)} disabled={actionLoading} title={m.orders_ship()}>
											{m.orders_ship()}
										</button>
										<button class="btn btn-small btn-danger" onclick={() => cancelOrder(order.id)} disabled={actionLoading} title={m.orders_cancel()}>
											{m.orders_cancel()}
										</button>
									{:else if order.status === 'SHIPPED'}
										<button class="btn btn-small btn-success" onclick={() => deliverOrder(order.id)} disabled={actionLoading} title={m.orders_deliver()}>
											{m.orders_deliver()}
										</button>
									{:else if order.status === 'DELIVERED' && !order.converted_to_invoice_id}
										<button class="btn btn-small btn-success" onclick={() => convertOrderToInvoice(order.id)} disabled={actionLoading} title={m.orders_convertToInvoice()}>
											{m.orders_convertToInvoice()}
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

{#if showCreateOrder}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showCreateOrder = false)} role="presentation">
		<div class="modal card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="create-order-title" tabindex="-1">
			<h2 id="create-order-title">{m.orders_newOrder()}</h2>
			<form onsubmit={createOrder}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="contact">{m.invoices_customer()} *</label>
						<select class="input" id="contact" bind:value={newContactId} required>
							<option value="">{m.invoices_selectContact()}</option>
							{#each contacts as contact (contact.id)}
								<option value={contact.id}>{contact.name}</option>
							{/each}
						</select>
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="order-date">{m.orders_orderDate()}</label>
						<input class="input" type="date" id="order-date" bind:value={newOrderDate} required />
					</div>
					<div class="form-group">
						<label class="label" for="expected-delivery">{m.orders_expectedDelivery()}</label>
						<input class="input" type="date" id="expected-delivery" bind:value={newExpectedDelivery} />
					</div>
				</div>

				<div class="form-group">
					<span class="label">{m.invoices_lineItems()}</span>
					<div class="lines-container">
						{#each newLines as line, i (line)}
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
								<span class="line-total">{formatCurrency(calcLineTotal(line))}</span>
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

				<div class="order-totals">
					<div class="total-row">
						<span>{m.common_total()}:</span>
						<span class="total-value">{formatCurrency(orderTotal)}</span>
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
					<button type="button" class="btn btn-secondary" onclick={() => (showCreateOrder = false)}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary" disabled={actionLoading}>{m.orders_createOrder()}</button>
				</div>
			</form>
		</div>
	</div>
{/if}

{#if emailOrderTarget}
	<div class="modal-backdrop">
		<div class="modal card" role="dialog" aria-modal="true" aria-labelledby="email-order-title" tabindex="-1">
			<h2 id="email-order-title">{m.orders_emailOrder()}</h2>
			<form onsubmit={sendOrderEmail}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="order-email-recipient">{m.common_email()} *</label>
						<input class="input" id="order-email-recipient" type="email" bind:value={emailRecipientEmail} required />
					</div>
					<div class="form-group">
						<label class="label" for="order-email-name">{m.common_name()}</label>
						<input class="input" id="order-email-name" bind:value={emailRecipientName} />
					</div>
				</div>

				<div class="form-group">
					<label class="label" for="order-email-subject">{m.email_subject()}</label>
					<input class="input" id="order-email-subject" bind:value={emailSubject} />
				</div>

				<div class="form-group">
					<label class="label" for="order-email-message">{m.recurring_emailMessageLabel()}</label>
					<textarea class="input" id="order-email-message" bind:value={emailMessage} rows="3"></textarea>
				</div>

				<label class="checkbox-row">
					<input type="checkbox" bind:checked={emailAttachPDF} />
					<span>{m.orders_attachPdf()}</span>
				</label>
				<label class="checkbox-row">
					<input type="checkbox" bind:checked={emailRequireApprovedEvidence} />
					<span>{m.orders_requireApprovedEvidence()}</span>
				</label>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={() => (emailOrderTarget = null)}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary" disabled={actionLoading}>
						{m.orders_email()}
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

	.order-totals {
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

	.checkbox-row {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		margin-top: 0.75rem;
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
