<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { api, type OverdueInvoicesSummary, type OverdueInvoice, type ReminderResult } from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let isLoading = $state(false);
	let isSending = $state(false);
	let error = $state('');
	let successMessage = $state('');

	let summary = $state<OverdueInvoicesSummary | null>(null);
	let selectedInvoices = $state<Set<string>>(new Set());
	let customMessage = $state('');
	let showMessageModal = $state(false);
	let sendMode = $state<'single' | 'bulk'>('single');
	let currentInvoice = $state<OverdueInvoice | null>(null);

	onMount(() => {
		if (tenantId) {
			loadOverdueInvoices();
		}
	});

	async function loadOverdueInvoices() {
		isLoading = true;
		error = '';
		successMessage = '';
		selectedInvoices = new Set();
		try {
			summary = await api.getOverdueInvoices(tenantId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load overdue invoices';
		} finally {
			isLoading = false;
		}
	}

	function formatAmount(amount: string | undefined): string {
		if (!amount) return '0.00';
		return new Decimal(amount).toFixed(2);
	}

	function toggleInvoice(invoiceId: string) {
		const newSet = new Set(selectedInvoices);
		if (newSet.has(invoiceId)) {
			newSet.delete(invoiceId);
		} else {
			newSet.add(invoiceId);
		}
		selectedInvoices = newSet;
	}

	function toggleAll() {
		if (!summary) return;
		if (selectedInvoices.size === summary.invoices.length) {
			selectedInvoices = new Set();
		} else {
			selectedInvoices = new Set(summary.invoices.map((inv) => inv.id));
		}
	}

	function openSendModal(invoice: OverdueInvoice) {
		currentInvoice = invoice;
		sendMode = 'single';
		customMessage = '';
		showMessageModal = true;
	}

	function openBulkSendModal() {
		if (selectedInvoices.size === 0) {
			error = m.reminder_select_at_least_one();
			return;
		}
		sendMode = 'bulk';
		customMessage = '';
		showMessageModal = true;
	}

	function closeModal() {
		showMessageModal = false;
		currentInvoice = null;
		customMessage = '';
	}

	async function sendReminder() {
		isSending = true;
		error = '';
		successMessage = '';

		try {
			if (sendMode === 'single' && currentInvoice) {
				const result = await api.sendPaymentReminder(
					tenantId,
					currentInvoice.id,
					customMessage || undefined
				);
				if (result.success) {
					successMessage = m.reminder_sent_success({ invoice: result.invoice_number });
				} else {
					error = result.message;
				}
			} else if (sendMode === 'bulk') {
				const invoiceIds = Array.from(selectedInvoices);
				const result = await api.sendBulkPaymentReminders(
					tenantId,
					invoiceIds,
					customMessage || undefined
				);
				if (result.successful > 0) {
					successMessage = m.reminder_bulk_sent({ sent: result.successful, total: result.total_requested });
				}
				if (result.failed > 0) {
					error = m.reminder_bulk_failed({ failed: result.failed });
				}
			}

			closeModal();
			await loadOverdueInvoices();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to send reminder';
		} finally {
			isSending = false;
		}
	}

	function getOverdueClass(days: number): string {
		if (days > 90) return 'severely-overdue';
		if (days > 60) return 'very-overdue';
		if (days > 30) return 'overdue';
		if (days > 0) return 'slightly-overdue';
		return '';
	}

	function canSendReminder(invoice: OverdueInvoice): boolean {
		return !!invoice.contact_email;
	}
</script>

<svelte:head>
	<title>{m.reminder_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>{m.reminder_title()}</h1>
		<div class="header-actions">
			<button class="btn btn-secondary" onclick={loadOverdueInvoices} disabled={isLoading}>
				{isLoading ? m.common_loading() : m.common_refresh()}
			</button>
			<a href="/invoices?tenant={tenantId}" class="btn btn-secondary">{m.common_back()}</a>
		</div>
	</div>

	{#if !tenantId}
		<div class="card empty-state">
			<p>Select a tenant from <a href="/dashboard">{m.nav_dashboard()}</a>.</p>
		</div>
	{:else}
		{#if error}
			<div class="alert alert-error">{error}</div>
		{/if}

		{#if successMessage}
			<div class="alert alert-success">{successMessage}</div>
		{/if}

		{#if summary}
			<div class="card summary-card">
				<div class="summary-header">
					<h2>{m.reminder_overdue_summary()}</h2>
				</div>

				<div class="summary-stats">
					<div class="stat">
						<span class="stat-label">{m.reminder_total_overdue()}</span>
						<span class="stat-value danger">{formatAmount(summary.total_overdue)} EUR</span>
					</div>
					<div class="stat">
						<span class="stat-label">{m.reminder_invoice_count()}</span>
						<span class="stat-value">{summary.invoice_count}</span>
					</div>
					<div class="stat">
						<span class="stat-label">{m.reminder_contact_count()}</span>
						<span class="stat-value">{summary.contact_count}</span>
					</div>
					<div class="stat">
						<span class="stat-label">{m.reminder_avg_days_overdue()}</span>
						<span class="stat-value">{summary.average_days_overdue}</span>
					</div>
				</div>
			</div>

			{#if summary.invoices.length === 0}
				<div class="card empty-state">
					<p>{m.reminder_no_overdue()}</p>
				</div>
			{:else}
				<div class="card">
					<div class="table-actions">
						<button
							class="btn btn-primary"
							onclick={openBulkSendModal}
							disabled={selectedInvoices.size === 0}
						>
							{m.reminder_send_selected()} ({selectedInvoices.size})
						</button>
					</div>

					<table class="table">
						<thead>
							<tr>
								<th class="checkbox-col">
									<input
										type="checkbox"
										checked={selectedInvoices.size === summary.invoices.length}
										onchange={toggleAll}
									/>
								</th>
								<th>{m.reminder_invoice()}</th>
								<th>{m.reminder_contact()}</th>
								<th class="text-right">{m.reminder_outstanding()}</th>
								<th class="text-right">{m.reminder_days_overdue()}</th>
								<th class="text-right">{m.reminder_reminders_sent()}</th>
								<th class="text-right">{m.common_actions()}</th>
							</tr>
						</thead>
						<tbody>
							{#each summary.invoices as invoice}
								<tr class={getOverdueClass(invoice.days_overdue)}>
									<td class="checkbox-col">
										<input
											type="checkbox"
											checked={selectedInvoices.has(invoice.id)}
											onchange={() => toggleInvoice(invoice.id)}
											disabled={!canSendReminder(invoice)}
										/>
									</td>
									<td>
										<div class="invoice-number">{invoice.invoice_number}</div>
										<div class="invoice-dates">
											{m.reminder_due()}: {invoice.due_date}
										</div>
									</td>
									<td>
										<div class="contact-name">{invoice.contact_name}</div>
										{#if invoice.contact_email}
											<div class="contact-email">{invoice.contact_email}</div>
										{:else}
											<div class="contact-no-email">{m.reminder_no_email()}</div>
										{/if}
									</td>
									<td class="text-right amount">{formatAmount(invoice.outstanding_amount)} {invoice.currency}</td>
									<td class="text-right">
										<span class="overdue-badge">{invoice.days_overdue}</span>
									</td>
									<td class="text-right">
										{#if invoice.reminder_count > 0}
											<span class="reminder-count">{invoice.reminder_count}</span>
										{:else}
											<span class="no-reminders">-</span>
										{/if}
									</td>
									<td class="text-right">
										<button
											class="btn btn-sm btn-primary"
											onclick={() => openSendModal(invoice)}
											disabled={!canSendReminder(invoice)}
											title={canSendReminder(invoice) ? m.reminder_send() : m.reminder_no_email()}
										>
											{m.reminder_send()}
										</button>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		{/if}
	{/if}
</div>

<!-- Send Reminder Modal -->
{#if showMessageModal}
	<div class="modal-overlay" onclick={closeModal} role="presentation">
		<div class="modal" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
			<div class="modal-header">
				<h2>{sendMode === 'single' ? m.reminder_send_single() : m.reminder_send_bulk()}</h2>
				<button class="btn-close" onclick={closeModal} aria-label="Close">&times;</button>
			</div>

			<div class="modal-body">
				{#if sendMode === 'single' && currentInvoice}
					<div class="reminder-preview">
						<p>
							<strong>{m.reminder_invoice()}:</strong> {currentInvoice.invoice_number}
						</p>
						<p>
							<strong>{m.reminder_contact()}:</strong> {currentInvoice.contact_name}
						</p>
						<p>
							<strong>{m.common_email()}:</strong> {currentInvoice.contact_email}
						</p>
						<p>
							<strong>{m.reminder_outstanding()}:</strong> {formatAmount(currentInvoice.outstanding_amount)} {currentInvoice.currency}
						</p>
					</div>
				{:else}
					<p>{m.reminder_bulk_description({ count: selectedInvoices.size })}</p>
				{/if}

				<div class="form-group">
					<label class="label" for="customMessage">{m.reminder_custom_message()}</label>
					<textarea
						class="textarea"
						id="customMessage"
						bind:value={customMessage}
						placeholder={m.reminder_message_placeholder()}
						rows="3"
					></textarea>
					<small class="hint">{m.reminder_message_hint()}</small>
				</div>
			</div>

			<div class="modal-footer">
				<button class="btn btn-secondary" onclick={closeModal} disabled={isSending}>
					{m.common_cancel()}
				</button>
				<button class="btn btn-primary" onclick={sendReminder} disabled={isSending}>
					{isSending ? m.reminder_sending() : m.reminder_send_now()}
				</button>
			</div>
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

	.header-actions {
		display: flex;
		gap: 0.5rem;
	}

	.summary-card {
		margin-bottom: 1.5rem;
	}

	.summary-header h2 {
		font-size: 1.25rem;
		margin-bottom: 1rem;
	}

	.summary-stats {
		display: flex;
		gap: 2rem;
		flex-wrap: wrap;
	}

	.stat {
		display: flex;
		flex-direction: column;
	}

	.stat-label {
		font-size: 0.875rem;
		color: var(--color-text-muted);
	}

	.stat-value {
		font-size: 1.5rem;
		font-weight: 600;
		color: var(--color-primary);
	}

	.stat-value.danger {
		color: var(--color-error);
	}

	.table-actions {
		margin-bottom: 1rem;
	}

	.table {
		width: 100%;
		border-collapse: collapse;
	}

	.table th,
	.table td {
		padding: 0.75rem;
		border-bottom: 1px solid var(--color-border);
		text-align: left;
	}

	.table th {
		font-weight: 600;
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}

	.checkbox-col {
		width: 40px;
	}

	.text-right {
		text-align: right;
	}

	.amount {
		font-family: var(--font-mono);
	}

	.invoice-number {
		font-weight: 500;
	}

	.invoice-dates {
		font-size: 0.875rem;
		color: var(--color-text-muted);
	}

	.contact-name {
		font-weight: 500;
	}

	.contact-email {
		font-size: 0.875rem;
		color: var(--color-text-muted);
	}

	.contact-no-email {
		font-size: 0.875rem;
		color: var(--color-error);
		font-style: italic;
	}

	/* Overdue styling */
	.slightly-overdue {
		background: rgba(255, 193, 7, 0.1);
	}

	.overdue {
		background: rgba(255, 152, 0, 0.15);
	}

	.very-overdue {
		background: rgba(244, 67, 54, 0.1);
	}

	.severely-overdue {
		background: rgba(244, 67, 54, 0.2);
	}

	.overdue-badge {
		display: inline-block;
		padding: 0.125rem 0.5rem;
		background: var(--color-error);
		color: white;
		border-radius: 1rem;
		font-size: 0.75rem;
		font-weight: 600;
	}

	.reminder-count {
		display: inline-block;
		padding: 0.125rem 0.5rem;
		background: var(--color-warning);
		color: white;
		border-radius: 1rem;
		font-size: 0.75rem;
		font-weight: 600;
	}

	.no-reminders {
		color: var(--color-text-muted);
	}

	.btn-sm {
		padding: 0.25rem 0.5rem;
		font-size: 0.875rem;
	}

	/* Modal */
	.modal-overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.5);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 1000;
		padding: 1rem;
	}

	.modal {
		background: var(--color-surface);
		border-radius: 0.5rem;
		max-width: 500px;
		width: 100%;
		box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
	}

	.modal-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 1rem 1.5rem;
		border-bottom: 1px solid var(--color-border);
	}

	.modal-header h2 {
		font-size: 1.25rem;
		margin: 0;
	}

	.btn-close {
		background: none;
		border: none;
		font-size: 1.5rem;
		cursor: pointer;
		color: var(--color-text-muted);
		padding: 0;
		line-height: 1;
	}

	.btn-close:hover {
		color: var(--color-text);
	}

	.modal-body {
		padding: 1.5rem;
	}

	.modal-footer {
		display: flex;
		justify-content: flex-end;
		gap: 0.75rem;
		padding: 1rem 1.5rem;
		border-top: 1px solid var(--color-border);
	}

	.reminder-preview {
		background: var(--color-bg);
		padding: 1rem;
		border-radius: 0.5rem;
		margin-bottom: 1rem;
	}

	.reminder-preview p {
		margin: 0.25rem 0;
	}

	.textarea {
		width: 100%;
		padding: 0.5rem;
		border: 1px solid var(--color-border);
		border-radius: 0.25rem;
		font-family: inherit;
		resize: vertical;
	}

	.hint {
		display: block;
		margin-top: 0.25rem;
		color: var(--color-text-muted);
		font-size: 0.75rem;
	}

	.empty-state {
		text-align: center;
		padding: 2rem;
		color: var(--color-text-muted);
	}

	.alert-success {
		background: rgba(76, 175, 80, 0.1);
		color: #2e7d32;
		padding: 0.75rem 1rem;
		border-radius: 0.25rem;
		margin-bottom: 1rem;
	}

	@media (max-width: 768px) {
		.header {
			flex-direction: column;
			align-items: flex-start;
			gap: 1rem;
		}

		.summary-stats {
			flex-direction: column;
			gap: 1rem;
		}

		.table {
			font-size: 0.875rem;
		}

		.table th,
		.table td {
			padding: 0.5rem;
		}
	}
</style>
