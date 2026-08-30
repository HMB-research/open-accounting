<script lang="ts">
	import {
		api,
		type SmartAccountsBrowserBatchWorkflowSource,
		type SmartAccountsFullClaimEligibility,
		type SmartAccountsReconciliationEvaluation,
		type SmartAccountsReconciliationRollup,
		type SmartAccountsTolerancePolicyCandidate
	} from '$lib/api';

	type SourcePolicy = {
		candidate?: SmartAccountsTolerancePolicyCandidate;
		confirmed?: true;
	};
	type SourceReadAccess = 'owner' | 'accountant';

	let {
		batchId,
		sources,
		companyNames = {},
	}: {
		batchId: string;
		sources: SmartAccountsBrowserBatchWorkflowSource[];
		companyNames?: Record<string, string>;
	} = $props();

	let evaluations = $state<Record<string, SmartAccountsReconciliationEvaluation>>({});
	let sourceReadAccess = $state<Record<string, SourceReadAccess>>({});
	let policies = $state<Record<string, SourcePolicy>>({});
	let rollup = $state<SmartAccountsReconciliationRollup | null>(null);
	let fullClaimEligibility = $state<SmartAccountsFullClaimEligibility | null>(null);
	let policyConfirmed = $state<Record<string, boolean>>({});
	let finalConfirmed = $state<Record<string, boolean>>({});
	let loading = $state(false);
	let busySource = $state('');
	let message = $state('');
	let error = $state('');
	let loadedBatchId = '';
	let autoPreparedEvidence = new Set<string>();

	function sourceName(source: SmartAccountsBrowserBatchWorkflowSource): string {
		return companyNames[source.source_company_id] ?? `Company ${source.ordinal + 1}`;
	}

	function sourceReadyForPolicy(source: SmartAccountsBrowserBatchWorkflowSource): boolean {
		return Boolean(source.tenant_id && source.package_id && source.preview_id && source.preview_sha256);
	}

	function sourceKey(source: SmartAccountsBrowserBatchWorkflowSource): string {
		return source.source_company_id;
	}

	function automaticEvaluationKey(source: SmartAccountsBrowserBatchWorkflowSource, evaluation: SmartAccountsReconciliationEvaluation): string {
		return `${source.source_company_id}:${evaluation.evidence_sha256 ?? evaluation.updated_at}`;
	}

	function isDigest(value: string | undefined): value is string {
		return Boolean(value && /^[0-9a-f]{64}$/.test(value));
	}

	function isAccessDenied(caught: unknown): boolean {
		return caught instanceof Error && /(?:status 403|forbidden|permission|owner|accountant)/i.test(caught.message);
	}

	function isMissing(caught: unknown): boolean {
		return caught instanceof Error && /(?:status 404|not found)/i.test(caught.message);
	}

	function statusLabel(evaluation: SmartAccountsReconciliationEvaluation | undefined): string {
		if (!evaluation) return 'Not prepared';
		switch (evaluation.status) {
			case 'EVIDENCE_PENDING': return 'Evidence pending';
			case 'READY_FOR_ACCOUNTANT': return 'Ready for accountant';
			case 'PASS': return 'Accountant attested';
			case 'PARTIAL_FAILURE': return 'Partial or blocked';
			default: return 'Not prepared';
		}
	}

	function statusExplanation(evaluation: SmartAccountsReconciliationEvaluation | undefined): string {
		if (!evaluation) return 'Prepare an owner-safe technical status when the staged source is ready. This action never transfers or applies data.';
		if (evaluation.claim_kind === 'partial' || evaluation.expected_coverage_state === 'partial') return 'This source is partial evidence only. It is not a full sync and cannot be marked reconciled.';
		switch (evaluation.status) {
			case 'EVIDENCE_PENDING': return 'Technical evidence is incomplete. Apply, exact replay, reference review, coverage, or accountant steps may still be required.';
			case 'READY_FOR_ACCOUNTANT': return 'The technical proof is ready for a separate accountant attestation; no accounting action is started here.';
			case 'PASS': return 'This source has a current accountant attestation. The selected/all batch remains complete only when its aggregate roll-up passes.';
			case 'PARTIAL_FAILURE': return 'The source has terminal review blockers. It is not a successful full sync.';
			default: return 'Technical reconciliation has not been evaluated.';
		}
	}

	function messageFrom(caught: unknown, fallback: string): string {
		return caught instanceof Error && caught.message ? caught.message : fallback;
	}

	async function refresh() {
		if (!batchId || loading) return;
		loading = true;
		error = '';
		try {
			const entries = await Promise.all(sources.map(async (source) => {
				try {
					return { sourceCompanyId: source.source_company_id, access: 'owner' as const, evaluation: await api.getSmartAccountsReconciliation(batchId, source.source_company_id) };
				} catch (ownerError) {
					if (isMissing(ownerError)) return { sourceCompanyId: source.source_company_id, access: 'owner' as const, evaluation: undefined };
					if (!isAccessDenied(ownerError)) throw ownerError;
					const accountantEvaluation = await api.getSmartAccountsTenantReconciliation(source.tenant_id, batchId, source.source_company_id);
					return { sourceCompanyId: source.source_company_id, access: 'accountant' as const, evaluation: accountantEvaluation };
				}
			}));
			const nextEvaluations = Object.fromEntries(entries
				.filter((entry): entry is typeof entry & { evaluation: SmartAccountsReconciliationEvaluation } => entry.evaluation !== undefined)
				.map((entry) => [entry.sourceCompanyId, entry.evaluation]));
			const nextReadAccess = Object.fromEntries(entries.map((entry) => [entry.sourceCompanyId, entry.access]));
			// A replay-verified GL preview is the only state in which the owner-safe
			// technical evaluation can be retried automatically. It is idempotent
			// server work and never transfers, approves, or applies finance. Every
			// other state deliberately leaves one explicit Prepare action.
			const automatic = await Promise.all(sources.map(async (source) => {
				const current = nextEvaluations[source.source_company_id];
				if (nextReadAccess[source.source_company_id] !== 'owner' || !current || current.status !== 'EVIDENCE_PENDING' || current.gl_state !== 'APPLIED_REPLAY_VERIFIED') return null;
				const key = automaticEvaluationKey(source, current);
				if (autoPreparedEvidence.has(key)) return null;
				autoPreparedEvidence.add(key);
				try {
					const response = await api.evaluateSmartAccountsReconciliation(batchId, source.source_company_id);
					return [source.source_company_id, response.evaluation] as const;
				} catch {
					// Keep the safe GET state in view so the owner can make one explicit
					// Prepare attempt and receive the server's actionable rejection.
					return null;
				}
			}));
			for (const entry of automatic) {
				if (entry) nextEvaluations[entry[0]] = entry[1];
			}
			evaluations = nextEvaluations;
			sourceReadAccess = nextReadAccess;
			try {
				rollup = await api.getSmartAccountsReconciliationRollup(batchId);
			} catch {
				rollup = null;
			}
			// The full-claim gate is owner-only and contains aggregate counts plus
			// fixed product-coverage codes. An accountant handoff must not probe a
			// selected/all aggregate that can reveal cross-tenant progress.
			if (entries.every((entry) => entry.access === 'owner')) {
				try {
					fullClaimEligibility = await api.getSmartAccountsFullClaimEligibility(batchId);
				} catch {
					fullClaimEligibility = null;
				}
			} else {
				fullClaimEligibility = null;
			}
			message = entries.some((entry) => entry.access === 'accountant')
				? 'Accountant-safe per-company status refreshed. Selected/all aggregate roll-up remains owner-only.'
				: 'Safe reconciliation status refreshed.';
		} catch (caught) {
			error = messageFrom(caught, 'Could not refresh the safe reconciliation status.');
		} finally {
			loading = false;
		}
	}

	async function prepare(source: SmartAccountsBrowserBatchWorkflowSource) {
		if (busySource) return;
		if (sourceReadAccess[source.source_company_id] === 'accountant') {
			error = 'Only a batch owner can prepare technical reconciliation. Accountants can review the safe status and attest ready evidence.';
			return;
		}
		busySource = source.source_company_id;
		error = '';
		try {
			const response = await api.evaluateSmartAccountsReconciliation(batchId, source.source_company_id);
			evaluations = { ...evaluations, [source.source_company_id]: response.evaluation };
			message = response.reused ? `${sourceName(source)} already has this exact technical evidence.` : `${sourceName(source)} technical evidence was prepared without applying data.`;
			await refresh();
		} catch (caught) {
			error = messageFrom(caught, 'Technical reconciliation could not be prepared.');
		} finally {
			busySource = '';
		}
	}

	async function loadPolicyCandidate(source: SmartAccountsBrowserBatchWorkflowSource) {
		if (!sourceReadyForPolicy(source) || busySource) return;
		const { tenant_id: tenantId, package_id: packageId, preview_id: previewId } = source;
		if (!tenantId || !packageId || !previewId) return;
		busySource = source.source_company_id;
		error = '';
		try {
			const candidate = await api.getSmartAccountsTolerancePolicyCandidate(tenantId, source.source_company_id, {
				package_id: packageId,
				preview_id: previewId
			});
			policies = { ...policies, [sourceKey(source)]: { ...policies[sourceKey(source)], candidate } };
			policyConfirmed = { ...policyConfirmed, [sourceKey(source)]: false };
			message = `${sourceName(source)} has a server-derived accountant policy candidate. No policy rule, source row, or monetary value was returned.`;
		} catch (caught) {
			error = messageFrom(caught, 'An accountant policy candidate is unavailable for this exact staged preview.');
		} finally {
			busySource = '';
		}
	}

	async function confirmPolicy(source: SmartAccountsBrowserBatchWorkflowSource) {
		const state = policies[sourceKey(source)];
		if (!sourceReadyForPolicy(source) || !state?.candidate || !policyConfirmed[sourceKey(source)] || busySource) return;
		const { tenant_id: tenantId, package_id: packageId, preview_id: previewId } = source;
		if (!tenantId || !packageId || !previewId) return;
		busySource = source.source_company_id;
		error = '';
		try {
			await api.approveSmartAccountsTolerancePolicy(tenantId, source.source_company_id, {
				confirmed: true,
				package_id: packageId,
				preview_id: previewId,
				expected_candidate_sha256: state.candidate.candidate_sha256
			});
			// Forget candidate and confirmation identifiers immediately. A future
			// financial action resolves the persisted policy afresh and keeps its
			// opaque ID only for that one submit.
			policies = { ...policies, [sourceKey(source)]: { confirmed: true } };
			message = `${sourceName(source)} tolerance policy is confirmed for this exact staged preview. A separate financial operator still must confirm GL apply.`;
		} catch (caught) {
			error = messageFrom(caught, 'The accountant policy confirmation was blocked. Use an active accountant session for the exact current preview.');
		} finally {
			busySource = '';
		}
	}

	async function attest(source: SmartAccountsBrowserBatchWorkflowSource, evaluation: SmartAccountsReconciliationEvaluation) {
		if (!finalConfirmed[sourceKey(source)] || !isDigest(evaluation.evidence_sha256) || !isDigest(evaluation.tolerance_sha256) || busySource) return;
		busySource = source.source_company_id;
		error = '';
		try {
			const next = await api.approveSmartAccountsReconciliation(source.tenant_id, evaluation.evaluation_id, {
				confirmed: true,
				evidence_sha256: evaluation.evidence_sha256,
				tolerance_sha256: evaluation.tolerance_sha256
			});
			evaluations = { ...evaluations, [source.source_company_id]: next };
			message = `${sourceName(source)} accountant attestation was recorded. Refreshing the selected/all aggregate.`;
			await refresh();
		} catch (caught) {
			error = messageFrom(caught, 'The accountant attestation was blocked because evidence is stale, incomplete, or not independent.');
		} finally {
			busySource = '';
		}
	}

	$effect(() => {
		if (!batchId || batchId === loadedBatchId) return;
		loadedBatchId = batchId;
		evaluations = {};
		sourceReadAccess = {};
		policies = {};
		rollup = null;
		fullClaimEligibility = null;
		autoPreparedEvidence = new Set<string>();
		void refresh();
	});
</script>

<section class="reconciliation-panel package-review" aria-labelledby="smartaccounts-reconciliation-heading">
	<div class="heading">
		<div>
			<h3 id="smartaccounts-reconciliation-heading">Selected/all reconciliation</h3>
			<p class="help">This records and reviews safe digest-bound evidence only. It never displays source rows, proof payloads, amounts, or browser capabilities, and it never starts a financial apply.</p>
		</div>
		<button class="btn btn-secondary" type="button" disabled={loading || Boolean(busySource)} onclick={() => void refresh()}>{loading ? 'Refreshing…' : 'Refresh reconciliation status'}</button>
	</div>

	{#if rollup}
		<div class="rollup" aria-live="polite">
			<strong>Batch {rollup.status.replaceAll('_', ' ').toLowerCase()}</strong>
			<span>{rollup.pass_count}/{rollup.selected_count} attested, {rollup.pending_count} pending, {rollup.review_count} awaiting accountant, {rollup.failure_count} partial/blocked.</span>
			{#if rollup.status === 'PASS'}<p class="help">Every originally selected source has passed its independent accountant attestation.</p>{/if}
			{#if rollup.status === 'PARTIAL_FAILURE'}<p class="help">At least one selected source is terminally blocked or partial. This batch is not a full sync.</p>{/if}
		</div>
	{/if}

	{#if fullClaimEligibility}
		<div class="rollup full-claim" aria-live="polite">
			<strong>Full-sync eligibility: {fullClaimEligibility.full_claim_eligible ? 'eligible' : 'blocked'}</strong>
			<span>{fullClaimEligibility.current_pass_count}/{fullClaimEligibility.selected_count} selected sources have a current PASS; {fullClaimEligibility.matrix_blocker_count} fixed product-coverage blockers remain.</span>
			{#if !fullClaimEligibility.full_claim_eligible}
				<p class="help">This count-only gate is separate from accountant attestation and cannot apply data. The current product matrix is incomplete, so this batch must not be labelled a full sync.</p>
			{/if}
			{#if fullClaimEligibility.blocking_codes?.length}
				<ul class="blockers">{#each fullClaimEligibility.blocking_codes as blocker (blocker)}<li>{blocker.replaceAll('_', ' ')}</li>{/each}</ul>
			{/if}
		</div>
	{/if}

	<ul class="source-list" aria-label="Per-company reconciliation status">
		{#each sources as source (source.source_company_id)}
			{@const evaluation = evaluations[source.source_company_id]}
			{@const policy = policies[source.source_company_id]}
			<li>
				<div>
					<strong>{sourceName(source)}</strong>
					<span class="phase">{statusLabel(evaluation)}</span>
					<p class="help">{statusExplanation(evaluation)}</p>
					{#if evaluation}
						<p class="safe-state">GL: {evaluation.gl_state.replaceAll('_', ' ').toLowerCase()}; reference evidence: {evaluation.reference_state.replaceAll('_', ' ').toLowerCase()}.</p>
						{#if evaluation.gl_state === 'APPLIED_REPLAY_VERIFIED'}<p class="help">The server verified an exact replay of the applied GL preview. Preparing technical evidence is now safe; it still does not mean the source is reconciled.</p>{/if}
						{#if (evaluation.gl_revision_unresolved + evaluation.gl_tombstone_unresolved + evaluation.reference_revision_unresolved + evaluation.reference_tombstone_unresolved) > 0}
							<p class="help">Revision/tombstone review remains required before technical reconciliation can be ready.</p>
						{/if}
						{#if evaluation.blockers?.length}<ul class="blockers">{#each evaluation.blockers as blocker (blocker)}<li>{blocker.replaceAll('_', ' ')}</li>{/each}</ul>{/if}
					{/if}
				</div>
				<div class="actions source-actions">
					{#if sourceReadAccess[source.source_company_id] === 'accountant'}
						<p class="help">Accountant-safe evaluation view. Technical preparation and selected/all aggregate roll-up remain owner actions.</p>
					{:else}
						<button class="btn btn-secondary" type="button" disabled={Boolean(busySource)} onclick={() => void prepare(source)}>{busySource === source.source_company_id ? 'Preparing…' : evaluation ? 'Recheck technical evidence' : 'Prepare technical reconciliation'}</button>
					{/if}
					{#if sourceReadyForPolicy(source) && !policy?.confirmed}
						<button class="btn btn-secondary" type="button" disabled={Boolean(busySource)} onclick={() => void loadPolicyCandidate(source)}>{busySource === source.source_company_id ? 'Loading…' : 'Load accountant policy candidate'}</button>
					{/if}
					{#if policy?.candidate && !policy.confirmed}
						<p class="help">{policy.candidate.label}. The candidate is server-derived for this exact package and preview.</p>
						<label class="confirm"><input type="checkbox" checked={policyConfirmed[source.source_company_id] ?? false} onchange={(event) => { policyConfirmed = { ...policyConfirmed, [source.source_company_id]: (event.currentTarget as HTMLInputElement).checked }; }} /> As the accountant, I confirm this exact policy candidate for this staged preview.</label>
						<button class="btn btn-secondary" type="button" disabled={Boolean(busySource) || !policyConfirmed[source.source_company_id]} onclick={() => void confirmPolicy(source)}>{busySource === source.source_company_id ? 'Confirming…' : 'Confirm accountant policy'}</button>
					{/if}
					{#if policy?.confirmed}<p class="help">Accountant policy confirmed for this exact preview. It remains separate from financial GL apply.</p>{/if}
					{#if evaluation?.status === 'READY_FOR_ACCOUNTANT'}
						<label class="confirm"><input type="checkbox" checked={finalConfirmed[source.source_company_id] ?? false} onchange={(event) => { finalConfirmed = { ...finalConfirmed, [source.source_company_id]: (event.currentTarget as HTMLInputElement).checked }; }} /> As an independent accountant, I attest the current technical evidence for this source.</label>
						<button class="btn btn-primary" type="button" disabled={Boolean(busySource) || !finalConfirmed[source.source_company_id]} onclick={() => void attest(source, evaluation)}>{busySource === source.source_company_id ? 'Attesting…' : 'Approve current reconciliation evidence'}</button>
					{/if}
				</div>
			</li>
		{/each}
	</ul>

	{#if message}<p class="help" aria-live="polite">{message}</p>{/if}
	{#if error}<p class="error" role="alert">{error}</p>{/if}
</section>

<style>
	.reconciliation-panel { margin-top: 1rem; }
	.heading, .source-list li { align-items: start; display: flex; gap: 1rem; justify-content: space-between; }
	.heading h3 { margin: 0; }
	.help, .safe-state, .phase { color: var(--color-text-muted, #64748b); font-size: 0.85rem; }
	.phase { display: block; margin-top: 0.2rem; text-transform: capitalize; }
	.rollup { border-top: 1px solid var(--color-border, #e2e8f0); display: grid; gap: 0.3rem; margin-top: 1rem; padding-top: 0.75rem; }
	.source-list { display: grid; gap: 0.75rem; list-style: none; margin: 1rem 0; padding: 0; }
	.source-list li { border: 1px solid var(--color-border, #e2e8f0); border-radius: 0.5rem; padding: 0.75rem; }
	.actions { display: flex; flex-wrap: wrap; gap: 0.75rem; }
	.source-actions { align-items: flex-start; flex-direction: column; min-width: 15rem; }
	.confirm { display: block; font-size: 0.85rem; }
	.blockers { color: var(--color-text-muted, #64748b); font-size: 0.8rem; margin: 0.5rem 0 0; padding-left: 1.2rem; }
	.error { color: var(--color-danger, #b91c1c); }
</style>
