<script lang="ts">
	import {
		api,
		type SmartAccountsReconciliationEvaluation,
		type SmartAccountsTolerancePolicyCandidate
	} from '$lib/api';

	let {
		tenantId,
		batchId,
		sourceCompanyId
	}: {
		tenantId: string;
		batchId: string;
		sourceCompanyId: string;
	} = $props();

	let evaluation = $state<SmartAccountsReconciliationEvaluation | null>(null);
	let policyCandidate = $state<SmartAccountsTolerancePolicyCandidate | null>(null);
	let loading = $state(false);
	let policyBusy = $state(false);
	let approving = $state(false);
	let policyConfirmed = $state(false);
	let policyApproved = $state(false);
	let confirmed = $state(false);
	let message = $state('');
	let error = $state('');
	let loadedContext = '';

	// This is deliberately a lexical validation only. The accountant-only API
	// remains authoritative for the tenant/batch/source relationship and role.
	// Rejecting unsafe values here prevents a deep link from becoming an
	// arbitrary path construction or an endpoint call with malformed IDs.
	function isSafeOpaqueId(value: string): boolean {
		return /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(value);
	}

	function validContext(): boolean {
		return isSafeOpaqueId(tenantId) && isSafeOpaqueId(batchId) && isSafeOpaqueId(sourceCompanyId);
	}

	function matchesContext(value: SmartAccountsReconciliationEvaluation): boolean {
		return value.tenant_id === tenantId && value.batch_id === batchId && value.source_company_id === sourceCompanyId;
	}

	function isDigest(value: string | undefined): value is string {
		return Boolean(value && /^[0-9a-f]{64}$/.test(value));
	}

	function canReviewPolicy(value: SmartAccountsReconciliationEvaluation | null): value is SmartAccountsReconciliationEvaluation & { package_id: string; gl_preview_id: string } {
		// Policy approval is meaningful only before the owner performs the
		// separate GL action. A READY/PASS evaluation already represents later
		// technical evidence/attestation state, so do not offer a redundant
		// policy action there.
		return Boolean(value && value.status === 'EVIDENCE_PENDING' && isSafeOpaqueId(value.package_id ?? '') && isSafeOpaqueId(value.gl_preview_id ?? ''));
	}

	function statusLabel(value: SmartAccountsReconciliationEvaluation | null): string {
		if (!value) return 'Not evaluated';
		switch (value.status) {
			case 'EVIDENCE_PENDING': return 'Evidence pending';
			case 'READY_FOR_ACCOUNTANT': return 'Ready for accountant';
			case 'PASS': return 'Accountant attested';
			case 'PARTIAL_FAILURE': return 'Partial or blocked';
			default: return 'Not evaluated';
		}
	}

	function statusExplanation(value: SmartAccountsReconciliationEvaluation | null): string {
		if (!value) return 'The server has not supplied a current accountant-safe evaluation for this handoff.';
		if (value.claim_kind === 'partial' || value.expected_coverage_state === 'partial') return 'This source has partial evidence only. It is not a full sync and cannot be marked reconciled.';
		switch (value.status) {
			case 'EVIDENCE_PENDING': return 'Technical evidence is incomplete. An owner must complete the separate technical workflow before accountant attestation is available.';
			case 'READY_FOR_ACCOUNTANT': return 'The current technical evidence can be independently attested. This action does not apply financial data.';
			case 'PASS': return 'The server records a current accountant attestation for this source. Selected/all aggregate status remains owner-visible only.';
			case 'PARTIAL_FAILURE': return 'This source is partial or blocked and is not a successful full sync.';
			default: return 'The server has not supplied a current accountant-safe evaluation for this handoff.';
		}
	}

	function messageFrom(caught: unknown, fallback: string): string {
		return caught instanceof Error && caught.message ? caught.message : fallback;
	}

	async function refresh() {
		if (!validContext() || loading || policyBusy || approving) return;
		loading = true;
		error = '';
		try {
			// Deliberately use only the accountant-safe tenant/batch/source route.
			// This deep-link view never calls owner workflow, status, roll-up,
			// browser, source, or financial-apply endpoints.
			const next = await api.getSmartAccountsTenantReconciliation(tenantId, batchId, sourceCompanyId);
			if (!matchesContext(next)) throw new Error('Accountant review response binding mismatch. Request a fresh link from the batch owner.');
			evaluation = next;
			policyCandidate = null;
			policyConfirmed = false;
			policyApproved = false;
			confirmed = false;
			message = 'Accountant-safe reconciliation status refreshed.';
		} catch (caught) {
			evaluation = null;
			error = messageFrom(caught, 'This accountant review handoff is unavailable, stale, or not authorized for the current tenant.');
		} finally {
			loading = false;
		}
	}

	async function loadPolicyCandidate() {
		if (!canReviewPolicy(evaluation) || policyBusy || loading || approving) return;
		const current = evaluation;
		policyBusy = true;
		error = '';
		try {
			// OA derives the current package/scope/preview binding server-side. The
			// candidate digest is held only until the immediately following POST.
		const candidate = await api.getSmartAccountsTolerancePolicyCandidate(tenantId, sourceCompanyId, {
				package_id: current.package_id,
				preview_id: current.gl_preview_id
			});
			if (
				candidate.algorithm_version !== 'smartaccounts-exact-match-v1' ||
				!isDigest(candidate.candidate_sha256) ||
				typeof candidate.label !== 'string' ||
				candidate.label.length === 0 ||
				candidate.label.length > 160
			) throw new Error('Accountant policy candidate is malformed. Refresh the handoff before confirming.');
			if (evaluation !== current || !matchesContext(current)) throw new Error('Accountant policy context changed. Refresh the handoff before confirming.');
			policyCandidate = candidate;
			policyConfirmed = false;
			policyApproved = false;
			message = 'A server-derived exact-match policy candidate is ready for this staged preview. No policy rule, source row, amount, or digest is displayed.';
		} catch (caught) {
			policyCandidate = null;
			error = messageFrom(caught, 'The accountant policy candidate is unavailable for this exact staged preview.');
		} finally {
			policyBusy = false;
		}
	}

	async function approvePolicy() {
		if (!canReviewPolicy(evaluation) || !policyCandidate || !policyConfirmed || policyBusy || loading || approving) return;
		const current = evaluation;
		const candidate = policyCandidate;
		policyBusy = true;
		error = '';
		try {
			// Discard the returned policy object (including its derived digest).
			// The owner later resolves an opaque policy ID only for the one apply.
			await api.approveSmartAccountsTolerancePolicy(tenantId, sourceCompanyId, {
				confirmed: true,
				package_id: current.package_id,
				preview_id: current.gl_preview_id,
				expected_candidate_sha256: candidate.candidate_sha256
			});
			if (evaluation !== current || !matchesContext(current)) throw new Error('Accountant policy context changed. Refresh the handoff before continuing.');
			policyCandidate = null;
			policyConfirmed = false;
			policyApproved = true;
			message = 'The exact accountant policy is approved for this staged preview. It remains separate from financial GL apply and final reconciliation attestation.';
		} catch (caught) {
			error = messageFrom(caught, 'The accountant policy approval was blocked because the staged preview is stale, unavailable, or not authorized.');
		} finally {
			policyBusy = false;
		}
	}

	async function approve() {
		if (!evaluation || evaluation.status !== 'READY_FOR_ACCOUNTANT' || !confirmed || approving || policyBusy || !isDigest(evaluation.evidence_sha256) || !isDigest(evaluation.tolerance_sha256)) return;
		approving = true;
		error = '';
		try {
			// Evidence handles are held in component memory only for this one
			// approval request. They are never rendered, placed in the URL, or
			// stored in browser storage.
			const next = await api.approveSmartAccountsReconciliation(tenantId, evaluation.evaluation_id, {
				confirmed: true,
				evidence_sha256: evaluation.evidence_sha256,
				tolerance_sha256: evaluation.tolerance_sha256
			});
			if (!matchesContext(next)) {
				evaluation = null;
				throw new Error('Accountant approval response binding mismatch. Refresh with a new owner handoff link.');
			}
			evaluation = next;
			confirmed = false;
			message = 'The current accountant attestation was recorded. This does not claim a selected/all full sync.';
		} catch (caught) {
			error = messageFrom(caught, 'The accountant attestation was blocked because evidence is stale, incomplete, or not independent.');
		} finally {
			approving = false;
		}
	}

	$effect(() => {
		const context = `${tenantId}\u0000${batchId}\u0000${sourceCompanyId}`;
		if (context === loadedContext) return;
		loadedContext = context;
		evaluation = null;
		policyCandidate = null;
		policyConfirmed = false;
		policyApproved = false;
		confirmed = false;
		message = '';
		error = '';
		if (!validContext()) {
			error = 'This accountant review link has invalid identifiers. Request a fresh link from the batch owner.';
			return;
		}
		void refresh();
	});
</script>

<section class="accountant-review package-review" aria-labelledby="smartaccounts-accountant-review-heading">
	<div class="heading">
		<div>
			<h3 id="smartaccounts-accountant-review-heading">Accountant reconciliation review</h3>
			<p class="help">This handoff uses only the tenant-, batch-, and source-bound accountant-safe reconciliation API. It does not expose or store source names, source rows, proof payloads, amounts, browser capabilities, tokens, or evidence digests.</p>
		</div>
		<button class="btn btn-secondary" type="button" disabled={!validContext() || loading || policyBusy || approving} onclick={() => void refresh()}>{loading ? 'Refreshing…' : 'Refresh accountant-safe status'}</button>
	</div>

	{#if evaluation}
		<div class="safe-state" aria-live="polite">
			<strong>{statusLabel(evaluation)}</strong>
			<p>{statusExplanation(evaluation)}</p>
			<p class="help">GL: {evaluation.gl_state.replaceAll('_', ' ').toLowerCase()}; reference evidence: {evaluation.reference_state.replaceAll('_', ' ').toLowerCase()}.</p>
			{#if evaluation.blockers?.length}
				<ul class="blockers">{#each evaluation.blockers as blocker (blocker)}<li>{blocker.replaceAll('_', ' ')}</li>{/each}</ul>
			{/if}
		</div>
		{#if canReviewPolicy(evaluation) && !policyApproved}
			<div class="policy-action">
				<p class="help">The owner prepared a server-bound staged preview. Independently approve its exact-match policy before that owner can separately confirm financial GL apply.</p>
				{#if !policyCandidate}
					<button class="btn btn-secondary" type="button" disabled={loading || policyBusy || approving} onclick={() => void loadPolicyCandidate()}>{policyBusy ? 'Loading policy candidate…' : 'Load accountant policy candidate'}</button>
				{:else}
					<p class="help">{policyCandidate.label}. The candidate is server-derived for the current tenant/source/package/preview binding.</p>
					<label class="confirm"><input type="checkbox" bind:checked={policyConfirmed} /> As an accountant, I confirm this exact policy candidate for the staged preview.</label>
					<button class="btn btn-secondary" type="button" disabled={policyBusy || !policyConfirmed} onclick={() => void approvePolicy()}>{policyBusy ? 'Confirming policy…' : 'Confirm accountant policy'}</button>
				{/if}
			</div>
		{:else if policyApproved}
			<p class="help">Accountant policy approval is recorded for the current staged preview. The owner must still separately review and confirm GL apply.</p>
		{/if}
		{#if evaluation.status === 'READY_FOR_ACCOUNTANT'}
			<label class="confirm"><input type="checkbox" bind:checked={confirmed} /> As an independent accountant, I attest the current technical evidence for this source.</label>
			<button class="btn btn-primary" type="button" disabled={approving || !confirmed} onclick={() => void approve()}>{approving ? 'Attesting…' : 'Approve current reconciliation evidence'}</button>
		{/if}
	{/if}

	{#if message}<p class="help" aria-live="polite">{message}</p>{/if}
	{#if error}<p class="error" role="alert">{error}</p>{/if}
</section>

<style>
	.accountant-review { margin-top: 1rem; }
	.heading { align-items: start; display: flex; gap: 1rem; justify-content: space-between; }
	.heading h3 { margin: 0; }
	.help, .safe-state { color: var(--color-text-muted, #64748b); font-size: 0.85rem; }
	.safe-state { border-top: 1px solid var(--color-border, #e2e8f0); margin-top: 1rem; padding-top: 0.75rem; }
	.policy-action { border-top: 1px solid var(--color-border, #e2e8f0); margin-top: 1rem; padding-top: 0.75rem; }
	.confirm { display: block; margin: 0.75rem 0; }
	.blockers { color: var(--color-text-muted, #64748b); font-size: 0.8rem; margin: 0.5rem 0 0; padding-left: 1.2rem; }
	.error { color: var(--color-danger, #b91c1c); }
</style>
