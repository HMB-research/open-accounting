<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import {
		api,
		type BalanceConfirmationType,
		type BalanceConfirmationSummary,
		type BalanceConfirmation,
		type ContactBalance
	} from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let isLoading = $state(false);
	let error = $state('');

	let balanceType = $state<BalanceConfirmationType>('RECEIVABLE');
	let asOfDate = $state(new Date().toISOString().split('T')[0]);

	let summary = $state<BalanceConfirmationSummary | null>(null);
	let selectedContact = $state<ContactBalance | null>(null);
	let contactDetail = $state<BalanceConfirmation | null>(null);
	let showDetailModal = $state(false);

	onMount(() => {
		if (tenantId) {
			loadSummary();
		}
	});

	async function loadSummary() {
		if (!tenantId) return;

		isLoading = true;
		error = '';
		selectedContact = null;
		contactDetail = null;
		try {
			summary = await api.getBalanceConfirmationSummary(tenantId, balanceType, asOfDate);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load balance confirmations';
		} finally {
			isLoading = false;
		}
	}

	async function viewContactDetail(contact: ContactBalance) {
		selectedContact = contact;
		isLoading = true;
		try {
			contactDetail = await api.getBalanceConfirmation(
				tenantId,
				contact.contact_id,
				balanceType,
				asOfDate
			);
			showDetailModal = true;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load contact details';
		} finally {
			isLoading = false;
		}
	}

	function closeModal() {
		showDetailModal = false;
		selectedContact = null;
		contactDetail = null;
	}

	function formatAmount(amount: string | undefined): string {
		if (!amount) return '0.00';
		const dec = new Decimal(amount);
		return dec.toFixed(2);
	}

	function formatDate(dateStr: string | undefined): string {
		if (!dateStr) return '-';
		return dateStr;
	}

	function printReport() {
		window.print();
	}

	function printConfirmation() {
		window.print();
	}

	function getOverdueClass(days: number): string {
		if (days > 90) return 'severely-overdue';
		if (days > 60) return 'very-overdue';
		if (days > 30) return 'overdue';
		if (days > 0) return 'slightly-overdue';
		return '';
	}
</script>

<svelte:head>
	<title>{m.balance_confirmation_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>{m.balance_confirmation_title()}</h1>
		<div class="header-actions">
			<a href="/reports?tenant={tenantId}" class="btn btn-secondary">{m.common_back()}</a>
		</div>
	</div>

	{#if !tenantId}
		<div class="card empty-state">
			<p>Select a tenant from <a href="/dashboard">{m.nav_dashboard()}</a>.</p>
		</div>
	{:else}
		<div class="card controls-section">
			<div class="controls-form">
				<div class="form-group">
					<label class="label" for="balanceType">{m.balance_confirmation_type()}</label>
					<select class="select" id="balanceType" bind:value={balanceType}>
						<option value="RECEIVABLE">{m.balance_confirmation_receivable()}</option>
						<option value="PAYABLE">{m.balance_confirmation_payable()}</option>
					</select>
				</div>
				<div class="form-group">
					<label class="label" for="asOfDate">{m.balance_confirmation_as_of_date()}</label>
					<input type="date" class="input" id="asOfDate" bind:value={asOfDate} />
				</div>
				<button class="btn btn-primary" onclick={loadSummary} disabled={isLoading}>
					{isLoading ? m.common_loading() : m.balance_confirmation_generate()}
				</button>
				{#if summary && summary.contacts.length > 0}
					<button class="btn btn-secondary" onclick={printReport}>{m.common_print()}</button>
				{/if}
			</div>
		</div>

		{#if error}
			<div class="alert alert-error">{error}</div>
		{/if}

		{#if summary}
			<div class="card summary-card">
				<div class="summary-header">
					<h2>
						{balanceType === 'RECEIVABLE'
							? m.balance_confirmation_receivables_summary()
							: m.balance_confirmation_payables_summary()}
					</h2>
					<p class="as-of-date">{m.balance_confirmation_as_of()}: {summary.as_of_date}</p>
				</div>

				<div class="summary-stats">
					<div class="stat">
						<span class="stat-label">{m.balance_confirmation_total_balance()}</span>
						<span class="stat-value">{formatAmount(summary.total_balance)} EUR</span>
					</div>
					<div class="stat">
						<span class="stat-label">{m.balance_confirmation_contact_count()}</span>
						<span class="stat-value">{summary.contact_count}</span>
					</div>
					<div class="stat">
						<span class="stat-label">{m.balance_confirmation_invoice_count()}</span>
						<span class="stat-value">{summary.invoice_count}</span>
					</div>
				</div>
			</div>

			{#if summary.contacts.length === 0}
				<div class="card empty-state">
					<p>{m.balance_confirmation_no_outstanding()}</p>
				</div>
			{:else}
				<div class="card">
					<table class="table">
						<thead>
							<tr>
								<th>{m.balance_confirmation_contact()}</th>
								<th>{m.balance_confirmation_contact_code()}</th>
								<th class="text-right">{m.balance_confirmation_invoices()}</th>
								<th class="text-right">{m.balance_confirmation_oldest()}</th>
								<th class="text-right">{m.balance_confirmation_balance()}</th>
								<th class="text-right">{m.common_actions()}</th>
							</tr>
						</thead>
						<tbody>
							{#each summary.contacts as contact}
								<tr>
									<td>
										<div class="contact-name">{contact.contact_name}</div>
										{#if contact.contact_email}
											<div class="contact-email">{contact.contact_email}</div>
										{/if}
									</td>
									<td>{contact.contact_code || '-'}</td>
									<td class="text-right">{contact.invoice_count}</td>
									<td class="text-right">{formatDate(contact.oldest_invoice)}</td>
									<td class="text-right amount">{formatAmount(contact.balance)}</td>
									<td class="text-right">
										<button
											class="btn btn-sm btn-secondary"
											onclick={() => viewContactDetail(contact)}
										>
											{m.balance_confirmation_view()}
										</button>
									</td>
								</tr>
							{/each}
						</tbody>
						<tfoot>
							<tr class="total-row">
								<td colspan="4"><strong>{m.common_total()}</strong></td>
								<td class="text-right amount">
									<strong>{formatAmount(summary.total_balance)}</strong>
								</td>
								<td></td>
							</tr>
						</tfoot>
					</table>
				</div>
			{/if}
		{/if}
	{/if}
</div>

<!-- Contact Detail Modal -->
{#if showDetailModal && contactDetail}
	<div class="modal-overlay" onclick={closeModal} role="presentation">
		<div class="modal" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
			<div class="modal-header">
				<h2>{m.balance_confirmation_detail_title()}</h2>
				<button class="btn-close" onclick={closeModal} aria-label="Close">&times;</button>
			</div>

			<div class="modal-body">
				<div class="confirmation-header">
					<div class="contact-info">
						<h3>{contactDetail.contact_name}</h3>
						{#if contactDetail.contact_code}
							<p>{m.balance_confirmation_contact_code()}: {contactDetail.contact_code}</p>
						{/if}
						{#if contactDetail.contact_email}
							<p>{m.common_email()}: {contactDetail.contact_email}</p>
						{/if}
					</div>
					<div class="confirmation-meta">
						<p>
							<strong>{m.balance_confirmation_type()}:</strong>
							{contactDetail.type === 'RECEIVABLE'
								? m.balance_confirmation_receivable()
								: m.balance_confirmation_payable()}
						</p>
						<p><strong>{m.balance_confirmation_as_of()}:</strong> {contactDetail.as_of_date}</p>
						<p class="total-balance">
							<strong>{m.balance_confirmation_total_balance()}:</strong>
							{formatAmount(contactDetail.total_balance)} EUR
						</p>
					</div>
				</div>

				<div class="invoices-table">
					<h4>{m.balance_confirmation_outstanding_invoices()}</h4>
					<table class="table">
						<thead>
							<tr>
								<th>{m.invoices_invoice_number()}</th>
								<th>{m.invoices_invoice_date()}</th>
								<th>{m.invoices_due_date()}</th>
								<th class="text-right">{m.invoices_total()}</th>
								<th class="text-right">{m.balance_confirmation_paid()}</th>
								<th class="text-right">{m.balance_confirmation_outstanding()}</th>
								<th class="text-right">{m.balance_confirmation_days_overdue()}</th>
							</tr>
						</thead>
						<tbody>
							{#each contactDetail.invoices as invoice}
								<tr class={getOverdueClass(invoice.days_overdue)}>
									<td>{invoice.invoice_number}</td>
									<td>{invoice.invoice_date}</td>
									<td>{invoice.due_date}</td>
									<td class="text-right">{formatAmount(invoice.total_amount)}</td>
									<td class="text-right">{formatAmount(invoice.amount_paid)}</td>
									<td class="text-right amount">{formatAmount(invoice.outstanding_amount)}</td>
									<td class="text-right">
										{#if invoice.days_overdue > 0}
											<span class="overdue-badge">{invoice.days_overdue}</span>
										{:else}
											-
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
						<tfoot>
							<tr class="total-row">
								<td colspan="5"><strong>{m.common_total()}</strong></td>
								<td class="text-right amount">
									<strong>{formatAmount(contactDetail.total_balance)}</strong>
								</td>
								<td></td>
							</tr>
						</tfoot>
					</table>
				</div>
			</div>

			<div class="modal-footer">
				<button class="btn btn-secondary" onclick={printConfirmation}>{m.common_print()}</button>
				<button class="btn btn-primary" onclick={closeModal}>{m.common_close()}</button>
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

	.controls-section {
		margin-bottom: 1.5rem;
	}

	.controls-form {
		display: flex;
		gap: 1rem;
		align-items: flex-end;
		flex-wrap: wrap;
	}

	.controls-form .form-group {
		flex: 1;
		min-width: 150px;
		max-width: 200px;
	}

	.summary-card {
		margin-bottom: 1.5rem;
	}

	.summary-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1rem;
		padding-bottom: 0.75rem;
		border-bottom: 1px solid var(--color-border);
	}

	.summary-header h2 {
		font-size: 1.25rem;
	}

	.as-of-date {
		color: var(--color-text-muted);
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

	.text-right {
		text-align: right;
	}

	.amount {
		font-family: var(--font-mono);
	}

	.contact-name {
		font-weight: 500;
	}

	.contact-email {
		font-size: 0.875rem;
		color: var(--color-text-muted);
	}

	.total-row {
		background: var(--color-bg);
		font-weight: 600;
	}

	.btn-sm {
		padding: 0.25rem 0.5rem;
		font-size: 0.875rem;
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
		max-width: 900px;
		width: 100%;
		max-height: 90vh;
		overflow-y: auto;
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

	.confirmation-header {
		display: flex;
		justify-content: space-between;
		margin-bottom: 1.5rem;
		padding-bottom: 1rem;
		border-bottom: 1px solid var(--color-border);
	}

	.contact-info h3 {
		margin: 0 0 0.5rem;
		font-size: 1.25rem;
	}

	.contact-info p {
		margin: 0.25rem 0;
		color: var(--color-text-muted);
	}

	.confirmation-meta {
		text-align: right;
	}

	.confirmation-meta p {
		margin: 0.25rem 0;
	}

	.total-balance {
		font-size: 1.1rem;
		color: var(--color-primary);
	}

	.invoices-table h4 {
		margin-bottom: 0.75rem;
		color: var(--color-text-muted);
	}

	.empty-state {
		text-align: center;
		padding: 2rem;
		color: var(--color-text-muted);
	}

	@media print {
		.header-actions,
		.controls-section,
		.modal-overlay {
			display: none;
		}
	}

	@media (max-width: 768px) {
		.header {
			flex-direction: column;
			align-items: flex-start;
			gap: 1rem;
		}

		.controls-form {
			flex-direction: column;
		}

		.controls-form .form-group {
			max-width: none;
			width: 100%;
		}

		.summary-stats {
			flex-direction: column;
			gap: 1rem;
		}

		.confirmation-header {
			flex-direction: column;
			gap: 1rem;
		}

		.confirmation-meta {
			text-align: left;
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
