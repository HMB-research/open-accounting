<script lang="ts">
	import { onMount } from 'svelte';
	import {
		api,
		type RecurringInvoice,
		type Contact,
		type Frequency,
		type CreateRecurringInvoiceRequest
	} from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';

	let recurringInvoices = $state<RecurringInvoice[]>([]);
	let contacts = $state<Contact[]>([]);
	let isLoading = $state(true);
	let error = $state('');
	let selectedTenantId = $state('');
	let showActiveOnly = $state(false);
	let showCreateModal = $state(false);

	// Create form state
	let newName = $state('');
	let newContactId = $state('');
	let newFrequency = $state<Frequency>('MONTHLY');
	let newStartDate = $state(new Date().toISOString().split('T')[0]);
	let newEndDate = $state('');
	let newPaymentTermsDays = $state(14);
	let newReference = $state('');
	let newNotes = $state('');
	let newLines = $state<
		{ description: string; quantity: string; unit_price: string; vat_rate: string }[]
	>([{ description: '', quantity: '1', unit_price: '', vat_rate: '22' }]);

	// Email configuration state
	let sendEmailOnGeneration = $state(false);
	let attachPdfToEmail = $state(true);
	let recipientEmailOverride = $state('');
	let emailSubjectOverride = $state('');
	let emailMessage = $state('');

	onMount(async () => {
		const urlParams = new URLSearchParams(window.location.search);
		selectedTenantId = urlParams.get('tenant') || '';

		if (!selectedTenantId) {
			const tenants = await api.getMyTenants();
			if (tenants.length > 0) {
				selectedTenantId = tenants.find((t) => t.is_default)?.tenant.id || tenants[0].tenant.id;
			}
		}

		if (selectedTenantId) {
			await loadData();
		}
		isLoading = false;
	});

	async function loadData() {
		try {
			const [invoicesData, contactsData] = await Promise.all([
				api.listRecurringInvoices(selectedTenantId, showActiveOnly),
				api.listContacts(selectedTenantId, { active_only: true })
			]);
			recurringInvoices = invoicesData;
			contacts = contactsData;
		} catch (err) {
			error = err instanceof Error ? err.message : String(m.recurring_failedToLoad());
		}
	}

	async function handleToggleActive(invoice: RecurringInvoice) {
		try {
			if (invoice.is_active) {
				await api.pauseRecurringInvoice(selectedTenantId, invoice.id);
			} else {
				await api.resumeRecurringInvoice(selectedTenantId, invoice.id);
			}
			await loadData();
		} catch (err) {
			error = err instanceof Error ? err.message : String(m.recurring_failedToUpdateStatus());
		}
	}

	async function handleGenerate(invoice: RecurringInvoice) {
		try {
			const result = await api.generateRecurringInvoice(selectedTenantId, invoice.id);
			let message: string = m.recurring_generatedInvoice({ number: result.generated_invoice_number });
			if (result.email_sent) {
				message += '\n' + m.recurring_emailSent();
			} else if (result.email_status === 'FAILED') {
				message += '\n' + m.recurring_emailFailed() + (result.email_error ? ': ' + result.email_error : '');
			} else if (result.email_status === 'SKIPPED') {
				message += '\n' + m.recurring_emailSkipped();
			}
			alert(message);
			await loadData();
		} catch (err) {
			error = err instanceof Error ? err.message : String(m.recurring_failedToGenerate());
		}
	}

	async function handleDelete(invoice: RecurringInvoice) {
		if (!confirm(m.recurring_deleteConfirm({ name: invoice.name }))) return;
		try {
			await api.deleteRecurringInvoice(selectedTenantId, invoice.id);
			await loadData();
		} catch (err) {
			error = err instanceof Error ? err.message : String(m.recurring_failedToDelete());
		}
	}

	async function handleCreate(e: Event) {
		e.preventDefault();
		error = '';

		const data: CreateRecurringInvoiceRequest = {
			name: newName,
			contact_id: newContactId,
			frequency: newFrequency,
			start_date: newStartDate,
			payment_terms_days: newPaymentTermsDays,
			reference: newReference || undefined,
			notes: newNotes || undefined,
			lines: newLines.map((l) => ({
				description: l.description,
				quantity: l.quantity,
				unit_price: l.unit_price,
				vat_rate: l.vat_rate
			})),
			// Email configuration
			send_email_on_generation: sendEmailOnGeneration,
			attach_pdf_to_email: attachPdfToEmail,
			recipient_email_override: recipientEmailOverride || undefined,
			email_subject_override: emailSubjectOverride || undefined,
			email_message: emailMessage || undefined
		};

		if (newEndDate) {
			data.end_date = newEndDate;
		}

		try {
			await api.createRecurringInvoice(selectedTenantId, data);
			showCreateModal = false;
			resetForm();
			await loadData();
		} catch (err) {
			error = err instanceof Error ? err.message : String(m.recurring_failedToCreate());
		}
	}

	function resetForm() {
		newName = '';
		newContactId = '';
		newFrequency = 'MONTHLY';
		newStartDate = new Date().toISOString().split('T')[0];
		newEndDate = '';
		newPaymentTermsDays = 14;
		newReference = '';
		newNotes = '';
		newLines = [{ description: '', quantity: '1', unit_price: '', vat_rate: '22' }];
		// Reset email config
		sendEmailOnGeneration = false;
		attachPdfToEmail = true;
		recipientEmailOverride = '';
		emailSubjectOverride = '';
		emailMessage = '';
	}

	function addLine() {
		newLines = [...newLines, { description: '', quantity: '1', unit_price: '', vat_rate: '22' }];
	}

	function removeLine(index: number) {
		if (newLines.length > 1) {
			newLines = newLines.filter((_, i) => i !== index);
		}
	}

	function formatCurrency(value: Decimal | number | string): string {
		const num = typeof value === 'object' && 'toFixed' in value ? value.toNumber() : Number(value);
		return new Intl.NumberFormat('et-EE', {
			style: 'currency',
			currency: 'EUR'
		}).format(num);
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('en-GB');
	}

	function frequencyLabel(f: Frequency): string {
		switch (f) {
			case 'WEEKLY': return m.recurring_weekly();
			case 'BIWEEKLY': return m.recurring_biweekly();
			case 'MONTHLY': return m.recurring_monthly();
			case 'QUARTERLY': return m.recurring_quarterly();
			case 'YEARLY': return m.recurring_yearly();
			default: return f;
		}
	}
</script>

<svelte:head>
	<title>Recurring Invoices - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>{m.recurring_title()}</h1>
		<div class="header-actions">
			<label class="checkbox-label">
				<input type="checkbox" bind:checked={showActiveOnly} onchange={loadData} />
				{m.recurring_activeOnly()}
			</label>
			<button class="btn btn-primary" onclick={() => (showCreateModal = true)}>
				{m.recurring_newRecurring()}
			</button>
		</div>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<p>{m.common_loading()}</p>
	{:else if recurringInvoices.length === 0}
		<div class="card empty-state">
			<h2>{m.recurring_noRecurringInvoices()}</h2>
			<p>{m.recurring_createFirst()}</p>
			<button class="btn btn-primary" onclick={() => (showCreateModal = true)}>
				{m.recurring_createRecurringInvoice()}
			</button>
		</div>
	{:else}
		<div class="table-container">
			<table class="table">
				<thead>
					<tr>
						<th>{m.common_name()}</th>
						<th>{m.recurring_contact()}</th>
						<th>{m.recurring_frequency()}</th>
						<th>{m.recurring_nextGeneration()}</th>
						<th>{m.recurring_generated()}</th>
						<th>{m.common_email()}</th>
						<th>{m.common_status()}</th>
						<th>{m.common_actions()}</th>
					</tr>
				</thead>
				<tbody>
					{#each recurringInvoices as invoice}
						<tr>
							<td>
								<strong>{invoice.name}</strong>
								{#if invoice.reference}
									<br /><small class="text-muted">{invoice.reference}</small>
								{/if}
							</td>
							<td>{invoice.contact_name || '-'}</td>
							<td>{frequencyLabel(invoice.frequency)}</td>
							<td>{formatDate(invoice.next_generation_date)}</td>
							<td>{invoice.generated_count}</td>
							<td>
								{#if invoice.send_email_on_generation}
									<span class="badge badge-info" title={m.recurring_emailEnabled()}>
										{invoice.attach_pdf_to_email ? '📧📎' : '📧'}
									</span>
								{:else}
									<span class="text-muted">-</span>
								{/if}
							</td>
							<td>
								<span class="badge" class:badge-success={invoice.is_active} class:badge-muted={!invoice.is_active}>
									{invoice.is_active ? m.recurring_active() : m.recurring_paused()}
								</span>
							</td>
							<td class="actions">
								<button
									class="btn btn-secondary btn-sm"
									onclick={() => handleGenerate(invoice)}
									title={m.recurring_generateNow()}
								>
									{m.recurring_generate()}
								</button>
								<button
									class="btn btn-secondary btn-sm"
									onclick={() => handleToggleActive(invoice)}
								>
									{invoice.is_active ? m.recurring_pause() : m.recurring_resume()}
								</button>
								<button
									class="btn btn-danger btn-sm"
									onclick={() => handleDelete(invoice)}
								>
									{m.common_delete()}
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

{#if showCreateModal}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showCreateModal = false)} role="presentation">
		<div class="modal card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="create-recurring-title" tabindex="-1">
			<h2 id="create-recurring-title">{m.recurring_createRecurringInvoice()}</h2>
			<form onsubmit={handleCreate}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="name">{m.common_name()}</label>
						<input
							class="input"
							type="text"
							id="name"
							bind:value={newName}
							required
							placeholder={m.recurring_namePlaceholder()}
						/>
					</div>
					<div class="form-group">
						<label class="label" for="contact">{m.recurring_contact()}</label>
						<select class="input" id="contact" bind:value={newContactId} required>
							<option value="">{m.recurring_selectContact()}</option>
							{#each contacts as contact}
								<option value={contact.id}>{contact.name}</option>
							{/each}
						</select>
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="frequency">{m.recurring_frequency()}</label>
						<select class="input" id="frequency" bind:value={newFrequency}>
							<option value="WEEKLY">{m.recurring_weekly()}</option>
							<option value="BIWEEKLY">{m.recurring_biweekly()}</option>
							<option value="MONTHLY">{m.recurring_monthly()}</option>
							<option value="QUARTERLY">{m.recurring_quarterly()}</option>
							<option value="YEARLY">{m.recurring_yearly()}</option>
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="payment_terms">{m.recurring_paymentTermsDays()}</label>
						<input
							class="input"
							type="number"
							id="payment_terms"
							bind:value={newPaymentTermsDays}
							min="0"
						/>
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="start_date">{m.recurring_startDate()}</label>
						<input
							class="input"
							type="date"
							id="start_date"
							bind:value={newStartDate}
							required
						/>
					</div>
					<div class="form-group">
						<label class="label" for="end_date">{m.recurring_endDateOptional()}</label>
						<input class="input" type="date" id="end_date" bind:value={newEndDate} />
					</div>
				</div>

				<div class="form-group">
					<label class="label" for="reference">{m.recurring_reference()}</label>
					<input class="input" type="text" id="reference" bind:value={newReference} />
				</div>

				<h3>{m.recurring_lineItems()}</h3>
				{#each newLines as line, index}
					<div class="line-row">
						<input
							class="input"
							type="text"
							placeholder={m.recurring_descriptionPlaceholder()}
							bind:value={line.description}
							required
						/>
						<input
							class="input input-sm"
							type="number"
							placeholder={m.recurring_qtyPlaceholder()}
							bind:value={line.quantity}
							step="0.01"
							required
						/>
						<input
							class="input input-sm"
							type="number"
							placeholder={m.recurring_unitPricePlaceholder()}
							bind:value={line.unit_price}
							step="0.01"
							required
						/>
						<input
							class="input input-sm"
							type="number"
							placeholder={m.recurring_vatPlaceholder()}
							bind:value={line.vat_rate}
							step="0.01"
							required
						/>
						<button
							type="button"
							class="btn btn-danger btn-sm"
							onclick={() => removeLine(index)}
							disabled={newLines.length === 1}
						>
							-
						</button>
					</div>
				{/each}
				<button type="button" class="btn btn-secondary" onclick={addLine}>{m.recurring_addLine()}</button>

				<h3>{m.recurring_emailSettings()}</h3>
				<div class="form-group">
					<label class="checkbox-label">
						<input type="checkbox" bind:checked={sendEmailOnGeneration} />
						{m.recurring_sendEmailOnGeneration()}
					</label>
				</div>

				{#if sendEmailOnGeneration}
					<div class="form-group">
						<label class="checkbox-label">
							<input type="checkbox" bind:checked={attachPdfToEmail} />
							{m.recurring_attachPdf()}
						</label>
					</div>

					<div class="form-group">
						<label class="label" for="recipient_override">{m.recurring_recipientOverride()}</label>
						<input
							class="input"
							type="email"
							id="recipient_override"
							bind:value={recipientEmailOverride}
							placeholder={m.recurring_recipientOverridePlaceholder()}
						/>
					</div>

					<div class="form-group">
						<label class="label" for="email_subject">{m.recurring_emailSubject()}</label>
						<input
							class="input"
							type="text"
							id="email_subject"
							bind:value={emailSubjectOverride}
							placeholder={m.recurring_emailSubjectPlaceholder()}
						/>
					</div>

					<div class="form-group">
						<label class="label" for="email_message">{m.recurring_emailMessageLabel()}</label>
						<textarea
							class="input"
							id="email_message"
							bind:value={emailMessage}
							rows="3"
							placeholder={m.recurring_emailMessagePlaceholder()}
						></textarea>
					</div>
				{/if}

				<div class="modal-actions">
					<button
						type="button"
						class="btn btn-secondary"
						onclick={() => (showCreateModal = false)}
					>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary">{m.common_create()}</button>
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

	.header-actions {
		display: flex;
		gap: 1rem;
		align-items: center;
	}

	.checkbox-label {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		cursor: pointer;
	}

	h1 {
		font-size: 1.75rem;
	}

	.empty-state {
		text-align: center;
		padding: 3rem;
	}

	.empty-state h2 {
		margin-bottom: 0.5rem;
	}

	.empty-state p {
		color: var(--color-text-muted);
		margin-bottom: 1.5rem;
	}

	.table-container {
		overflow-x: auto;
	}

	.table {
		width: 100%;
		border-collapse: collapse;
	}

	.table th,
	.table td {
		padding: 0.75rem;
		text-align: left;
		border-bottom: 1px solid var(--color-border);
	}

	.table th {
		font-weight: 600;
		color: var(--color-text-muted);
		font-size: 0.75rem;
		text-transform: uppercase;
	}

	.text-muted {
		color: var(--color-text-muted);
	}

	.badge {
		display: inline-block;
		padding: 0.25rem 0.5rem;
		border-radius: var(--radius-sm);
		font-size: 0.75rem;
		font-weight: 500;
	}

	.badge-success {
		background: rgba(34, 197, 94, 0.1);
		color: #22c55e;
	}

	.badge-muted {
		background: var(--color-border);
		color: var(--color-text-muted);
	}

	.badge-info {
		background: rgba(59, 130, 246, 0.1);
		color: #3b82f6;
	}

	.actions {
		display: flex;
		gap: 0.25rem;
	}

	.btn-sm {
		padding: 0.25rem 0.5rem;
		font-size: 0.75rem;
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
		max-height: 90vh;
		overflow-y: auto;
		margin: 1rem;
	}

	.modal h2 {
		margin-bottom: 1.5rem;
	}

	.modal h3 {
		margin: 1.5rem 0 1rem;
		font-size: 1rem;
	}

	.form-row {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1rem;
	}

	.line-row {
		display: grid;
		grid-template-columns: 2fr 1fr 1fr 1fr auto;
		gap: 0.5rem;
		margin-bottom: 0.5rem;
	}

	.input-sm {
		width: 100%;
	}

	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1.5rem;
	}
</style>
