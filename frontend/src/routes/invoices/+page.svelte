<script lang="ts">
	import { browser } from '$app/environment';
	import { page } from '$app/stores';
	import {
		api,
		type Contact,
		type ImportInvoicesResult,
		type Invoice,
		type InvoiceStatus,
		type InvoiceType,
		type Tenant
	} from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';
	import DateRangeFilter from '$lib/components/DateRangeFilter.svelte';
	import ContactFormModal from '$lib/components/ContactFormModal.svelte';
	import DocumentManager from '$lib/components/DocumentManager.svelte';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import StatusBadge, { type StatusConfig } from '$lib/components/StatusBadge.svelte';
	import WorkflowHero, { type WorkflowHeroAction, type WorkflowHeroAside, type WorkflowHeroStat } from '$lib/components/WorkflowHero.svelte';
	import { requireTenantId, parseApiError } from '$lib/utils/tenant';
	import {
		formatCurrency,
		formatDate,
		calculateLineTotal as calcLineTotal,
		calculateLinesTotal,
		createEmptyLine,
		type LineItem
	} from '$lib/utils/formatting';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let invoices = $state<Invoice[]>([]);
	let contacts = $state<Contact[]>([]);
	let tenant = $state<Tenant | null>(null);
	let isLoading = $state(true);
	let error = $state('');
	let success = $state('');
	let actionLoading = $state(false);
	let showCreateInvoice = $state(false);
	let showImportInvoices = $state(false);
	let filterType = $state<InvoiceType | ''>('');
	let filterStatus = $state<InvoiceStatus | ''>('');
	let filterFromDate = $state('');
	let filterToDate = $state('');
	let showContactModal = $state(false);
	let showDocumentManager = $state(false);
	let importError = $state('');
	let importFileName = $state('');
	let importCSVContent = $state('');
	let isImporting = $state(false);
	let importResult = $state<ImportInvoicesResult | null>(null);

	// New invoice form
	let newType = $state<InvoiceType>('SALES');
	let newContactId = $state('');
	let newIssueDate = $state(new Date().toISOString().split('T')[0]);
	let newDueDate = $state(new Date(Date.now() + 14 * 24 * 60 * 60 * 1000).toISOString().split('T')[0]);
	let newReference = $state('');
	let newNotes = $state('');
	let newLines = $state<LineItem[]>([createEmptyLine()]);
	let selectedInvoiceForDocuments = $state<Invoice | null>(null);

	function decimalValue(value: Decimal | number | string | undefined | null): Decimal {
		if (value instanceof Decimal) return value;
		return new Decimal(value || 0);
	}

	let draftCount = $derived(invoices.filter((invoice) => invoice.status === 'DRAFT').length);
	let overdueCount = $derived(invoices.filter((invoice) => invoice.status === 'OVERDUE').length);
	let openBalance = $derived.by(() =>
		invoices.reduce((sum, invoice) => {
			if (invoice.status === 'PAID' || invoice.status === 'VOIDED') {
				return sum;
			}

			return sum.plus(decimalValue(invoice.total).minus(decimalValue(invoice.amount_paid)));
		}, new Decimal(0))
	);
	let currentPeriodLockDate = $derived(tenant?.settings?.period_lock_date || '');

	let heroStats = $derived.by<WorkflowHeroStat[]>(() => [
		{
			label: m.common_total(),
			value: String(invoices.length),
			detail: m.invoices_totalInvoicesHint()
		},
		{
			label: m.invoices_openBalance(),
			value: formatCurrency(openBalance),
			tone: openBalance.greaterThan(0) ? 'warning' : 'default'
		},
		{
			label: m.invoices_overdue(),
			value: String(overdueCount),
			tone: overdueCount > 0 ? 'danger' : 'success'
		},
		{
			label: m.settings_periodLockDate(),
			value: currentPeriodLockDate ? formatDate(currentPeriodLockDate) : m.settings_periodOpenStatus(),
			tone: currentPeriodLockDate ? 'warning' : 'success',
			href: tenantId ? `/settings/company?tenant=${tenantId}` : undefined
		}
	]);

	let heroActions = $derived.by<WorkflowHeroAction[]>(() => [
		{
			label: m.invoices_importInvoices(),
			variant: 'secondary',
			onclick: openImportModal,
			disabled: !tenantId
		},
		{
			label: m.invoices_newInvoice(),
			onclick: () => (showCreateInvoice = true),
			disabled: !tenantId
		}
	]);

	let heroAside = $derived.by<WorkflowHeroAside>(() => {
		if (contacts.length === 0) {
			return {
				kicker: m.contacts_title(),
				title: m.invoices_contactsFirstTitle(),
				body: m.invoices_contactsFirstDesc(),
				linkLabel: m.contacts_newContact(),
				href: tenantId ? `/contacts?tenant=${tenantId}` : '/contacts',
				items: [m.invoices_contactsFirstItemOne(), m.invoices_contactsFirstItemTwo()]
			};
		}

		if (currentPeriodLockDate) {
			return {
				kicker: m.settings_accountingControls(),
				title: m.invoices_closeStatusTitle(),
				body: m.invoices_closeStatusDesc({ date: formatDate(currentPeriodLockDate) }),
				linkLabel: m.invoices_manageCloseControls(),
				href: tenantId ? `/settings/company?tenant=${tenantId}` : '/settings/company',
				items: [m.invoices_closeStatusItemOne(), m.invoices_closeStatusItemTwo()]
			};
		}

		return {
			kicker: m.dashboard_setupCenter(),
			title: m.invoices_importLegacyTitle(),
			body: m.invoices_importLegacyDesc(),
			linkLabel: m.invoices_importInvoices(),
			href: tenantId ? `/invoices?tenant=${tenantId}` : '/invoices',
			items: [m.invoices_importLegacyItemOne(), m.invoices_importLegacyItemTwo()]
		};
	});

	$effect(() => {
		if (tenantId) {
			loadData(tenantId);
		}
	});

	async function loadData(tenantId: string) {
		isLoading = true;
		error = '';

	try {
			const [invoiceData, contactData, tenantData] = await Promise.all([
				api.listInvoices(tenantId, {
					type: filterType || undefined,
					status: filterStatus || undefined,
					from_date: filterFromDate || undefined,
					to_date: filterToDate || undefined
				}),
				api.listContacts(tenantId, { active_only: true }),
				api.getTenant(tenantId)
			]);
			invoices = invoiceData;
			contacts = contactData;
			tenant = tenantData;
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

	function openImportModal() {
		showImportInvoices = true;
		importError = '';
		importFileName = '';
		importCSVContent = '';
		importResult = null;
	}

	function closeImportModal() {
		showImportInvoices = false;
		importError = '';
		importFileName = '';
		importCSVContent = '';
		importResult = null;
	}

	function openDocumentManager(invoice: Invoice) {
		selectedInvoiceForDocuments = invoice;
		showDocumentManager = true;
	}

	function closeDocumentManager() {
		showDocumentManager = false;
		selectedInvoiceForDocuments = null;
	}

	async function handleImportFileChange(event: Event) {
		const input = event.currentTarget as HTMLInputElement | null;
		const file = input?.files?.[0];

		importResult = null;

		if (!file) {
			importFileName = '';
			importCSVContent = '';
			return;
		}

		importFileName = file.name;
		importCSVContent = await file.text();
		importError = '';
	}

	async function submitImport(event: Event) {
		event.preventDefault();

		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		if (!importCSVContent.trim()) {
			importError = m.invoices_importFileRequired();
			return;
		}

		isImporting = true;
		importError = '';

		try {
			importResult = await api.importInvoices(tenantId, {
				file_name: importFileName || undefined,
				csv_content: importCSVContent
			});

			if (importResult.invoices_created > 0) {
				await loadData(tenantId);
			}
		} catch (err) {
			importError = err instanceof Error ? err.message : 'Failed to import invoices';
		} finally {
			isImporting = false;
		}
	}

	function downloadImportTemplate() {
		if (!browser) return;

		const template = [
			'invoice_number,invoice_type,contact_code,contact_name,issue_date,due_date,status,amount_paid,reference,notes,line_description,quantity,unit,unit_price,discount_percent,vat_rate',
			'INV-EXT-001,SALES,CUST-001,Example Customer,2026-02-01,2026-02-15,SENT,0,PO-12345,Imported migration invoice,Implementation work,1,hour,100.00,0,22',
			'INV-EXT-001,SALES,CUST-001,Example Customer,2026-02-01,2026-02-15,SENT,0,PO-12345,Imported migration invoice,Support retainer,1,month,50.00,0,22'
		].join('\n');

		const blob = new Blob([template], { type: 'text/csv;charset=utf-8' });
		const url = window.URL.createObjectURL(blob);
		const link = document.createElement('a');
		link.href = url;
		link.download = 'invoices-import-template.csv';
		document.body.appendChild(link);
		link.click();
		document.body.removeChild(link);
		window.URL.revokeObjectURL(url);
	}

	// Use imported calcLineTotal for line calculations
	// calculateLinesTotal for the total

	async function createInvoice(e: Event) {
		e.preventDefault();
		const tenantId = requireTenantId($page, (err) => (error = err));
		if (!tenantId) return;

		actionLoading = true;
		error = '';
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
			success = m.invoices_newInvoice();
			setTimeout(() => (success = ''), 3000);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionLoading = false;
		}
	}

	function resetForm() {
		newType = 'SALES';
		newContactId = '';
		newIssueDate = new Date().toISOString().split('T')[0];
		newDueDate = new Date(Date.now() + 14 * 24 * 60 * 60 * 1000).toISOString().split('T')[0];
		newReference = '';
		newNotes = '';
		newLines = [createEmptyLine()];
	}

	function handleNewContactSaved(contact: Contact) {
		// Add the new contact to the list and sort alphabetically
		contacts = [...contacts, contact].sort((a, b) => a.name.localeCompare(b.name));
		// Select the newly created contact
		newContactId = contact.id;
		// Close the contact modal
		showContactModal = false;
	}

	async function handleFilter() {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadData(tenantId);
		}
	}

	async function sendInvoice(invoiceId: string) {
		const tenantId = requireTenantId($page, (err) => (error = err));
		if (!tenantId) return;

		actionLoading = true;
		error = '';
		try {
			await api.sendInvoice(tenantId, invoiceId);
			success = m.invoices_sent();
			setTimeout(() => (success = ''), 3000);
			loadData(tenantId);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionLoading = false;
		}
	}

	async function downloadPDF(invoiceId: string, invoiceNumber: string) {
		const tenantId = requireTenantId($page, (err) => (error = err));
		if (!tenantId) return;

		actionLoading = true;
		error = '';
		try {
			await api.downloadInvoicePDF(tenantId, invoiceId, invoiceNumber);
		} catch (err) {
			error = parseApiError(err);
		} finally {
			actionLoading = false;
		}
	}

	function getTypeLabel(type: InvoiceType): string {
		switch (type) {
			case 'SALES': return m.invoices_salesInvoice();
			case 'PURCHASE': return m.invoices_purchaseInvoice();
			case 'CREDIT_NOTE': return m.invoices_creditNote();
		}
	}

	// Status configuration for StatusBadge component
	const statusConfig: Record<InvoiceStatus, StatusConfig> = {
		DRAFT: { class: 'badge-draft', label: m.invoices_draft() },
		SENT: { class: 'badge-sent', label: m.invoices_sent() },
		PARTIALLY_PAID: { class: 'badge-partial', label: m.invoices_partial() },
		PAID: { class: 'badge-paid', label: m.invoices_paid() },
		OVERDUE: { class: 'badge-overdue', label: m.invoices_overdue() },
		VOIDED: { class: 'badge-voided', label: m.invoices_voided() }
	};

	// formatCurrency and formatDate imported from $lib/utils/formatting

	function getContactName(invoice: Invoice): string {
		// First try the populated contact object
		if (invoice.contact?.name) return invoice.contact.name;
		// Fall back to looking up from loaded contacts
		if (invoice.contact_id) {
			const contact = contacts.find(c => c.id === invoice.contact_id);
			if (contact) return contact.name;
		}
		return '-';
	}
</script>

<svelte:head>
	<title>{m.invoices_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<WorkflowHero
		eyebrow={m.dashboard_invoiceStatus()}
		title={m.invoices_title()}
		description={m.invoices_heroDesc()}
		badgeLabel={draftCount > 0 ? m.invoices_draftBadge({ count: String(draftCount) }) : m.invoices_readyToSend()}
		badgeTone={draftCount > 0 ? 'warning' : 'success'}
		actions={heroActions}
		stats={heroStats}
		aside={heroAside}
	/>

	<div class="filters card">
		<div class="filter-row">
			<select class="input" bind:value={filterType} onchange={handleFilter}>
				<option value="">{m.common_filter()} - {m.invoices_title()}</option>
				<option value="SALES">{m.invoices_salesInvoice()}</option>
				<option value="PURCHASE">{m.invoices_purchaseInvoice()}</option>
				<option value="CREDIT_NOTE">{m.invoices_creditNote()}</option>
			</select>
			<select class="input" bind:value={filterStatus} onchange={handleFilter}>
				<option value="">{m.common_filter()} - {m.common_status()}</option>
				<option value="DRAFT">{m.invoices_draft()}</option>
				<option value="SENT">{m.invoices_sent()}</option>
				<option value="PARTIALLY_PAID">{m.invoices_partial()}</option>
				<option value="PAID">{m.invoices_paid()}</option>
				<option value="OVERDUE">{m.invoices_overdue()}</option>
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
	{:else if invoices.length === 0}
		<div class="empty-state card">
			<h2>{m.invoices_noInvoices()}</h2>
			<p>{m.invoices_createFirst()}</p>
			<div class="empty-actions">
				<button class="btn btn-secondary" onclick={openImportModal}>
					{m.invoices_importInvoices()}
				</button>
				<button class="btn btn-primary" onclick={() => (showCreateInvoice = true)}>
					{m.invoices_newInvoice()}
				</button>
			</div>
		</div>
	{:else}
		<div class="card">
			<div class="table-container">
				<table class="table table-mobile-cards">
					<thead>
						<tr>
							<th>{m.invoices_invoiceNumber()}</th>
							<th class="hide-mobile">{m.accounts_accountType()}</th>
							<th>{m.invoices_customer()}</th>
							<th class="hide-mobile">{m.invoices_issueDate()}</th>
							<th>{m.invoices_dueDate()}</th>
							<th>{m.common_total()}</th>
							<th>{m.common_status()}</th>
							<th>{m.common_actions()}</th>
						</tr>
					</thead>
					<tbody>
						{#each invoices as invoice}
							<tr>
								<td class="number" data-label="Number">{invoice.invoice_number}</td>
								<td class="hide-mobile" data-label="Type">{getTypeLabel(invoice.invoice_type)}</td>
								<td data-label="Contact">{getContactName(invoice)}</td>
								<td class="hide-mobile" data-label="Issue Date">{formatDate(invoice.issue_date)}</td>
								<td data-label="Due Date">{formatDate(invoice.due_date)}</td>
								<td class="amount" data-label="Total">{formatCurrency(invoice.total)}</td>
								<td data-label="Status">
									<StatusBadge status={invoice.status} config={statusConfig} />
								</td>
								<td class="actions actions-cell">
									<button
										class="btn btn-small btn-secondary"
										onclick={() => openDocumentManager(invoice)}
									>
										{m.documents_manageAction()}
									</button>
									<button
										class="btn btn-small btn-secondary"
										onclick={() => downloadPDF(invoice.id, invoice.invoice_number)}
										title="Download PDF"
									>
										PDF
									</button>
									{#if invoice.status === 'DRAFT'}
										<button class="btn btn-small" onclick={() => sendInvoice(invoice.id)}>
											{m.invoices_send()}
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

<ContactFormModal
	open={showContactModal}
	tenantId={$page.url.searchParams.get('tenant') || ''}
	onSave={handleNewContactSaved}
	onClose={() => (showContactModal = false)}
/>

<DocumentManager
	open={showDocumentManager}
	tenantId={tenantId}
	entityType="invoice"
	entityId={selectedInvoiceForDocuments?.id || ''}
	title={
		selectedInvoiceForDocuments
			? m.documents_invoiceTitle({ number: selectedInvoiceForDocuments.invoice_number })
			: m.documents_manageAction()
	}
	onClose={closeDocumentManager}
/>

{#if showImportInvoices}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={closeImportModal} role="presentation">
		<div class="modal card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="import-invoices-title" tabindex="-1">
			<h2 id="import-invoices-title">{m.invoices_importInvoices()}</h2>
			<p class="modal-description">{m.invoices_importDescription()}</p>

			<form onsubmit={submitImport}>
				<div class="form-group">
					<label class="label" for="invoice-import-file">{m.invoices_importChooseFile()}</label>
					<input
						class="input"
						id="invoice-import-file"
						type="file"
						accept=".csv,text/csv"
						onchange={handleImportFileChange}
					/>
					<div class="help-text">{m.invoices_importTemplateHint()}</div>
				</div>

				<div class="modal-actions import-actions">
					<button type="button" class="btn btn-secondary" onclick={downloadImportTemplate}>
						{m.invoices_importTemplate()}
					</button>
					<button type="button" class="btn btn-secondary" onclick={closeImportModal}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary" disabled={isImporting}>
						{isImporting ? m.invoices_importing() : m.invoices_importInvoices()}
					</button>
				</div>
			</form>

			{#if importFileName}
				<p class="selected-file">{m.invoices_importSelectedFile()}: <strong>{importFileName}</strong></p>
			{/if}

			{#if importError}
				<div class="alert alert-error">{importError}</div>
			{/if}

			{#if importResult}
				<div class="import-summary">
					<h3>{m.invoices_importSummary()}</h3>
					<div class="import-stats">
						<div class="import-stat">
							<span class="summary-label">{m.invoices_importRowsProcessed()}</span>
							<strong>{importResult.rows_processed}</strong>
						</div>
						<div class="import-stat">
							<span class="summary-label">{m.invoices_importInvoicesCreated()}</span>
							<strong>{importResult.invoices_created}</strong>
						</div>
						<div class="import-stat">
							<span class="summary-label">{m.invoices_importLinesImported()}</span>
							<strong>{importResult.lines_imported}</strong>
						</div>
						<div class="import-stat">
							<span class="summary-label">{m.invoices_importRowsSkipped()}</span>
							<strong>{importResult.rows_skipped}</strong>
						</div>
					</div>

					{#if importResult.errors?.length}
						<div class="import-errors">
							<h4>{m.invoices_importErrors()}</h4>
							<ul>
								{#each importResult.errors as rowError}
									<li>
										<strong>{m.invoices_importRow()} {rowError.row}</strong>
										{#if rowError.invoice_number}
											<span> · {rowError.invoice_number}</span>
										{/if}
										<div>{m.invoices_importMessage()}: {rowError.message}</div>
									</li>
								{/each}
							</ul>
						</div>
					{/if}
				</div>
			{/if}
		</div>
	</div>
{/if}

{#if showCreateInvoice}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showCreateInvoice = false)} role="presentation">
		<div class="modal modal-large card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="create-invoice-title" tabindex="-1">
			<h2 id="create-invoice-title">{m.invoices_newInvoice()}</h2>
			<form onsubmit={createInvoice}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="type">{m.accounts_accountType()}</label>
						<select class="input" id="type" bind:value={newType}>
							<option value="SALES">{m.invoices_salesInvoice()}</option>
							<option value="PURCHASE">{m.invoices_purchaseInvoice()}</option>
							<option value="CREDIT_NOTE">{m.invoices_creditNote()}</option>
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="contact">{m.invoices_customer()} *</label>
						<div class="contact-select-row">
							<select class="input" id="contact" bind:value={newContactId} required>
								<option value="">{m.invoices_selectContact()}</option>
								{#each contacts as contact (contact.id)}
									<option value={contact.id}>{contact.name}</option>
								{/each}
							</select>
							<button
								type="button"
								class="btn btn-secondary btn-new-contact"
								onclick={() => (showContactModal = true)}
								title={m.contacts_newContact()}
							>
								+
							</button>
						</div>
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="issue-date">{m.invoices_issueDate()}</label>
						<input class="input" type="date" id="issue-date" bind:value={newIssueDate} required />
					</div>
					<div class="form-group">
						<label class="label" for="due-date">{m.invoices_dueDate()}</label>
						<input class="input" type="date" id="due-date" bind:value={newDueDate} required />
					</div>
					<div class="form-group">
						<label class="label" for="reference">{m.payments_reference()}</label>
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
					<h3>{m.invoices_lineItems()}</h3>
					<table class="lines-table">
						<thead>
							<tr>
								<th>{m.common_description()}</th>
								<th>{m.invoices_qty()}</th>
								<th>{m.invoices_unitPrice()}</th>
								<th>{m.invoices_vat()} %</th>
								<th>{m.invoices_discount()} %</th>
								<th>{m.common_total()}</th>
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
											placeholder={m.invoices_productOrService()}
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
									<td class="amount">{formatCurrency(calcLineTotal(line))}</td>
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
								<td colspan="5" style="text-align: right;"><strong>{m.common_total()}:</strong></td>
								<td class="amount"><strong>{formatCurrency(calculateLinesTotal(newLines))}</strong></td>
								<td></td>
							</tr>
						</tfoot>
					</table>
					<button type="button" class="btn btn-secondary" onclick={addLine}>+ {m.invoices_addLine()}</button>
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
					<button type="button" class="btn btn-secondary" onclick={() => (showCreateInvoice = false)}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary">{m.invoices_newInvoice()}</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<style>
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

	.empty-state h2 {
		color: var(--color-text);
		margin-bottom: 0.5rem;
	}

	.empty-actions {
		display: flex;
		justify-content: center;
		gap: 0.75rem;
		flex-wrap: wrap;
		margin-top: 1.25rem;
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

	.modal-description {
		margin-bottom: 1rem;
		color: var(--color-text-muted);
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

	.contact-select-row {
		display: flex;
		gap: 0.5rem;
	}

	.contact-select-row select {
		flex: 1;
	}

	.btn-new-contact {
		padding: 0.5rem 0.75rem;
		font-size: 1rem;
		font-weight: bold;
		flex-shrink: 0;
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

	.import-actions {
		justify-content: flex-start;
		flex-wrap: wrap;
	}

	.selected-file {
		margin-top: 1rem;
		color: var(--color-text-muted);
	}

	.import-summary {
		margin-top: 1.5rem;
		border-top: 1px solid var(--color-border);
		padding-top: 1.5rem;
	}

	.import-stats {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
		gap: 0.75rem;
		margin-top: 1rem;
	}

	.import-stat {
		border: 1px solid var(--color-border);
		border-radius: 0.75rem;
		padding: 0.75rem;
		background: var(--color-surface, rgba(255, 255, 255, 0.02));
	}

	.summary-label {
		display: block;
		font-size: 0.75rem;
		color: var(--color-text-muted);
		margin-bottom: 0.25rem;
	}

	.import-errors {
		margin-top: 1rem;
	}

	.import-errors ul {
		display: grid;
		gap: 0.75rem;
		padding-left: 1rem;
	}

	.import-errors li {
		color: var(--color-text-muted);
	}

	/* Mobile responsive styles */
	@media (max-width: 768px) {
		.filter-row {
			flex-direction: column;
		}

		.filter-row select {
			min-height: 44px;
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
			-webkit-overflow-scrolling: touch;
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
			min-height: 44px;
		}

		.actions {
			flex-wrap: wrap;
		}

		.btn-small {
			min-height: 36px;
			padding: 0.5rem 0.75rem;
		}

		.btn-new-contact {
			min-height: 44px;
			padding: 0.5rem 1rem;
		}

		.empty-actions {
			flex-direction: column;
		}

		.empty-actions .btn {
			width: 100%;
		}
	}
</style>
