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
			error = err instanceof Error ? err.message : 'Failed to load data';
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
			error = err instanceof Error ? err.message : 'Failed to update status';
		}
	}

	async function handleGenerate(invoice: RecurringInvoice) {
		try {
			const result = await api.generateRecurringInvoice(selectedTenantId, invoice.id);
			alert(`Generated invoice: ${result.generated_invoice_number}`);
			await loadData();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to generate invoice';
		}
	}

	async function handleDelete(invoice: RecurringInvoice) {
		if (!confirm(`Delete recurring invoice "${invoice.name}"?`)) return;
		try {
			await api.deleteRecurringInvoice(selectedTenantId, invoice.id);
			await loadData();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete';
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
			}))
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
			error = err instanceof Error ? err.message : 'Failed to create recurring invoice';
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
		const labels: Record<Frequency, string> = {
			WEEKLY: 'Weekly',
			BIWEEKLY: 'Bi-weekly',
			MONTHLY: 'Monthly',
			QUARTERLY: 'Quarterly',
			YEARLY: 'Yearly'
		};
		return labels[f] || f;
	}
</script>

<svelte:head>
	<title>Recurring Invoices - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>Recurring Invoices</h1>
		<div class="header-actions">
			<label class="checkbox-label">
				<input type="checkbox" bind:checked={showActiveOnly} onchange={loadData} />
				Active only
			</label>
			<button class="btn btn-primary" onclick={() => (showCreateModal = true)}>
				+ New Recurring Invoice
			</button>
		</div>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<p>Loading...</p>
	{:else if recurringInvoices.length === 0}
		<div class="card empty-state">
			<h2>No recurring invoices</h2>
			<p>Create your first recurring invoice to automate billing.</p>
			<button class="btn btn-primary" onclick={() => (showCreateModal = true)}>
				Create Recurring Invoice
			</button>
		</div>
	{:else}
		<div class="table-container">
			<table class="table">
				<thead>
					<tr>
						<th>Name</th>
						<th>Contact</th>
						<th>Frequency</th>
						<th>Next Generation</th>
						<th>Generated</th>
						<th>Status</th>
						<th>Actions</th>
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
								<span class="badge" class:badge-success={invoice.is_active} class:badge-muted={!invoice.is_active}>
									{invoice.is_active ? 'Active' : 'Paused'}
								</span>
							</td>
							<td class="actions">
								<button
									class="btn btn-secondary btn-sm"
									onclick={() => handleGenerate(invoice)}
									title="Generate invoice now"
								>
									Generate
								</button>
								<button
									class="btn btn-secondary btn-sm"
									onclick={() => handleToggleActive(invoice)}
								>
									{invoice.is_active ? 'Pause' : 'Resume'}
								</button>
								<button
									class="btn btn-danger btn-sm"
									onclick={() => handleDelete(invoice)}
								>
									Delete
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
			<h2 id="create-recurring-title">Create Recurring Invoice</h2>
			<form onsubmit={handleCreate}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="name">Name</label>
						<input
							class="input"
							type="text"
							id="name"
							bind:value={newName}
							required
							placeholder="Monthly Hosting Fee"
						/>
					</div>
					<div class="form-group">
						<label class="label" for="contact">Contact</label>
						<select class="input" id="contact" bind:value={newContactId} required>
							<option value="">Select contact</option>
							{#each contacts as contact}
								<option value={contact.id}>{contact.name}</option>
							{/each}
						</select>
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="frequency">Frequency</label>
						<select class="input" id="frequency" bind:value={newFrequency}>
							<option value="WEEKLY">Weekly</option>
							<option value="BIWEEKLY">Bi-weekly</option>
							<option value="MONTHLY">Monthly</option>
							<option value="QUARTERLY">Quarterly</option>
							<option value="YEARLY">Yearly</option>
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="payment_terms">Payment Terms (days)</label>
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
						<label class="label" for="start_date">Start Date</label>
						<input
							class="input"
							type="date"
							id="start_date"
							bind:value={newStartDate}
							required
						/>
					</div>
					<div class="form-group">
						<label class="label" for="end_date">End Date (optional)</label>
						<input class="input" type="date" id="end_date" bind:value={newEndDate} />
					</div>
				</div>

				<div class="form-group">
					<label class="label" for="reference">Reference</label>
					<input class="input" type="text" id="reference" bind:value={newReference} />
				</div>

				<h3>Line Items</h3>
				{#each newLines as line, index}
					<div class="line-row">
						<input
							class="input"
							type="text"
							placeholder="Description"
							bind:value={line.description}
							required
						/>
						<input
							class="input input-sm"
							type="number"
							placeholder="Qty"
							bind:value={line.quantity}
							step="0.01"
							required
						/>
						<input
							class="input input-sm"
							type="number"
							placeholder="Unit Price"
							bind:value={line.unit_price}
							step="0.01"
							required
						/>
						<input
							class="input input-sm"
							type="number"
							placeholder="VAT %"
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
				<button type="button" class="btn btn-secondary" onclick={addLine}>+ Add Line</button>

				<div class="modal-actions">
					<button
						type="button"
						class="btn btn-secondary"
						onclick={() => (showCreateModal = false)}
					>
						Cancel
					</button>
					<button type="submit" class="btn btn-primary">Create</button>
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
