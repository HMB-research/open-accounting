<script lang="ts">
	import { api, type DocumentAttachment } from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';

	interface Props {
		open: boolean;
		tenantId: string;
		entityType: DocumentAttachment['entity_type'];
		entityId: string;
		title: string;
		onClose?: () => void;
	}

	let { open, tenantId, entityType, entityId, title, onClose }: Props = $props();

	let documents = $state<DocumentAttachment[]>([]);
	let selectedFiles = $state<File[]>([]);
	let selectedDocumentType = $state<DocumentAttachment['document_type']>('supporting_document');
	let notes = $state('');
	let retentionUntil = $state('');
	let isLoading = $state(false);
	let isUploading = $state(false);
	let error = $state('');

	$effect(() => {
		if (open && tenantId && entityId) {
			selectedDocumentType = defaultDocumentType(entityType);
			void loadDocuments();
		}
	});

	async function loadDocuments() {
		isLoading = true;
		error = '';

		try {
			documents = await api.listDocuments(tenantId, entityType, entityId);
		} catch (err) {
			error = err instanceof Error ? err.message : m.documents_loadError();
		} finally {
			isLoading = false;
		}
	}

	function handleFileChange(event: Event) {
		const input = event.currentTarget as HTMLInputElement | null;
		selectedFiles = input?.files ? Array.from(input.files) : [];
		error = '';
	}

	async function uploadSelected() {
		if (!selectedFiles.length) {
			error = m.documents_fileRequired();
			return;
		}

		isUploading = true;
		error = '';

		try {
			for (const file of selectedFiles) {
				await api.uploadDocument(tenantId, entityType, entityId, file, {
					document_type: selectedDocumentType,
					notes: notes.trim() || undefined,
					retention_until: retentionUntil || undefined
				});
			}
			resetUploadState();
			await loadDocuments();
		} catch (err) {
			error = err instanceof Error ? err.message : m.documents_uploadError();
		} finally {
			isUploading = false;
		}
	}

	async function downloadAttachment(doc: DocumentAttachment) {
		try {
			await api.downloadDocument(tenantId, doc.id, doc.file_name);
		} catch (err) {
			error = err instanceof Error ? err.message : m.documents_downloadError();
		}
	}

	async function deleteAttachment(doc: DocumentAttachment) {
		if (!confirm(m.documents_deleteConfirm({ file: doc.file_name }))) {
			return;
		}

		try {
			await api.deleteDocument(tenantId, doc.id);
			documents = documents.filter((item) => item.id !== doc.id);
		} catch (err) {
			error = err instanceof Error ? err.message : m.documents_deleteError();
		}
	}

	async function markReviewed(doc: DocumentAttachment) {
		try {
			const updated = await api.markDocumentReviewed(tenantId, doc.id);
			documents = documents.map((item) => (item.id === updated.id ? updated : item));
		} catch (err) {
			error = err instanceof Error ? err.message : m.documents_reviewError();
		}
	}

	function closeModal() {
		resetUploadState();
		onClose?.();
	}

	function resetUploadState() {
		selectedFiles = [];
		selectedDocumentType = defaultDocumentType(entityType);
		notes = '';
		retentionUntil = '';
		error = '';
	}

	function formatFileSize(size: number): string {
		if (size >= 1024 * 1024) {
			return `${(size / (1024 * 1024)).toFixed(1)} MB`;
		}
		if (size >= 1024) {
			return `${Math.round(size / 1024)} KB`;
		}
		return `${size} B`;
	}

	function formatDateTime(value: string): string {
		return new Date(value).toLocaleString();
	}

	function formatDate(value: string): string {
		return new Date(value).toLocaleDateString();
	}

	function defaultDocumentType(
		type: DocumentAttachment['entity_type']
	): DocumentAttachment['document_type'] {
		switch (type) {
			case 'bank_transaction':
				return 'reconciliation_evidence';
			case 'asset':
				return 'asset_record';
			case 'payment':
				return 'receipt';
			default:
				return 'supporting_document';
		}
	}

	function getDocumentTypeLabel(
		type: DocumentAttachment['document_type']
	): string {
		switch (type) {
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
			default:
				return m.documents_typeOther();
		}
	}
</script>

{#if open}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={closeModal} role="presentation">
		<div
			class="modal card document-modal"
			onclick={(event) => event.stopPropagation()}
			role="dialog"
			aria-modal="true"
			aria-labelledby="document-manager-title"
			tabindex="-1"
		>
			<div class="document-header">
				<div>
					<p class="document-kicker">{m.documents_kicker()}</p>
					<h2 id="document-manager-title">{title}</h2>
					<p class="document-subtitle">{m.documents_subtitle()}</p>
				</div>
				<button type="button" class="btn btn-secondary" onclick={closeModal}>
					{m.common_close()}
				</button>
			</div>

			{#if error}
				<div class="alert alert-error">{error}</div>
			{/if}

			<section class="document-upload card">
				<div class="document-upload-copy">
					<h3>{m.documents_uploadTitle()}</h3>
					<p>{m.documents_uploadDesc()}</p>
				</div>
				<div class="document-metadata-grid">
					<div>
						<label class="label" for="documentType">{m.documents_typeLabel()}</label>
						<select class="input" id="documentType" bind:value={selectedDocumentType}>
							<option value="supporting_document">{m.documents_typeSupporting()}</option>
							<option value="receipt">{m.documents_typeReceipt()}</option>
							<option value="reconciliation_evidence">{m.documents_typeReconciliation()}</option>
							<option value="contract">{m.documents_typeContract()}</option>
							<option value="asset_record">{m.documents_typeAsset()}</option>
							<option value="tax_support">{m.documents_typeTax()}</option>
							<option value="other">{m.documents_typeOther()}</option>
						</select>
					</div>
					<div>
						<label class="label" for="retentionUntil">{m.documents_retentionUntil()}</label>
						<input class="input" type="date" id="retentionUntil" bind:value={retentionUntil} />
					</div>
				</div>
				<div>
					<label class="label" for="documentNotes">{m.documents_notesLabel()}</label>
					<textarea class="input" id="documentNotes" rows="2" bind:value={notes} placeholder={m.documents_notesPlaceholder()}></textarea>
				</div>
				<div class="document-upload-controls">
					<input
						class="input"
						type="file"
						multiple
						onchange={handleFileChange}
						accept=".pdf,.png,.jpg,.jpeg,.csv,.txt"
					/>
					<button class="btn btn-primary" type="button" onclick={uploadSelected} disabled={isUploading}>
						{isUploading ? m.documents_uploading() : m.documents_uploadAction()}
					</button>
				</div>
				{#if selectedFiles.length > 0}
					<ul class="selected-files">
						{#each selectedFiles as file}
							<li>{file.name} · {formatFileSize(file.size)}</li>
						{/each}
					</ul>
				{/if}
			</section>

			<section class="document-list-section">
				<div class="document-section-header">
					<h3>{m.documents_existingTitle()}</h3>
					<span>{documents.length}</span>
				</div>

				{#if isLoading}
					<p>{m.common_loading()}</p>
				{:else if documents.length === 0}
					<div class="empty-documents">
						<p>{m.documents_emptyState()}</p>
					</div>
				{:else}
					<ul class="document-list">
						{#each documents as doc}
							<li class="document-item">
								<div class="document-meta">
									<strong>{doc.file_name}</strong>
									<div class="document-badges">
										<span class="document-badge">{getDocumentTypeLabel(doc.document_type)}</span>
										<span
											class="document-badge"
											class:document-badge-pending={doc.review_status === 'PENDING'}
											class:document-badge-reviewed={doc.review_status === 'REVIEWED'}
										>
											{doc.review_status === 'REVIEWED' ? m.documents_reviewed() : m.documents_pendingReview()}
										</span>
									</div>
									<div class="document-details">
										<span>{formatFileSize(doc.file_size)}</span>
										<span>{doc.content_type}</span>
										<span>{formatDateTime(doc.created_at)}</span>
									</div>
									{#if doc.notes}
										<p class="document-notes">{doc.notes}</p>
									{/if}
									<div class="document-details">
										{#if doc.retention_until}
											<span>{m.documents_retentionUntilLabel({ date: formatDate(doc.retention_until) })}</span>
										{/if}
										{#if doc.review_status === 'REVIEWED' && doc.reviewed_at}
											<span>{m.documents_reviewedAt({ date: formatDateTime(doc.reviewed_at) })}</span>
										{/if}
									</div>
								</div>
								<div class="document-actions">
									{#if doc.review_status !== 'REVIEWED'}
										<button type="button" class="btn btn-secondary" onclick={() => markReviewed(doc)}>
											{m.documents_markReviewed()}
										</button>
									{/if}
									<button type="button" class="btn btn-secondary" onclick={() => downloadAttachment(doc)}>
										{m.documents_downloadAction()}
									</button>
									<button type="button" class="btn btn-secondary btn-danger-soft" onclick={() => deleteAttachment(doc)}>
										{m.common_delete()}
									</button>
								</div>
							</li>
						{/each}
					</ul>
				{/if}
			</section>
		</div>
	</div>
{/if}

<style>
	.modal-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(15, 23, 42, 0.45);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 120;
		padding: 1rem;
	}

	.document-modal {
		width: min(52rem, 100%);
		max-height: 92vh;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.document-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 1rem;
	}

	.document-kicker {
		font-size: 0.76rem;
		letter-spacing: 0.16em;
		text-transform: uppercase;
		color: var(--color-text-muted);
	}

	.document-header h2 {
		margin: 0.3rem 0 0;
		font-family: var(--font-display);
		font-size: 1.8rem;
	}

	.document-subtitle {
		margin-top: 0.5rem;
		color: var(--color-text-muted);
	}

	.document-upload {
		display: grid;
		gap: 1rem;
		background: rgba(255, 255, 255, 0.6);
	}

	.document-metadata-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 1rem;
	}

	.document-upload-copy h3,
	.document-section-header h3 {
		margin: 0;
	}

	.document-upload-copy p {
		margin-top: 0.45rem;
		color: var(--color-text-muted);
	}

	.document-upload-controls {
		display: flex;
		gap: 0.75rem;
		flex-wrap: wrap;
		align-items: center;
	}

	.document-upload-controls .input {
		flex: 1;
		min-width: 14rem;
	}

	.selected-files {
		display: grid;
		gap: 0.4rem;
		padding-left: 1rem;
		color: var(--color-text-muted);
	}

	.document-list-section {
		display: grid;
		gap: 0.85rem;
	}

	.document-section-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	.document-section-header span {
		font-size: 0.95rem;
		font-weight: 600;
		color: var(--color-text-muted);
	}

	.empty-documents {
		padding: 1.25rem;
		border: 1px dashed var(--color-border);
		border-radius: var(--radius-sm);
		text-align: center;
		color: var(--color-text-muted);
	}

	.document-list {
		display: grid;
		gap: 0.75rem;
		list-style: none;
		padding: 0;
	}

	.document-item {
		display: flex;
		justify-content: space-between;
		gap: 1rem;
		align-items: center;
		padding: 1rem;
		border: 1px solid var(--color-border);
		border-radius: 0.95rem;
		background: rgba(255, 255, 255, 0.72);
	}

	.document-meta strong {
		display: block;
	}

	.document-badges {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		margin-top: 0.5rem;
	}

	.document-badge {
		display: inline-flex;
		align-items: center;
		padding: 0.2rem 0.55rem;
		border-radius: 999px;
		background: rgba(148, 163, 184, 0.14);
		color: var(--color-text);
		font-size: 0.78rem;
	}

	.document-badge-pending {
		background: rgba(245, 158, 11, 0.16);
		color: #9a6700;
	}

	.document-badge-reviewed {
		background: rgba(34, 197, 94, 0.16);
		color: #176b34;
	}

	.document-details {
		display: flex;
		flex-wrap: wrap;
		gap: 0.75rem;
		margin-top: 0.35rem;
		font-size: 0.85rem;
		color: var(--color-text-muted);
	}

	.document-notes {
		margin-top: 0.5rem;
		font-size: 0.92rem;
		color: var(--color-text-muted);
	}

	.document-actions {
		display: flex;
		gap: 0.5rem;
		flex-wrap: wrap;
	}

	.btn-danger-soft {
		color: #991b1b;
		border-color: rgba(153, 27, 27, 0.18);
		background: rgba(254, 226, 226, 0.92);
	}

	@media (max-width: 768px) {
		.document-metadata-grid {
			grid-template-columns: 1fr;
		}

		.document-header,
		.document-item,
		.document-upload-controls {
			flex-direction: column;
			align-items: stretch;
		}

		.document-actions {
			width: 100%;
		}

		.document-actions .btn,
		.document-upload-controls .btn {
			width: 100%;
		}
	}
</style>
