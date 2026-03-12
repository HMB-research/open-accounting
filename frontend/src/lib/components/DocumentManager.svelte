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
	let isLoading = $state(false);
	let isUploading = $state(false);
	let error = $state('');

	$effect(() => {
		if (open && tenantId && entityId) {
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
				await api.uploadDocument(tenantId, entityType, entityId, file);
			}
			selectedFiles = [];
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

	function closeModal() {
		selectedFiles = [];
		error = '';
		onClose?.();
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
									<div class="document-details">
										<span>{formatFileSize(doc.file_size)}</span>
										<span>{doc.content_type}</span>
										<span>{formatDateTime(doc.created_at)}</span>
									</div>
								</div>
								<div class="document-actions">
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

	.document-details {
		display: flex;
		flex-wrap: wrap;
		gap: 0.75rem;
		margin-top: 0.35rem;
		font-size: 0.85rem;
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
