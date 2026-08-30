import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import type { SmartAccountsReconciliationEvaluation } from '$lib/api';

const { apiMock } = vi.hoisted(() => ({
	apiMock: {
		getSmartAccountsTenantReconciliation: vi.fn(),
		getSmartAccountsTolerancePolicyCandidate: vi.fn(),
		approveSmartAccountsTolerancePolicy: vi.fn(),
		approveSmartAccountsReconciliation: vi.fn()
	}
}));

vi.mock('$lib/api', async () => {
	const actual = await vi.importActual<typeof import('$lib/api')>('$lib/api');
	return { ...actual, api: apiMock };
});

import SmartAccountsAccountantReconciliationReview from '$lib/components/SmartAccountsAccountantReconciliationReview.svelte';

const evidenceDigest = 'a'.repeat(64);
const toleranceDigest = 'b'.repeat(64);

function evaluation(overrides: Partial<SmartAccountsReconciliationEvaluation> = {}): SmartAccountsReconciliationEvaluation {
	return {
		evaluation_id: 'evaluation-1',
		batch_id: 'batch-1',
		source_company_id: 'sa-browser-v1-hmb',
		tenant_id: 'tenant-1',
		package_id: 'package-1',
		gl_preview_id: 'preview-1',
		gl_preview_sha256: evidenceDigest,
		gl_state: 'APPLIED_REPLAY_VERIFIED',
		reference_state: 'APPLIED',
		claim_kind: 'full',
		expected_coverage_state: 'full',
		variance_within_policy: true,
		gl_revision_unresolved: 0,
		gl_tombstone_unresolved: 0,
		reference_revision_unresolved: 0,
		reference_tombstone_unresolved: 0,
		blockers: [],
		evidence_sha256: evidenceDigest,
		tolerance_sha256: toleranceDigest,
		status: 'READY_FOR_ACCOUNTANT',
		created_at: '2026-08-28T10:00:00Z',
		updated_at: '2026-08-28T10:00:00Z',
		...overrides
	};
}

describe('SmartAccountsAccountantReconciliationReview', () => {
	beforeEach(() => {
		vi.resetAllMocks();
		window.sessionStorage.clear();
		apiMock.getSmartAccountsTenantReconciliation.mockResolvedValue(evaluation());
	});

	afterEach(cleanup);

	it('uses only the accountant-safe bound GET and approval routes for a valid handoff', async () => {
		const response = evaluation({ status: 'PASS', accountant_approved_at: '2026-08-28T10:05:00Z' });
		apiMock.approveSmartAccountsReconciliation.mockResolvedValue(response);
		render(SmartAccountsAccountantReconciliationReview, {
			tenantId: 'tenant-1', batchId: 'batch-1', sourceCompanyId: 'sa-browser-v1-hmb'
		});

		await screen.findByText('Ready for accountant');
		expect(apiMock.getSmartAccountsTenantReconciliation).toHaveBeenCalledWith('tenant-1', 'batch-1', 'sa-browser-v1-hmb');
		await fireEvent.click(screen.getByLabelText(/As an independent accountant/));
		await fireEvent.click(screen.getByRole('button', { name: 'Approve current reconciliation evidence' }));
		await waitFor(() => expect(apiMock.approveSmartAccountsReconciliation).toHaveBeenCalledWith('tenant-1', 'evaluation-1', {
			confirmed: true, evidence_sha256: evidenceDigest, tolerance_sha256: toleranceDigest
		}));
		expect(await screen.findByText('Accountant attested')).toBeInTheDocument();
		expect(document.body.textContent).not.toContain(evidenceDigest);
		expect(document.body.textContent).not.toContain(toleranceDigest);
		expect(window.sessionStorage.length).toBe(0);
	});

	it('fails closed on a malformed server policy candidate and never forwards its digest', async () => {
		const malformedDigest = 'e'.repeat(64);
		apiMock.getSmartAccountsTenantReconciliation.mockResolvedValue(evaluation({ status: 'EVIDENCE_PENDING' }));
		apiMock.getSmartAccountsTolerancePolicyCandidate.mockResolvedValue({
			algorithm_version: 'unexpected-policy-v1',
			label: 'Unexpected policy',
			candidate_sha256: malformedDigest
		});
		render(SmartAccountsAccountantReconciliationReview, {
			tenantId: 'tenant-1', batchId: 'batch-1', sourceCompanyId: 'sa-browser-v1-hmb'
		});

		await screen.findByRole('button', { name: 'Load accountant policy candidate' });
		await fireEvent.click(screen.getByRole('button', { name: 'Load accountant policy candidate' }));
		expect(await screen.findByRole('alert')).toHaveTextContent(/candidate is malformed/i);
		expect(apiMock.approveSmartAccountsTolerancePolicy).not.toHaveBeenCalled();
		expect(document.body.textContent).not.toContain(malformedDigest);
	});

	it('loads then approves only the server-derived policy candidate without rendering or storing either digest', async () => {
		const candidateDigest = 'c'.repeat(64);
		const returnedPolicyDigest = 'd'.repeat(64);
		apiMock.getSmartAccountsTenantReconciliation.mockResolvedValue(evaluation({ status: 'EVIDENCE_PENDING' }));
		apiMock.getSmartAccountsTolerancePolicyCandidate.mockResolvedValue({
			algorithm_version: 'smartaccounts-exact-match-v1',
			label: 'Exact match — zero variance',
			candidate_sha256: candidateDigest
		});
		apiMock.approveSmartAccountsTolerancePolicy.mockResolvedValue({
			policy_id: 'policy-1',
			algorithm_version: 'smartaccounts-exact-match-v1',
			tenant_id: 'tenant-1',
			source_company_id: 'sa-browser-v1-hmb',
			package_id: 'package-1',
			scope_sha256: evidenceDigest,
			preview_sha256: evidenceDigest,
			tolerance_policy_sha256: returnedPolicyDigest,
			approved_at: '2026-08-28T10:05:00Z'
		});
		render(SmartAccountsAccountantReconciliationReview, {
			tenantId: 'tenant-1', batchId: 'batch-1', sourceCompanyId: 'sa-browser-v1-hmb'
		});

		await screen.findByText('Evidence pending');
		await fireEvent.click(screen.getByRole('button', { name: 'Load accountant policy candidate' }));
		await waitFor(() => expect(apiMock.getSmartAccountsTolerancePolicyCandidate).toHaveBeenCalledWith('tenant-1', 'sa-browser-v1-hmb', {
			package_id: 'package-1', preview_id: 'preview-1'
		}));
		expect(screen.getByText('Exact match — zero variance. The candidate is server-derived for the current tenant/source/package/preview binding.')).toBeInTheDocument();
		expect(document.body.textContent).not.toContain(candidateDigest);
		await fireEvent.click(screen.getByLabelText(/As an accountant, I confirm this exact policy candidate/));
		await fireEvent.click(screen.getByRole('button', { name: 'Confirm accountant policy' }));
		await waitFor(() => expect(apiMock.approveSmartAccountsTolerancePolicy).toHaveBeenCalledWith('tenant-1', 'sa-browser-v1-hmb', {
			confirmed: true,
			package_id: 'package-1',
			preview_id: 'preview-1',
			expected_candidate_sha256: candidateDigest
		}));
		expect(await screen.findByText(/Accountant policy approval is recorded/)).toBeInTheDocument();
		expect(document.body.textContent).not.toContain(candidateDigest);
		expect(document.body.textContent).not.toContain(returnedPolicyDigest);
		expect(window.sessionStorage.length).toBe(0);
	});

	it('fails closed without any endpoint call for malformed handoff identifiers', async () => {
		render(SmartAccountsAccountantReconciliationReview, {
			tenantId: '../tenant', batchId: 'batch-1', sourceCompanyId: 'sa-browser-v1-hmb'
		});

		expect(await screen.findByRole('alert')).toHaveTextContent(/invalid identifiers/i);
		expect(apiMock.getSmartAccountsTenantReconciliation).not.toHaveBeenCalled();
		expect(apiMock.getSmartAccountsTolerancePolicyCandidate).not.toHaveBeenCalled();
		expect(apiMock.approveSmartAccountsTolerancePolicy).not.toHaveBeenCalled();
		expect(apiMock.approveSmartAccountsReconciliation).not.toHaveBeenCalled();
		expect(window.sessionStorage.length).toBe(0);
	});

	it('keeps a cross-tenant, stale, or unauthorized handoff blocked and cannot attest it', async () => {
		apiMock.getSmartAccountsTenantReconciliation.mockRejectedValue(new Error('Request failed with status 404'));
		render(SmartAccountsAccountantReconciliationReview, {
			tenantId: 'tenant-other', batchId: 'batch-1', sourceCompanyId: 'sa-browser-v1-hmb'
		});

		expect(await screen.findByRole('alert')).toHaveTextContent(/status 404/i);
		expect(screen.queryByRole('button', { name: 'Approve current reconciliation evidence' })).not.toBeInTheDocument();
		expect(apiMock.getSmartAccountsTolerancePolicyCandidate).not.toHaveBeenCalled();
		expect(apiMock.approveSmartAccountsTolerancePolicy).not.toHaveBeenCalled();
		expect(apiMock.approveSmartAccountsReconciliation).not.toHaveBeenCalled();
	});

	it('fails closed when an otherwise successful response is bound to another tenant, batch, or source', async () => {
		apiMock.getSmartAccountsTenantReconciliation.mockResolvedValue(evaluation({ tenant_id: 'tenant-other' }));
		render(SmartAccountsAccountantReconciliationReview, {
			tenantId: 'tenant-1', batchId: 'batch-1', sourceCompanyId: 'sa-browser-v1-hmb'
		});

		expect(await screen.findByRole('alert')).toHaveTextContent(/binding mismatch/i);
		expect(screen.queryByRole('button', { name: 'Approve current reconciliation evidence' })).not.toBeInTheDocument();
		expect(apiMock.getSmartAccountsTolerancePolicyCandidate).not.toHaveBeenCalled();
		expect(apiMock.approveSmartAccountsTolerancePolicy).not.toHaveBeenCalled();
		expect(apiMock.approveSmartAccountsReconciliation).not.toHaveBeenCalled();
	});
});
