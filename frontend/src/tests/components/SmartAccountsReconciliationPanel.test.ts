import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import type {
	SmartAccountsBrowserBatchWorkflowSource,
	SmartAccountsReconciliationEvaluation,
	SmartAccountsReconciliationRollup
} from '$lib/api';

const { apiMock } = vi.hoisted(() => ({
	apiMock: {
		getSmartAccountsReconciliation: vi.fn(),
		getSmartAccountsTenantReconciliation: vi.fn(),
		getSmartAccountsReconciliationRollup: vi.fn(),
		getSmartAccountsFullClaimEligibility: vi.fn(),
		evaluateSmartAccountsReconciliation: vi.fn(),
		getSmartAccountsTolerancePolicyCandidate: vi.fn(),
		approveSmartAccountsTolerancePolicy: vi.fn(),
		approveSmartAccountsReconciliation: vi.fn()
	}
}));

vi.mock('$lib/api', async () => {
	const actual = await vi.importActual<typeof import('$lib/api')>('$lib/api');
	return { ...actual, api: apiMock };
});

import SmartAccountsReconciliationPanel from '$lib/components/SmartAccountsReconciliationPanel.svelte';

const digest = 'a'.repeat(64);
const batchId = 'batch-1';

function source(overrides: Partial<SmartAccountsBrowserBatchWorkflowSource> = {}): SmartAccountsBrowserBatchWorkflowSource {
	return {
		batch_id: batchId,
		source_company_id: 'sa-browser-v1-hmb',
		tenant_id: 'tenant-1',
		ordinal: 0,
		phase: 'PREVIEW_READY',
		phase_generation: 4,
		attempt_count: 1,
		package_id: 'package-1',
		preview_id: 'preview-1',
		preview_sha256: digest,
		created_at: '2026-08-28T10:00:00Z',
		updated_at: '2026-08-28T10:00:00Z',
		...overrides
	};
}

function evaluation(overrides: Partial<SmartAccountsReconciliationEvaluation> = {}): SmartAccountsReconciliationEvaluation {
	return {
		evaluation_id: 'evaluation-1',
		batch_id: batchId,
		source_company_id: 'sa-browser-v1-hmb',
		tenant_id: 'tenant-1',
		package_id: 'package-1',
		gl_preview_id: 'preview-1',
		gl_preview_sha256: digest,
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
		evidence_sha256: digest,
		tolerance_sha256: 'b'.repeat(64),
		status: 'EVIDENCE_PENDING',
		created_at: '2026-08-28T10:00:00Z',
		updated_at: '2026-08-28T10:00:00Z',
		...overrides
	};
}

function rollup(overrides: Partial<SmartAccountsReconciliationRollup> = {}): SmartAccountsReconciliationRollup {
	return {
		batch_id: batchId,
		status: 'IN_PROGRESS',
		selected_count: 1,
		pass_count: 0,
		pending_count: 1,
		review_count: 0,
		failure_count: 0,
		...overrides
	};
}

describe('SmartAccountsReconciliationPanel', () => {
	beforeEach(() => {
		vi.resetAllMocks();
		apiMock.getSmartAccountsReconciliation.mockResolvedValue(evaluation());
		apiMock.getSmartAccountsReconciliationRollup.mockResolvedValue(rollup());
		apiMock.getSmartAccountsFullClaimEligibility.mockResolvedValue({
			status: 'NOT_ELIGIBLE',
			full_claim_eligible: false,
			selected_count: 1,
			current_pass_count: 0,
			current_pass_gap_count: 1,
			tombstone_gap_source_count: 0,
			source_coverage_gap_count: 1,
			matrix_blocker_count: 31,
			matrix_filter_contract_gap_count: 22,
			matrix_page_only_gap_count: 7,
			matrix_review_required_count: 2,
			matrix_unconsumed_count: 4,
			matrix_missing_endpoint_count: 2,
			matrix_schema_gap_count: 22,
			matrix_coverage_gap_count: 29,
			blocking_codes: ['matrix_filter_contract_gap', 'selected_sources_not_current_pass']
		});
	});

	afterEach(cleanup);

	it('automatically prepares an owner-safe technical evaluation only after server-confirmed exact GL replay', async () => {
		const ready = evaluation({ status: 'READY_FOR_ACCOUNTANT' });
		apiMock.evaluateSmartAccountsReconciliation.mockResolvedValue({ evaluation: ready, reused: false });
		render(SmartAccountsReconciliationPanel, { batchId, sources: [source()], companyNames: { 'sa-browser-v1-hmb': 'Hold My Beer OÜ' } });

		await screen.findByText(/server verified an exact replay/i);
		await waitFor(() => expect(apiMock.evaluateSmartAccountsReconciliation).toHaveBeenCalledWith(batchId, 'sa-browser-v1-hmb'));
		expect(await screen.findByText('Ready for accountant')).toBeInTheDocument();
		expect(screen.getByText(/never starts a financial apply/i)).toBeInTheDocument();
	});

	it('otherwise leaves one explicit technical Prepare action instead of inferring replay from workflow phase', async () => {
		const pending = evaluation({ gl_state: 'APPLIED' });
		apiMock.getSmartAccountsReconciliation.mockResolvedValue(pending);
		apiMock.evaluateSmartAccountsReconciliation.mockResolvedValue({ evaluation: evaluation({ status: 'READY_FOR_ACCOUNTANT' }), reused: false });
		render(SmartAccountsReconciliationPanel, { batchId, sources: [source()] });

		await screen.findByText('Evidence pending');
		expect(apiMock.evaluateSmartAccountsReconciliation).not.toHaveBeenCalled();
		await fireEvent.click(screen.getByRole('button', { name: 'Recheck technical evidence' }));
		await waitFor(() => expect(apiMock.evaluateSmartAccountsReconciliation).toHaveBeenCalledWith(batchId, 'sa-browser-v1-hmb'));
	});

	it('uses only a server-derived accountant candidate and keeps its digest out of the DOM', async () => {
		const candidateDigest = 'c'.repeat(64);
		apiMock.getSmartAccountsTolerancePolicyCandidate.mockResolvedValue({
			algorithm_version: 'smartaccounts-exact-match-v1',
			label: 'Exact match — zero variance',
			candidate_sha256: candidateDigest
		});
		apiMock.approveSmartAccountsTolerancePolicy.mockResolvedValue({
			policy_id: 'policy-1', algorithm_version: 'smartaccounts-exact-match-v1', tenant_id: 'tenant-1',
			source_company_id: 'sa-browser-v1-hmb', package_id: 'package-1', scope_sha256: digest,
			preview_sha256: digest, tolerance_policy_sha256: candidateDigest, approved_at: '2026-08-28T10:02:00Z'
		});
		render(SmartAccountsReconciliationPanel, { batchId, sources: [source()] });

		await screen.findByText('Evidence pending');
		await fireEvent.click(screen.getByRole('button', { name: 'Load accountant policy candidate' }));
		await screen.findByText(/Exact match — zero variance/);
		expect(screen.queryByText(candidateDigest)).not.toBeInTheDocument();
		await fireEvent.click(screen.getByLabelText(/As the accountant, I confirm this exact policy candidate/));
		await fireEvent.click(screen.getByRole('button', { name: 'Confirm accountant policy' }));

		await waitFor(() => expect(apiMock.approveSmartAccountsTolerancePolicy).toHaveBeenCalledWith('tenant-1', 'sa-browser-v1-hmb', {
			confirmed: true, package_id: 'package-1', preview_id: 'preview-1', expected_candidate_sha256: candidateDigest
		}));
		expect(screen.getByText(/remains separate from financial GL apply/i)).toBeInTheDocument();
	});

	it('surfaces wrong-role and stale-evidence blocks without exposing proof or monetary data', async () => {
		const ready = evaluation({ status: 'READY_FOR_ACCOUNTANT' }) as SmartAccountsReconciliationEvaluation & { raw_proof?: string; total?: string };
		ready.raw_proof = 'source row that must never render';
		ready.total = '€42.00';
		apiMock.getSmartAccountsReconciliation.mockResolvedValue(ready);
		apiMock.getSmartAccountsTolerancePolicyCandidate.mockRejectedValue(new Error('accountant role required'));
		apiMock.approveSmartAccountsReconciliation.mockRejectedValue(new Error('evidence is stale'));
		render(SmartAccountsReconciliationPanel, { batchId, sources: [source()] });

		await screen.findByText('Ready for accountant');
		await fireEvent.click(screen.getByRole('button', { name: 'Load accountant policy candidate' }));
		expect(await screen.findByText(/accountant role required/)).toBeInTheDocument();
		await fireEvent.click(screen.getByLabelText(/As an independent accountant/));
		await fireEvent.click(screen.getByRole('button', { name: 'Approve current reconciliation evidence' }));
		await waitFor(() => expect(apiMock.approveSmartAccountsReconciliation).toHaveBeenCalledWith('tenant-1', 'evaluation-1', {
			confirmed: true, evidence_sha256: digest, tolerance_sha256: 'b'.repeat(64)
		}));
		expect(await screen.findByText(/evidence is stale/)).toBeInTheDocument();
		expect(screen.queryByText('source row that must never render')).not.toBeInTheDocument();
		expect(screen.queryByText('€42.00')).not.toBeInTheDocument();
	});

	it('uses the tenant/batch/source-bound accountant view after an owner-only status read is denied', async () => {
		const ready = evaluation({ status: 'READY_FOR_ACCOUNTANT' });
		apiMock.getSmartAccountsReconciliation.mockRejectedValue(new Error('Request failed with status 403'));
		apiMock.getSmartAccountsTenantReconciliation.mockResolvedValue(ready);
		apiMock.approveSmartAccountsReconciliation.mockResolvedValue(evaluation({ status: 'PASS', accountant_approved_at: '2026-08-28T10:04:00Z' }));
		render(SmartAccountsReconciliationPanel, { batchId, sources: [source()] });

		expect(await screen.findByText(/Accountant-safe evaluation view/)).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Recheck technical evidence' })).not.toBeInTheDocument();
		expect(apiMock.getSmartAccountsTenantReconciliation).toHaveBeenCalledWith('tenant-1', batchId, 'sa-browser-v1-hmb');
		await fireEvent.click(screen.getByLabelText(/As an independent accountant/));
		await fireEvent.click(screen.getByRole('button', { name: 'Approve current reconciliation evidence' }));
		await waitFor(() => expect(apiMock.approveSmartAccountsReconciliation).toHaveBeenCalledWith('tenant-1', 'evaluation-1', {
			confirmed: true, evidence_sha256: digest, tolerance_sha256: 'b'.repeat(64)
		}));
	});

	it('reports selected/all partial failures as not a full sync and preserves every source in the roll-up', async () => {
		const second = source({ source_company_id: 'sa-browser-v1-other', tenant_id: 'tenant-2', ordinal: 1, package_id: 'package-2', preview_id: 'preview-2' });
		apiMock.getSmartAccountsReconciliation.mockImplementation(async (_batch: string, sourceCompanyId: string) => sourceCompanyId === 'sa-browser-v1-hmb'
			? evaluation({ claim_kind: 'partial', expected_coverage_state: 'partial', status: 'PARTIAL_FAILURE', blockers: ['partial_coverage'] })
			: evaluation({ evaluation_id: 'evaluation-2', source_company_id: 'sa-browser-v1-other', tenant_id: 'tenant-2', package_id: 'package-2', gl_preview_id: 'preview-2', status: 'PASS', accountant_approved_at: '2026-08-28T10:05:00Z' })
		);
		apiMock.getSmartAccountsReconciliationRollup.mockResolvedValue(rollup({ status: 'PARTIAL_FAILURE', selected_count: 2, pass_count: 1, pending_count: 0, review_count: 0, failure_count: 1 }));
		render(SmartAccountsReconciliationPanel, {
			batchId,
			sources: [source(), second],
			companyNames: { 'sa-browser-v1-hmb': 'Hold My Beer OÜ', 'sa-browser-v1-other': 'Other OÜ' }
		});

		await screen.findByText('Hold My Beer OÜ');
		await screen.findByText('Other OÜ');
		expect(await screen.findByText(/1\/2 attested, 0 pending, 0 awaiting accountant, 1 partial\/blocked/)).toBeInTheDocument();
		expect(screen.getAllByText(/not a full sync/i).length).toBeGreaterThan(0);
		expect(apiMock.getSmartAccountsReconciliation).toHaveBeenCalledTimes(2);
	});

	it('shows only aggregate fixed full-claim gaps and never calls it from an accountant fallback', async () => {
		const unsafe = {
			status: 'NOT_ELIGIBLE',
			full_claim_eligible: false,
			selected_count: 1,
			current_pass_count: 0,
			current_pass_gap_count: 1,
			tombstone_gap_source_count: 0,
			source_coverage_gap_count: 1,
			matrix_blocker_count: 31,
			matrix_filter_contract_gap_count: 22,
			matrix_page_only_gap_count: 7,
			matrix_review_required_count: 2,
			matrix_unconsumed_count: 4,
			matrix_missing_endpoint_count: 2,
			matrix_schema_gap_count: 22,
			matrix_coverage_gap_count: 29,
			blocking_codes: ['matrix_filter_contract_gap', 'selected_sources_not_current_pass'],
			source_company_id: 'sa-browser-v1-secret',
			proof: 'must not render',
			amount: '99.00'
		};
		apiMock.getSmartAccountsFullClaimEligibility.mockResolvedValue(unsafe);
		render(SmartAccountsReconciliationPanel, { batchId, sources: [source()] });

		expect(await screen.findByText('Full-sync eligibility: blocked')).toBeInTheDocument();
		expect(screen.getByText(/0\/1 selected sources have a current PASS; 31 fixed product-coverage blockers remain/)).toBeInTheDocument();
		expect(screen.getByText('matrix filter contract gap')).toBeInTheDocument();
		expect(screen.queryByText('sa-browser-v1-secret')).not.toBeInTheDocument();
		expect(screen.queryByText('must not render')).not.toBeInTheDocument();
		expect(screen.queryByText('99.00')).not.toBeInTheDocument();

		apiMock.getSmartAccountsReconciliation.mockRejectedValue(new Error('Request failed with status 403'));
		apiMock.getSmartAccountsTenantReconciliation.mockResolvedValue(evaluation({ status: 'READY_FOR_ACCOUNTANT' }));
		apiMock.getSmartAccountsFullClaimEligibility.mockClear();
		render(SmartAccountsReconciliationPanel, { batchId: 'batch-accountant', sources: [source({ batch_id: 'batch-accountant' })] });
		await screen.findAllByText(/Accountant-safe evaluation view/);
		expect(apiMock.getSmartAccountsFullClaimEligibility).not.toHaveBeenCalled();
	});
});
