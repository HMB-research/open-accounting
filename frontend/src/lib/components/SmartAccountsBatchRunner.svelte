<script lang="ts">
	import type {
		SmartAccountsBrowserBatchWorkflowPhase,
		SmartAccountsBrowserBatchWorkflowPreparationRequest,
		SmartAccountsBrowserBatchWorkflowSource,
		SmartAccountsBrowserBatchWorkflowStatus,
		SmartAccountsBrowserBatchWorkflowTransferConfirmationRequest
	} from '$lib/api';
	import SmartAccountsReconciliationPanel from '$lib/components/SmartAccountsReconciliationPanel.svelte';

	type AsyncStatusAction = () => Promise<SmartAccountsBrowserBatchWorkflowStatus>;
	type PreparationAction = (request: SmartAccountsBrowserBatchWorkflowPreparationRequest) => Promise<SmartAccountsBrowserBatchWorkflowStatus>;
	type TransferAction = (request: SmartAccountsBrowserBatchWorkflowTransferConfirmationRequest) => Promise<SmartAccountsBrowserBatchWorkflowStatus>;
	type AdvanceAction = (status: SmartAccountsBrowserBatchWorkflowStatus) => Promise<SmartAccountsBrowserBatchWorkflowStatus>;
	type SchemaConfirmationAction = (source: SmartAccountsBrowserBatchWorkflowSource) => Promise<SmartAccountsBrowserBatchWorkflowStatus>;
	type DiscoveryReissueAction = (source: SmartAccountsBrowserBatchWorkflowSource) => Promise<SmartAccountsBrowserBatchWorkflowStatus>;

	let {
		batchId,
		workflow = null,
		companyNames = {},
		onRefresh,
		onPrepare,
		onResume,
		onAdvanceSafe,
		onAdvanceConfirmedTransfer,
		onConfirmSchema,
		onReissueDiscovery,
		onOpenTransfer,
		onConfirmTransfer,
		onWorkflowChange
	}: {
		batchId: string;
		workflow?: SmartAccountsBrowserBatchWorkflowStatus | null;
		companyNames?: Record<string, string>;
		onRefresh: AsyncStatusAction;
		onPrepare: PreparationAction;
		onResume: AsyncStatusAction;
		onAdvanceSafe?: AdvanceAction;
		onAdvanceConfirmedTransfer?: AdvanceAction;
		onConfirmSchema?: SchemaConfirmationAction;
		onReissueDiscovery?: DiscoveryReissueAction;
		onOpenTransfer: AsyncStatusAction;
		onConfirmTransfer: TransferAction;
		onWorkflowChange?: (workflow: SmartAccountsBrowserBatchWorkflowStatus) => void;
	} = $props();

	let historyFrom = $state('');
	let preparationConfirmed = $state(false);
	let metadataDiscoveryConfirmed = $state(false);
	let headerProbeConfirmed = $state(false);
	let transferConfirmed = $state(false);
	let resumeTransferConfirmed = $state(false);
	let schemaConfirmed = $state<Record<string, boolean>>({});
	let discoveryReissueConfirmed = $state<Record<string, boolean>>({});
	let busy = $state(false);
	let message = $state('');
	let error = $state('');
	let copiedAccountantSource = $state('');

	const terminalPhases = new Set<SmartAccountsBrowserBatchWorkflowPhase>(['PREVIEW_READY', 'REVIEW_REQUIRED', 'BLOCKED']);
	const reviewPhases = new Set<SmartAccountsBrowserBatchWorkflowPhase>(['SCHEMA_REVIEW_REQUIRED', 'REVIEW_REQUIRED', 'BLOCKED']);
	const resumablePhases = new Set<SmartAccountsBrowserBatchWorkflowPhase>(['FAILED_RETRYABLE', 'DISCOVERY_RUNNING', 'CAPTURE_RUNNING']);

	let sources = $derived(workflow?.sources ?? []);
	let allSchemaApproved = $derived(sources.length > 0 && sources.every((source) => source.phase === 'SCHEMA_APPROVED' || source.phase === 'TRANSFER_CONFIRMATION_REQUIRED' || source.phase === 'CAPTURE_RUNNING' || source.phase === 'STAGED' || source.phase === 'PREVIEW_READY' || source.phase === 'REVIEW_REQUIRED'));
	let transferOpen = $derived(sources.length > 0 && sources.every((source) => source.phase === 'TRANSFER_CONFIRMATION_REQUIRED' || source.phase === 'CAPTURE_RUNNING' || source.phase === 'STAGED' || source.phase === 'PREVIEW_READY' || source.phase === 'REVIEW_REQUIRED'));
	let transferHasBeenConfirmed = $derived(Boolean(workflow?.workflow.transfer_confirmed_at));
	let sourceCount = $derived(sources.length);
	let completeCount = $derived(sources.filter((source) => terminalPhases.has(source.phase)).length);
	let activeSource = $derived(sources.find((source) => source.phase === 'DISCOVERY_RUNNING' || source.phase === 'CAPTURE_RUNNING'));
	let reviewSourceCount = $derived(sources.filter((source) => reviewPhases.has(source.phase)).length);
	let resumableSourceCount = $derived(sources.filter((source) => resumablePhases.has(source.phase)).length);
	let safeContinuationAvailable = $derived(
		sources.some((source) => source.phase === 'DISCOVERY_REQUIRED' || source.phase === 'DISCOVERY_COMPLETE' || source.phase === 'SCHEMA_APPROVED') ||
		resumableSourceCount > 0
	);

	function sourceName(sourceCompanyId: string, ordinal: number): string {
		return companyNames[sourceCompanyId] ?? `Company ${ordinal + 1}`;
	}

	function isSafeOpaqueId(value: string): boolean {
		return /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(value);
	}

	function accountantReviewHref(source: SmartAccountsBrowserBatchWorkflowSource): string {
		// The handoff contains only opaque, lexically safe binding identifiers.
		// The accountant-only endpoint revalidates tenant/batch/source ownership;
		// no source name, digest, token, capability, or data reaches this URL.
		if (!isSafeOpaqueId(batchId) || !isSafeOpaqueId(source.tenant_id) || !isSafeOpaqueId(source.source_company_id)) return '';
		const query = new URLSearchParams({
			tenant: source.tenant_id,
			reconciliation_batch: batchId,
			reconciliation_source: source.source_company_id
		});
		return `/migration?${query.toString()}`;
	}

	function ownerContinuationHref(source: SmartAccountsBrowserBatchWorkflowSource): string {
		// This URL has only immutable opaque binding IDs. The destination
		// re-fetches owner-safe state and rejects a stale/cross-bound source.
		if (!isSafeOpaqueId(batchId) || !isSafeOpaqueId(source.tenant_id) || !isSafeOpaqueId(source.source_company_id)) return '';
		const query = new URLSearchParams({
			tenant: source.tenant_id,
			workflow_batch: batchId,
			workflow_source: source.source_company_id
		});
		return `/migration?${query.toString()}`;
	}

	async function copyAccountantReviewLink(source: SmartAccountsBrowserBatchWorkflowSource) {
		const href = accountantReviewHref(source);
		if (!href) {
			error = 'The accountant handoff identifiers are invalid. Refresh the safe batch status before sharing a review link.';
			return;
		}
		if (!navigator.clipboard?.writeText) {
			error = 'Clipboard access is unavailable. Open the accountant review in a separate tab and share its address only through an approved channel.';
			return;
		}
		try {
			await navigator.clipboard.writeText(new URL(href, window.location.origin).toString());
			copiedAccountantSource = source.source_company_id;
			message = 'The opaque accountant review link was copied. It contains no names, source rows, browser capabilities, tokens, or evidence digests.';
		} catch {
			error = 'The accountant review link could not be copied. Open it in a separate tab and share its address only through an approved channel.';
		}
	}

	function phaseLabel(phase: SmartAccountsBrowserBatchWorkflowPhase): string {
		return phase.replaceAll('_', ' ').toLowerCase();
	}

	function transitionMessage(status: SmartAccountsBrowserBatchWorkflowStatus): string {
		const active = status.sources.find((source) => source.phase === 'DISCOVERY_RUNNING' || source.phase === 'CAPTURE_RUNNING');
		if (active?.phase === 'CAPTURE_RUNNING') return 'Source transfer is in progress or its package is compiling. The page retains only safe server progress; no data is shown here.';
		if (active?.phase === 'DISCOVERY_RUNNING') return 'The next company is undergoing metadata-only discovery. Refreshing reads safe server status only.';
		if (status.sources.some((source) => source.phase === 'SCHEMA_REVIEW_REQUIRED')) return 'A source requires a reviewed server-side CSV schema before the batch can continue.';
		if (status.sources.every((source) => terminalPhases.has(source.phase))) return 'Each company reached a safe terminal review state. Financial apply remains a separate per-tenant confirmation.';
		return 'Safe workflow status updated.';
	}

	function setWorkflow(next: SmartAccountsBrowserBatchWorkflowStatus) {
		workflow = next;
		onWorkflowChange?.(next);
		message = transitionMessage(next);
	}

	function safeAdvanceNeeded(status: SmartAccountsBrowserBatchWorkflowStatus): boolean {
		if (status.workflow.transfer_confirmed_at) return false;
		return status.sources.some((source) => source.phase === 'DISCOVERY_REQUIRED' || source.phase === 'DISCOVERY_COMPLETE' || source.phase === 'SCHEMA_APPROVED');
	}

	async function run(action: AsyncStatusAction, advance: 'never' | 'safe' | 'confirmed-transfer' = 'never') {
		if (busy) return;
		busy = true;
		error = '';
		try {
			const result = await action();
			setWorkflow(result);
			if (advance === 'safe' && onAdvanceSafe && safeAdvanceNeeded(result)) {
				setWorkflow(await onAdvanceSafe(result));
			}
			if (advance === 'confirmed-transfer' && onAdvanceConfirmedTransfer) {
				setWorkflow(await onAdvanceConfirmedTransfer(result));
			}
		} catch (cause) {
			error = cause instanceof Error ? cause.message : 'The safe batch action could not be completed.';
		} finally {
			busy = false;
		}
	}

	function prepare() {
		if (!historyFrom || !preparationConfirmed || !metadataDiscoveryConfirmed) return;
		return run(
			() => onPrepare({
				history_from: historyFrom,
				owner_confirmed: true,
				metadata_discovery_consent_confirmed: true,
				header_probe_consent_confirmed: headerProbeConfirmed
			}),
			'safe'
		);
	}

	function continueSafeWorkflow() {
		// A running/retryable lease must use the server's resume endpoint. Other
		// safe phases start from a fresh owner-safe GET and may advance only the
		// already-consented metadata/schema step. Transfer is deliberately absent:
		// its explicit action-time confirmation is rendered below.
		return run(resumableSourceCount > 0 ? onResume : onRefresh, 'safe');
	}

	function openTransfer() {
		return run(onOpenTransfer);
	}

	function confirmTransfer() {
		if (!workflow?.schema_readiness_sha256 || !transferConfirmed) return;
		return run(
			() => onConfirmTransfer({ owner_confirmed: true, expected_schema_sha256: workflow!.schema_readiness_sha256! }),
			'confirmed-transfer'
		);
	}

	function confirmSchema(source: SmartAccountsBrowserBatchWorkflowSource) {
		if (!schemaConfirmed[source.source_company_id] || !onConfirmSchema) return;
		return run(() => onConfirmSchema(source), 'safe');
	}

	function reissueDiscovery(source: SmartAccountsBrowserBatchWorkflowSource) {
		if (!discoveryReissueConfirmed[source.source_company_id] || !onReissueDiscovery) return;
		return run(() => onReissueDiscovery(source));
	}

	function resumeTransfer() {
		if (!resumeTransferConfirmed) return;
		return run(onResume, 'confirmed-transfer');
	}
</script>

<section class="batch-runner package-review" aria-labelledby="smartaccounts-batch-runner-heading">
	<div class="heading">
		<div>
			<h3 id="smartaccounts-batch-runner-heading">Selected/all company migration runner</h3>
			<p class="help">Batch {batchId}. Progress is serialized by Open Accounting: only one company can hold a discovery or capture lease at a time. Reference preview, reconciliation, and apply remain separate per-tenant actions; this batch never applies data as one aggregate.</p>
		</div>
		<span class="status-pill">{workflow?.status ?? 'Not prepared'}</span>
	</div>

	{#if error}
		<div class="alert alert-error" role="alert">{error}</div>
	{/if}

	{#if !workflow}
		<p>Prepare the paired companies for metadata-only discovery and server-side schema review. This does not transfer source records or post accounting data.</p>
		<div class="form-group compact-grid">
			<label class="label" for="smartaccounts-batch-history-from">History starts</label>
			<input id="smartaccounts-batch-history-from" class="input" type="date" bind:value={historyFrom} />
		</div>
		<label class="company-choice"><input type="checkbox" bind:checked={metadataDiscoveryConfirmed} /> I approve metadata-only discovery for every paired company in this immutable batch.</label>
		<label class="company-choice"><input type="checkbox" disabled={!metadataDiscoveryConfirmed} bind:checked={headerProbeConfirmed} /> I separately approve bounded CSV header-name probing where supported. No row values or CSV bodies are retained.</label>
		<label class="confirm"><input type="checkbox" bind:checked={preparationConfirmed} /> I confirm this selected/all batch is ready for non-destructive preparation. Source transfer and GL apply require separate later confirmations.</label>
		<button class="btn btn-primary" type="button" disabled={busy || !historyFrom || !preparationConfirmed || !metadataDiscoveryConfirmed} onclick={() => void prepare()}>{busy ? 'Preparing…' : 'Prepare safe batch workflow'}</button>
	{:else if sourceCount === 0}
		<p>No paired Open Accounting tenants are available for this batch, so source discovery and transfer stay unavailable.</p>
		<a class="link-button" href="/migration">Return to selected/all company onboarding</a>
	{:else}
		<div class="workflow-summary" aria-live="polite">
			<p><strong>{completeCount}/{sourceCount}</strong> companies have reached a terminal review state{activeSource ? `; ${sourceName(activeSource.source_company_id, activeSource.ordinal)} is ${phaseLabel(activeSource.phase)}` : ''}.</p>
			{#if transferHasBeenConfirmed && workflow.workflow.transfer_scope?.resource_ids}
				<p class="help">Frozen scope: {workflow.workflow.transfer_scope.from_inclusive}–{workflow.workflow.transfer_scope.to_inclusive}, <code>{workflow.workflow.transfer_scope.resource_ids.join(', ')}</code>. This is a partial browser transfer, never a full-sync claim.</p>
			{/if}
		</div>

		<ul class="source-list" aria-label="Per-company migration phases">
			{#each sources as source (source.source_company_id)}
				{@const reviewHref = accountantReviewHref(source)}
				{@const continuationHref = ownerContinuationHref(source)}
				<li>
					<div>
						<strong>{sourceName(source.source_company_id, source.ordinal)}</strong>
						<span class="phase">{phaseLabel(source.phase)}</span>
						{#if source.reason_code}<p class="help">Review blocker: {source.reason_code}</p>{/if}
						{#if source.phase === 'CAPTURE_RUNNING'}<p class="help">Capture may be complete at the source while Open Accounting safely compiles its package. It is not staged or ready for preview until the server says so.</p>{/if}
						{#if source.phase === 'DISCOVERY_RUNNING' && onReissueDiscovery}
							<p class="help">If the relay/page event was lost after a reload, rotate this exact discovery action now. The prior lease cannot complete afterward.</p>
							<label class="confirm"><input type="checkbox" bind:checked={discoveryReissueConfirmed[source.source_company_id]} /> I reconfirm metadata-only discovery for this exact paired company. Any optional header probe remains the frozen batch choice.</label>
							<button class="btn btn-secondary" type="button" disabled={busy || !discoveryReissueConfirmed[source.source_company_id]} onclick={() => void reissueDiscovery(source)}>{busy ? 'Reissuing…' : 'Reissue lost discovery action'}</button>
						{/if}
						{#if source.phase === 'SCHEMA_REVIEW_REQUIRED' && onConfirmSchema}
							<label class="confirm"><input type="checkbox" bind:checked={schemaConfirmed[source.source_company_id]} /> I reviewed the server-bound schema approval for this tenant. No source data or accounting data is displayed here.</label>
							<button class="btn btn-secondary" type="button" disabled={busy || !schemaConfirmed[source.source_company_id]} onclick={() => void confirmSchema(source)}>Confirm reviewed schema</button>
						{/if}
					</div>
					<div class="source-links">
						{#if continuationHref}
							<a class="link-button" href={continuationHref}>Open tenant review &amp; apply</a>
						{:else}
							<span class="help">Tenant continuation binding is invalid. Refresh the safe workflow before continuing.</span>
						{/if}
						{#if reviewHref}
							<a class="link-button" href={reviewHref} target="_blank" rel="noreferrer">Open accountant review</a>
							<button class="btn btn-secondary" type="button" disabled={busy} onclick={() => void copyAccountantReviewLink(source)}>{copiedAccountantSource === source.source_company_id ? 'Accountant link copied' : 'Copy accountant review link'}</button>
						{/if}
					</div>
				</li>
			{/each}
		</ul>

		{#if safeContinuationAvailable}
			<div class="actions">
				<button class="btn btn-secondary" type="button" disabled={busy} onclick={() => void continueSafeWorkflow()}>{busy ? 'Continuing…' : 'Continue safe workflow'}</button>
			</div>
		{/if}

		{#if reviewSourceCount > 0}
			<div class="review-blocker">
				<strong>{reviewSourceCount} {reviewSourceCount === 1 ? 'company needs' : 'companies need'} review</strong>
				<p class="help">Review the server-bound discovery/schema result for the affected tenant. The runner will not guess a CSV schema, skip a company, or open transfer while a selected/all source is blocked.</p>
			</div>
		{/if}

		{#if allSchemaApproved && !transferOpen && !transferHasBeenConfirmed}
			<button class="btn btn-secondary" type="button" disabled={busy} onclick={() => void openTransfer()}>{busy ? 'Checking readiness…' : 'Check batch transfer readiness'}</button>
		{/if}

		{#if transferOpen && !transferHasBeenConfirmed}
			<div class="transfer-confirmation">
				<h4>Confirm source transfer</h4>
				<p>This is the immediate action-time gate before Open Accounting asks Brave to transfer the reviewed partial journal CSV scope. It is not financial apply.</p>
				<label class="confirm"><input type="checkbox" bind:checked={transferConfirmed} /> I confirm the exact reviewed schema set and authorize this immutable partial source transfer for all {sourceCount} selected company tenants. I understand each GL apply remains separately confirmed in its tenant.</label>
				<button class="btn btn-primary" type="button" disabled={busy || !transferConfirmed || !workflow.schema_readiness_sha256} onclick={() => void confirmTransfer()}>{busy ? 'Confirming…' : `Confirm transfer for ${sourceCount} companies`}</button>
			</div>
		{/if}

		{#if transferHasBeenConfirmed && sources.some((source) => source.phase === 'TRANSFER_CONFIRMATION_REQUIRED' || source.phase === 'FAILED_RETRYABLE')}
			<div class="transfer-confirmation">
				<h4>Resume approved transfer</h4>
				<p class="help">The original source-transfer scope is immutable. A retry uses only the same batch, tenant binding, schema set, and cutoff; the server must still issue a fresh short-lived authorization.</p>
				<label class="confirm"><input type="checkbox" bind:checked={resumeTransferConfirmed} /> I reconfirm this exact immutable transfer scope before resuming the next company.</label>
				<button class="btn btn-secondary" type="button" disabled={busy || !resumeTransferConfirmed} onclick={() => void resumeTransfer()}>{busy ? 'Resuming transfer…' : 'Resume approved transfer'}</button>
			</div>
		{/if}

		{#if sources.length > 0}
			<SmartAccountsReconciliationPanel {batchId} {sources} {companyNames} />
		{/if}
	{/if}

	{#if message}<p class="help" aria-live="polite">{message}</p>{/if}
</section>

<style>
	.batch-runner { margin-top: 1rem; }
	.heading, .source-list li { align-items: start; display: flex; gap: 1rem; justify-content: space-between; }
	.heading h3 { margin: 0; }
	.status-pill { border-radius: 999px; background: #e0e7ff; color: #3730a3; font-size: 0.8rem; font-weight: 700; padding: 0.25rem 0.6rem; white-space: nowrap; }
	.help { color: var(--color-text-muted, #64748b); font-size: 0.85rem; }
	.confirm { display: block; margin: 0.75rem 0; }
	.company-choice { align-items: center; display: flex; gap: 0.55rem; margin: 0.55rem 0; }
	.compact-grid { max-width: 24rem; }
	.workflow-summary, .review-blocker, .transfer-confirmation { border-top: 1px solid var(--color-border, #e2e8f0); margin-top: 1rem; padding-top: 0.75rem; }
	.source-list { display: grid; gap: 0.75rem; list-style: none; margin: 1rem 0; padding: 0; }
	.source-list li { border: 1px solid var(--color-border, #e2e8f0); border-radius: 0.5rem; padding: 0.75rem; }
	.source-links { align-items: flex-start; display: flex; flex-direction: column; gap: 0.5rem; }
	.phase { color: var(--color-text-muted, #64748b); display: block; font-size: 0.85rem; margin-top: 0.2rem; text-transform: capitalize; }
	.actions { display: flex; flex-wrap: wrap; gap: 0.75rem; margin-top: 0.75rem; }
	.link-button { white-space: nowrap; }
</style>
