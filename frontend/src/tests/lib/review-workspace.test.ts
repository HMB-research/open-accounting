import { beforeEach, describe, expect, it, vi } from 'vitest';

const { apiMock } = vi.hoisted(() => ({
	apiMock: {
		getOverdueInvoices: vi.fn(),
		listBankAccounts: vi.fn(),
		listBankTransactions: vi.fn(),
		listDocumentReviewSummaries: vi.fn(),
		listPeriodCloseEvents: vi.fn(),
		listJournalEntries: vi.fn(),
		getDocumentRetentionReview: vi.fn(),
		evaluateDocumentEvidencePolicy: vi.fn(),
		listExpenses: vi.fn(),
		listPayrollRuns: vi.fn(),
		listTSD: vi.fn(),
		listKMD: vi.fn(),
		generateKMDINF: vi.fn(),
		generateEUVATOSS: vi.fn(),
		listMigrationExecutionRuns: vi.fn(),
		getYearEndCloseStatus: vi.fn()
	}
}));

vi.mock('$lib/api', async () => {
	const actual = await vi.importActual<typeof import('$lib/api')>('$lib/api');
	return {
		...actual,
		api: apiMock
	};
});

import { getLastCompletedFiscalYearEnd, loadTenantReviewSnapshot } from '$lib/review/workspace';

function resetWorkspaceApiMocks() {
	apiMock.getOverdueInvoices.mockResolvedValue(null);
	apiMock.listBankAccounts.mockResolvedValue([]);
	apiMock.listBankTransactions.mockResolvedValue([]);
	apiMock.listDocumentReviewSummaries.mockResolvedValue([]);
	apiMock.listPeriodCloseEvents.mockResolvedValue([]);
	apiMock.listJournalEntries.mockResolvedValue([]);
	apiMock.getDocumentRetentionReview.mockResolvedValue({
		as_of_date: '2026-06-15',
		cutoff_date: '2026-07-15',
		total_count: 0,
		expired_count: 0,
		due_soon_count: 0,
		missing_retention_count: 0,
		pending_review_count: 0,
		rejected_count: 0,
		documents: [],
		remediation_actions: []
	});
	apiMock.evaluateDocumentEvidencePolicy.mockResolvedValue([]);
	apiMock.listExpenses.mockResolvedValue([]);
	apiMock.listPayrollRuns.mockResolvedValue([]);
	apiMock.listTSD.mockResolvedValue([]);
	apiMock.listKMD.mockResolvedValue([]);
	apiMock.generateKMDINF.mockResolvedValue({ remediation_actions: [] });
	apiMock.generateEUVATOSS.mockResolvedValue({ remediation_actions: [] });
	apiMock.listMigrationExecutionRuns.mockResolvedValue([]);
	apiMock.getYearEndCloseStatus.mockResolvedValue({
		period_end_date: '2025-12-31',
		remediation_actions: []
	});
}

describe('review workspace helpers', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		resetWorkspaceApiMocks();
	});

	it('returns the previous calendar year end for January fiscal years', () => {
		expect(getLastCompletedFiscalYearEnd(1, new Date('2026-06-13T12:00:00Z'))).toBe('2025-12-31');
	});

	it('returns the previous completed custom fiscal year end', () => {
		expect(getLastCompletedFiscalYearEnd(7, new Date('2026-08-15T12:00:00Z'))).toBe('2026-06-30');
		expect(getLastCompletedFiscalYearEnd(7, new Date('2026-06-13T12:00:00Z'))).toBe('2025-06-30');
	});

	it('surfaces tax declaration evidence policy actions in the assignment queue', async () => {
		apiMock.listTSD.mockResolvedValue([
			{
				id: 'tsd-1',
				tenant_id: 'tenant-1',
				period_year: 2026,
				period_month: 5,
				status: 'DRAFT',
				remediation_actions: []
			},
			{
				id: 'tsd-accepted-1',
				tenant_id: 'tenant-1',
				period_year: 2026,
				period_month: 4,
				status: 'ACCEPTED',
				remediation_actions: []
			}
		]);
		apiMock.listKMD.mockResolvedValue([
			{
				id: 'kmd-1',
				tenant_id: 'tenant-1',
				year: 2026,
				month: 5,
				status: 'SUBMITTED',
				remediation_actions: []
			}
		]);
		apiMock.evaluateDocumentEvidencePolicy.mockImplementation(
			(_tenantId: string, request: { entity_type: string }) => {
				if (request.entity_type === 'tsd_declaration') {
					return Promise.resolve([
						{
							entity_type: 'tsd_declaration',
							entity_id: 'tsd-1',
							compliant: false,
							remediation_actions: [
								{
									code: 'document_evidence_missing',
									severity: 'ACTION',
									scope: 'documents',
									owner_role: 'accountant',
									workspace_queue: 'document_evidence',
									assignment_key: 'document-evidence:tsd-1:tax-support',
									priority: 'high',
									due_in_days: 0,
									message: 'TSD declaration tsd-1 is missing required tax support evidence.',
									action: 'Upload required tax/support evidence.',
									entity_type: 'tsd_declaration',
									entity_id: 'tsd-1',
									document_type: 'tax_support'
								}
							]
						}
					]);
				}
				return Promise.resolve([
					{
						entity_type: 'kmd_declaration',
						entity_id: 'kmd-1',
						compliant: false,
						remediation_actions: [
							{
								code: 'document_evidence_missing',
								severity: 'ACTION',
								scope: 'documents',
								owner_role: 'accountant',
								workspace_queue: 'document_evidence',
								assignment_key: 'document-evidence:kmd-1:tax-support',
								priority: 'high',
								due_in_days: 0,
								message: 'KMD declaration kmd-1 is missing required tax support evidence.',
								action: 'Upload required tax/support evidence.',
								entity_type: 'kmd_declaration',
								entity_id: 'kmd-1',
								document_type: 'tax_support'
							}
						]
					}
				]);
			}
		);

		const snapshot = await loadTenantReviewSnapshot({
			id: 'tenant-1',
			settings: { fiscal_year_start_month: 1 }
		} as never);

		expect(apiMock.evaluateDocumentEvidencePolicy).toHaveBeenCalledWith(
			'tenant-1',
			expect.objectContaining({
				entity_type: 'tsd_declaration',
				entity_ids: ['tsd-1']
			})
		);
		expect(apiMock.evaluateDocumentEvidencePolicy).toHaveBeenCalledWith(
			'tenant-1',
			expect.objectContaining({
				entity_type: 'kmd_declaration',
				entity_ids: ['kmd-1']
			})
		);
		expect(snapshot.assignmentActions).toEqual(
			expect.arrayContaining([
				expect.objectContaining({
					source: 'documents',
					entityType: 'tsd_declaration',
					entityId: 'tsd-1',
					documentType: 'tax_support'
				}),
				expect.objectContaining({
					source: 'documents',
					entityType: 'kmd_declaration',
					entityId: 'kmd-1',
					documentType: 'tax_support'
				})
			])
		);
	});
});
