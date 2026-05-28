<script lang="ts">
	import { onMount } from 'svelte';
	import {
		api,
		type DocumentAttachment,
		type DocumentRetentionReview,
		type DocumentReviewQueue,
		type DocumentReviewStatusFilter
	} from '$lib/api';
	import ErrorAlert from '$lib/components/ErrorAlert.svelte';
	import * as m from '$lib/paraglide/messages.js';

	interface Props {
		tenantId: string;
		backHref?: string;
	}

	type QueueView = 'review' | 'retention';
	type EntityTypeFilter = DocumentAttachment['entity_type'] | '';
	type DocumentTypeFilter = DocumentAttachment['document_type'] | '';

	let { tenantId, backHref = '' }: Props = $props();

	const reviewStatusOptions: DocumentReviewStatusFilter[] = ['PENDING', 'REVIEWED', 'APPROVED', 'REJECTED', 'ALL'];
	const entityTypeOptions: EntityTypeFilter[] = ['', 'invoice', 'journal_entry', 'payment', 'bank_transaction', 'asset', 'year_end_close'];
	const documentTypeOptions: DocumentTypeFilter[] = [
		'',
		'supporting_document',
		'receipt',
		'reconciliation_evidence',
		'contract',
		'asset_record',
		'tax_support',
		'close_pack',
		'other'
	];
	const limitOptions = [25, 50, 100, 200];

	function todayInputValue(): string {
		const now = new Date();
		const month = String(now.getMonth() + 1).padStart(2, '0');
		const day = String(now.getDate()).padStart(2, '0');
		return `${now.getFullYear()}-${month}-${day}`;
	}

	let activeView = $state<QueueView>('review');
	let entityType = $state<EntityTypeFilter>('');
	let documentType = $state<DocumentTypeFilter>('');
	let reviewStatus = $state<DocumentReviewStatusFilter>('PENDING');
	let queueLimit = $state(50);
	let retentionAsOf = $state(todayInputValue());
	let retentionHorizonDays = $state(30);
	let includeMissingRetention = $state(false);
	let reviewQueue = $state<DocumentReviewQueue | null>(null);
	let retentionReview = $state<DocumentRetentionReview | null>(null);
	let reviewNotes = $state<Record<string, string>>({});
	let isLoading = $state(false);
	let mutatingDocumentId = $state('');
	let downloadingDocumentId = $state('');
	let error = $state('');
	let successMessage = $state('');

	let activeDocuments = $derived(activeView === 'review' ? reviewQueue?.documents ?? [] : retentionReview?.documents ?? []);

	onMount(() => {
		void refreshActiveQueue();
	});

	async function refreshActiveQueue(clearSuccess = true) {
		if (clearSuccess) {
			successMessage = '';
		}
		if (!tenantId) {
			reviewQueue = null;
			retentionReview = null;
			error = m.settings_noTenantSelected();
			return;
		}
		if (activeView === 'review') {
			await loadReviewQueue();
			return;
		}
		await loadRetentionReview();
	}

	function switchView(view: QueueView) {
		if (activeView === view) {
			return;
		}
		activeView = view;
		void refreshActiveQueue();
	}

	async function loadReviewQueue() {
		isLoading = true;
		error = '';
		try {
			reviewQueue = await api.getDocumentReviewQueue(tenantId, {
				entity_type: entityType,
				document_type: documentType,
				review_status: reviewStatus,
				limit: queueLimit
			});
			syncReviewNotes(reviewQueue.documents);
		} catch (err) {
			error = err instanceof Error ? err.message : m.documents_loadReviewQueueError();
		} finally {
			isLoading = false;
		}
	}

	async function loadRetentionReview() {
		isLoading = true;
		error = '';
		try {
			retentionReview = await api.getDocumentRetentionReview(tenantId, {
				as_of: retentionAsOf || undefined,
				horizon_days: retentionHorizonDays,
				include_missing: includeMissingRetention ? true : undefined
			});
			syncReviewNotes(retentionReview.documents);
		} catch (err) {
			error = err instanceof Error ? err.message : m.documents_loadRetentionError();
		} finally {
			isLoading = false;
		}
	}

	function syncReviewNotes(documents: DocumentAttachment[]) {
		reviewNotes = Object.fromEntries(documents.map((doc) => [doc.id, reviewNotes[doc.id] ?? doc.review_note ?? '']));
	}

	function updateReviewNote(doc: DocumentAttachment, event: Event) {
		reviewNotes = {
			...reviewNotes,
			[doc.id]: (event.currentTarget as HTMLTextAreaElement).value
		};
	}

	async function markReviewed(doc: DocumentAttachment) {
		mutatingDocumentId = doc.id;
		error = '';
		try {
			await api.markDocumentReviewed(tenantId, doc.id);
			await refreshActiveQueue(false);
			successMessage = m.documents_reviewQueueUpdated({ file: doc.file_name });
		} catch (err) {
			error = err instanceof Error ? err.message : m.documents_reviewError();
		} finally {
			mutatingDocumentId = '';
		}
	}

	async function applyReviewDecision(doc: DocumentAttachment, status: 'APPROVED' | 'REJECTED') {
		const note = (reviewNotes[doc.id] || '').trim();
		if (status === 'REJECTED' && !note) {
			error = m.documents_reviewNoteRequired();
			return;
		}

		mutatingDocumentId = doc.id;
		error = '';
		try {
			await api.reviewDocument(tenantId, doc.id, {
				review_status: status,
				review_note: note || undefined
			});
			await refreshActiveQueue(false);
			successMessage = m.documents_reviewQueueUpdated({ file: doc.file_name });
		} catch (err) {
			error = err instanceof Error ? err.message : m.documents_reviewError();
		} finally {
			mutatingDocumentId = '';
		}
	}

	async function downloadDocument(doc: DocumentAttachment) {
		downloadingDocumentId = doc.id;
		error = '';
		try {
			await api.downloadDocument(tenantId, doc.id, doc.file_name);
		} catch (err) {
			error = err instanceof Error ? err.message : m.documents_downloadError();
		} finally {
			downloadingDocumentId = '';
		}
	}

	function formatDate(value?: string): string {
		if (!value) {
			return '-';
		}
		const parsed = new Date(value);
		if (Number.isNaN(parsed.getTime())) {
			return value;
		}
		return new Intl.DateTimeFormat(undefined, {
			year: 'numeric',
			month: 'short',
			day: 'numeric'
		}).format(parsed);
	}

	function formatDateTime(value: string): string {
		const parsed = new Date(value);
		if (Number.isNaN(parsed.getTime())) {
			return value;
		}
		return new Intl.DateTimeFormat(undefined, {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		}).format(parsed);
	}

	function formatFileSize(size: number): string {
		if (!Number.isFinite(size) || size <= 0) {
			return '0 B';
		}
		if (size < 1024) {
			return `${size} B`;
		}
		if (size < 1024 * 1024) {
			return `${new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 }).format(size / 1024)} KB`;
		}
		return `${new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 }).format(size / (1024 * 1024))} MB`;
	}

	function formatEntityType(value: DocumentAttachment['entity_type']): string {
		switch (value) {
			case 'invoice':
				return m.documents_entityInvoice();
			case 'journal_entry':
				return m.documents_entityJournalEntry();
			case 'payment':
				return m.documents_entityPayment();
			case 'bank_transaction':
				return m.documents_entityBankTransaction();
			case 'asset':
				return m.documents_entityAsset();
			case 'year_end_close':
				return m.documents_entityYearEndClose();
		}
	}

	function formatDocumentType(value: DocumentAttachment['document_type']): string {
		switch (value) {
			case 'supporting_document':
				return m.documents_typeSupporting();
			case 'receipt':
				return m.documents_typeReceipt();
			case 'reconciliation_evidence':
				return m.documents_typeReconciliation();
			case 'contract':
				return m.documents_typeContract();
			case 'asset_record':
				return m.documents_typeAsset();
			case 'tax_support':
				return m.documents_typeTax();
			case 'close_pack':
				return m.documents_typeClosePack();
			case 'other':
				return m.documents_typeOther();
		}
	}

	function formatReviewStatus(value: DocumentAttachment['review_status']): string {
		switch (value) {
			case 'REVIEWED':
				return m.documents_reviewed();
			case 'APPROVED':
				return m.documents_approved();
			case 'REJECTED':
				return m.documents_rejected();
			case 'PENDING':
				return m.documents_pendingReview();
		}
	}

	function getStatusClass(value: DocumentAttachment['review_status']): string {
		return `status-badge ${value.toLowerCase()}`;
	}
</script>

<div class="document-review-panel">
	<div class="panel-header">
		<div>
			{#if backHref}
				<a class="back-link" href={backHref}>{m.settings_backToSettings()}</a>
			{/if}
			<p class="panel-kicker">{m.documents_reviewQueueKicker()}</p>
			<h1>{m.documents_reviewQueueTitle()}</h1>
			<p class="panel-subtitle">{m.documents_reviewQueueSubtitle()}</p>
		</div>
		<button class="btn btn-secondary" type="button" onclick={() => refreshActiveQueue()} disabled={isLoading || !tenantId}>
			{isLoading ? m.common_loading() : m.common_refresh()}
		</button>
	</div>

	<div class="queue-tabs" role="tablist" aria-label={m.documents_reviewQueueTabsLabel()}>
		<button
			type="button"
			role="tab"
			aria-selected={activeView === 'review'}
			class:active={activeView === 'review'}
			onclick={() => switchView('review')}
		>
			{m.documents_reviewQueueTab()}
		</button>
		<button
			type="button"
			role="tab"
			aria-selected={activeView === 'retention'}
			class:active={activeView === 'retention'}
			onclick={() => switchView('retention')}
		>
			{m.documents_retentionQueueTab()}
		</button>
	</div>

	{#if error}
		<ErrorAlert message={error} type="error" onDismiss={() => (error = '')} />
	{/if}

	{#if successMessage}
		<div class="alert alert-success">{successMessage}</div>
	{/if}

	{#if !tenantId}
		<div class="card empty-state">
			<p>{m.settings_selectTenantDashboard()} <a href="/dashboard">{m.dashboard_title()}</a>.</p>
		</div>
	{:else}
		<section class="filter-band" aria-label={m.documents_reviewQueueFilters()}>
			{#if activeView === 'review'}
				<div class="filter-grid">
					<div class="form-group">
						<label class="label" for="document-review-status">{m.documents_reviewStatusLabel()}</label>
						<select id="document-review-status" class="input" bind:value={reviewStatus}>
							{#each reviewStatusOptions as option (option)}
								<option value={option}>{option === 'ALL' ? m.documents_statusAll() : formatReviewStatus(option)}</option>
							{/each}
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="document-review-entity">{m.documents_entityTypeLabel()}</label>
						<select id="document-review-entity" class="input" bind:value={entityType}>
							{#each entityTypeOptions as option (option)}
								<option value={option}>{option ? formatEntityType(option) : m.documents_entityTypeAll()}</option>
							{/each}
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="document-review-type">{m.documents_typeLabel()}</label>
						<select id="document-review-type" class="input" bind:value={documentType}>
							{#each documentTypeOptions as option (option)}
								<option value={option}>{option ? formatDocumentType(option) : m.documents_documentTypeAll()}</option>
							{/each}
						</select>
					</div>
					<div class="form-group compact">
						<label class="label" for="document-review-limit">{m.documents_limitLabel()}</label>
						<select id="document-review-limit" class="input" bind:value={queueLimit}>
							{#each limitOptions as option (option)}
								<option value={option}>{option}</option>
							{/each}
						</select>
					</div>
				</div>
			{:else}
				<div class="filter-grid retention">
					<div class="form-group">
						<label class="label" for="retention-as-of">{m.documents_asOfDate()}</label>
						<input id="retention-as-of" class="input" type="date" bind:value={retentionAsOf} />
					</div>
					<div class="form-group compact">
						<label class="label" for="retention-horizon">{m.documents_horizonDays()}</label>
						<input id="retention-horizon" class="input" type="number" min="0" step="1" bind:value={retentionHorizonDays} />
					</div>
					<label class="checkbox-row" for="include-missing-retention">
						<input id="include-missing-retention" type="checkbox" bind:checked={includeMissingRetention} />
						<span>{m.documents_includeMissingRetention()}</span>
					</label>
				</div>
			{/if}
			<button class="btn btn-primary" type="button" onclick={() => refreshActiveQueue()} disabled={isLoading}>
				{m.documents_applyFilters()}
			</button>
		</section>

		{#if isLoading}
			<div class="loading">{m.common_loading()}</div>
		{:else}
			{#if activeView === 'review' && reviewQueue}
				<div class="summary-grid" aria-label={m.documents_reviewQueueSummary()}>
					<div class="summary-item">
						<span>{m.documents_reviewQueueTotal()}</span>
						<strong>{reviewQueue.total_count}</strong>
					</div>
					<div class="summary-item">
						<span>{m.documents_reviewQueuePending()}</span>
						<strong>{reviewQueue.pending_review_count}</strong>
					</div>
					<div class="summary-item">
						<span>{m.documents_reviewQueueApproved()}</span>
						<strong>{reviewQueue.approved_count}</strong>
					</div>
					<div class="summary-item">
						<span>{m.documents_reviewQueueRejected()}</span>
						<strong>{reviewQueue.rejected_count}</strong>
					</div>
				</div>
			{:else if activeView === 'retention' && retentionReview}
				<div class="summary-grid" aria-label={m.documents_reviewQueueSummary()}>
					<div class="summary-item">
						<span>{m.documents_reviewQueueTotal()}</span>
						<strong>{retentionReview.total_count}</strong>
					</div>
					<div class="summary-item">
						<span>{m.documents_retentionExpired()}</span>
						<strong>{retentionReview.expired_count}</strong>
					</div>
					<div class="summary-item">
						<span>{m.documents_retentionDueSoon()}</span>
						<strong>{retentionReview.due_soon_count}</strong>
					</div>
					<div class="summary-item">
						<span>{m.documents_retentionMissing()}</span>
						<strong>{retentionReview.missing_retention_count}</strong>
					</div>
				</div>
			{/if}

			{#if activeDocuments.length === 0}
				<div class="card empty-state">
					<p>{activeView === 'review' ? m.documents_reviewQueueEmpty() : m.documents_retentionQueueEmpty()}</p>
				</div>
			{:else}
				<div class="card table-container document-table-card">
					<table class="table table-mobile-cards document-review-table">
						<thead>
							<tr>
								<th>{m.documents_reviewQueueFile()}</th>
								<th>{m.documents_reviewQueueEntity()}</th>
								<th>{m.common_status()}</th>
								<th>{m.documents_retentionUntil()}</th>
								<th>{m.documents_reviewQueueUploaded()}</th>
								<th>{m.common_actions()}</th>
							</tr>
						</thead>
						<tbody>
							{#each activeDocuments as doc (doc.id)}
								<tr>
									<td data-label={m.documents_reviewQueueFile()}>
										<div class="document-file">
											<strong>{doc.file_name}</strong>
											<span>{formatDocumentType(doc.document_type)} · {formatFileSize(doc.file_size)}</span>
											{#if doc.notes}
												<p>{doc.notes}</p>
											{/if}
										</div>
									</td>
									<td data-label={m.documents_reviewQueueEntity()}>
										<div class="document-entity">
											<span>{formatEntityType(doc.entity_type)}</span>
											<code>{doc.entity_id}</code>
										</div>
									</td>
									<td data-label={m.common_status()}>
										<div class="status-stack">
											<span class={getStatusClass(doc.review_status)}>{formatReviewStatus(doc.review_status)}</span>
											{#if doc.reviewed_at}
												<span>{m.documents_reviewedAt({ date: formatDateTime(doc.reviewed_at) })}</span>
											{/if}
											{#if doc.review_note}
												<p>{doc.review_note}</p>
											{/if}
										</div>
									</td>
									<td data-label={m.documents_retentionUntil()}>{formatDate(doc.retention_until)}</td>
									<td data-label={m.documents_reviewQueueUploaded()}>{formatDateTime(doc.created_at)}</td>
									<td data-label={m.common_actions()} class="actions-cell">
										<div class="review-actions">
											<label class="label" for={`queue-review-note-${doc.id}`}>{m.documents_reviewNoteFor({ file: doc.file_name })}</label>
											<textarea
												class="input"
												id={`queue-review-note-${doc.id}`}
												rows="2"
												value={reviewNotes[doc.id] || ''}
												placeholder={m.documents_reviewNotePlaceholder()}
												oninput={(event) => updateReviewNote(doc, event)}
											></textarea>
											<div class="action-row">
												<button type="button" class="btn btn-secondary approve" onclick={() => applyReviewDecision(doc, 'APPROVED')} disabled={mutatingDocumentId === doc.id}>
													{m.documents_approve()}
												</button>
												<button type="button" class="btn btn-secondary" onclick={() => markReviewed(doc)} disabled={mutatingDocumentId === doc.id}>
													{m.documents_markReviewed()}
												</button>
												<button type="button" class="btn btn-secondary reject" onclick={() => applyReviewDecision(doc, 'REJECTED')} disabled={mutatingDocumentId === doc.id}>
													{m.documents_reject()}
												</button>
												<button type="button" class="btn btn-secondary" onclick={() => downloadDocument(doc)} disabled={downloadingDocumentId === doc.id}>
													{downloadingDocumentId === doc.id ? m.common_loading() : m.documents_downloadAction()}
												</button>
											</div>
										</div>
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

<style>
	.document-review-panel {
		display: grid;
		gap: 1rem;
	}

	.panel-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 1rem;
	}

	.back-link {
		display: inline-flex;
		margin-bottom: 0.5rem;
		font-weight: 600;
	}

	.panel-kicker {
		margin: 0 0 0.35rem;
		color: var(--color-text-muted);
		font-size: 0.78rem;
		font-weight: 700;
		letter-spacing: 0;
		text-transform: uppercase;
	}

	h1 {
		margin: 0;
		font-size: 1.75rem;
		line-height: 1.15;
	}

	.panel-subtitle {
		max-width: 56rem;
		margin: 0.45rem 0 0;
		color: var(--color-text-muted);
	}

	.queue-tabs {
		display: inline-flex;
		width: fit-content;
		gap: 0.25rem;
		padding: 0.25rem;
		border: 1px solid var(--color-border);
		border-radius: 999px;
		background: rgba(255, 255, 255, 0.62);
	}

	.queue-tabs button {
		min-width: 8rem;
		padding: 0.55rem 0.9rem;
		border: 0;
		border-radius: 999px;
		background: transparent;
		color: var(--color-text-muted);
		font-weight: 700;
	}

	.queue-tabs button.active {
		background: var(--color-primary);
		color: white;
		box-shadow: 0 10px 22px rgba(37, 99, 235, 0.18);
	}

	.filter-band {
		display: flex;
		align-items: flex-end;
		justify-content: space-between;
		gap: 1rem;
		padding: 1rem;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-sm);
		background: rgba(255, 255, 255, 0.55);
	}

	.filter-grid {
		display: grid;
		grid-template-columns: minmax(10rem, 1fr) minmax(10rem, 1fr) minmax(10rem, 1fr) 7rem;
		gap: 0.75rem;
		flex: 1;
	}

	.filter-grid.retention {
		grid-template-columns: minmax(10rem, 1fr) 8rem minmax(12rem, 1fr);
		align-items: end;
	}

	.form-group {
		margin-bottom: 0;
	}

	.form-group.compact {
		max-width: 9rem;
	}

	.checkbox-row {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		min-height: 44px;
		font-weight: 600;
		color: var(--color-text);
	}

	.summary-grid {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		gap: 0.75rem;
	}

	.summary-item {
		padding: 1rem;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-sm);
		background: rgba(255, 255, 255, 0.66);
	}

	.summary-item span {
		display: block;
		margin-bottom: 0.25rem;
		color: var(--color-text-muted);
		font-size: 0.78rem;
		font-weight: 700;
		text-transform: uppercase;
	}

	.summary-item strong {
		font-size: 1.6rem;
		line-height: 1;
	}

	.loading,
	.empty-state {
		text-align: center;
		padding: 2rem;
		color: var(--color-text-muted);
	}

	.document-table-card {
		padding: 0;
	}

	.document-review-table {
		min-width: 1080px;
	}

	.document-review-table th,
	.document-review-table td {
		vertical-align: top;
	}

	.document-file {
		display: grid;
		gap: 0.25rem;
		min-width: 14rem;
	}

	.document-file strong {
		word-break: break-word;
	}

	.document-file span,
	.document-file p,
	.status-stack span,
	.status-stack p {
		margin: 0;
		color: var(--color-text-muted);
		font-size: 0.82rem;
	}

	.document-entity {
		display: grid;
		gap: 0.25rem;
		min-width: 10rem;
	}

	code {
		color: var(--color-text-muted);
		font-family: var(--font-mono);
		font-size: 0.78rem;
		white-space: normal;
		word-break: break-all;
	}

	.status-stack,
	.review-actions {
		display: grid;
		gap: 0.5rem;
	}

	.status-badge {
		display: inline-flex;
		width: fit-content;
		align-items: center;
		padding: 0.25rem 0.6rem;
		border-radius: 999px;
		font-size: 0.78rem;
		font-weight: 700;
	}

	.status-badge.pending {
		background: #fef3c7;
		color: #92400e;
	}

	.status-badge.reviewed {
		background: #dbeafe;
		color: #1d4ed8;
	}

	.status-badge.approved {
		background: #d1fae5;
		color: #065f46;
	}

	.status-badge.rejected {
		background: #fee2e2;
		color: #991b1b;
	}

	.review-actions {
		min-width: 20rem;
	}

	.action-row {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}

	.action-row .btn {
		justify-content: center;
		min-height: 2.35rem;
		padding: 0.45rem 0.75rem;
		white-space: nowrap;
	}

	.action-row .approve {
		border-color: rgba(34, 197, 94, 0.35);
		color: #047857;
	}

	.action-row .reject {
		border-color: rgba(239, 68, 68, 0.35);
		color: #b91c1c;
	}

	@media (max-width: 900px) {
		.panel-header,
		.filter-band {
			flex-direction: column;
			align-items: stretch;
		}

		.queue-tabs {
			width: 100%;
		}

		.queue-tabs button {
			flex: 1;
			min-width: 0;
		}

		.filter-grid,
		.filter-grid.retention,
		.summary-grid {
			grid-template-columns: 1fr 1fr;
		}

		.form-group.compact {
			max-width: none;
		}
	}

	@media (max-width: 768px) {
		.document-review-table {
			min-width: 0;
		}

		.document-review-table tbody tr {
			display: grid;
			gap: 0.5rem;
		}

		.document-review-table td {
			align-items: flex-start;
		}

		.document-review-table td::before {
			width: 8.5rem;
		}

		.document-file,
		.document-entity,
		.review-actions {
			min-width: 0;
			width: 100%;
		}
	}

	@media (max-width: 520px) {
		.filter-grid,
		.filter-grid.retention,
		.summary-grid {
			grid-template-columns: 1fr;
		}

		.action-row .btn {
			width: 100%;
		}
	}
</style>
