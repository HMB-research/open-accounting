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

	it('surfaces already-posted carry-forward actions with period context', async () => {
		apiMock.getYearEndCloseStatus.mockResolvedValue({
			period_end_date: '2025-12-31',
			remediation_actions: [
				{
					code: 'carry_forward_already_posted',
					severity: 'INFO',
					scope: 'close',
					owner_role: 'accountant',
					workspace_queue: 'year_end_close',
					assignment_key: 'year-end-close:carry-forward-already-posted:journal-entry:je-1:2025-12-31',
					priority: 'low',
					due_in_days: 0,
					message: 'Carry-forward journal JE-2026-001 already exists.',
					action:
						'Review the posted carry-forward; reverse it only when approved late corrections require a controlled repost.',
					entity_type: 'journal_entry',
					entity_id: 'je-1',
					ui_path: '/journal',
					cli_command:
						'oa close reverse-carry-forward --period-end 2025-12-31 --reason "Approved late correction"'
				}
			]
		});

		const snapshot = await loadTenantReviewSnapshot({
			id: 'tenant-1',
			settings: { fiscal_year_start_month: 1 }
		} as never);

		expect(snapshot.assignmentActions).toEqual(
			expect.arrayContaining([
				expect.objectContaining({
					source: 'close',
					code: 'carry_forward_already_posted',
					periodEndDate: '2025-12-31',
					entityType: 'journal_entry',
					entityId: 'je-1',
					cliCommand: expect.stringContaining('reverse-carry-forward')
				})
			])
		);
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

	it('surfaces running and blocked migration saved-run assignments with deep links', async () => {
		apiMock.listMigrationExecutionRuns.mockResolvedValue([
			{
				id: 'run-running',
				tenant_id: 'tenant-1',
				summary: {
					status: 'running',
					confirmed: true,
					resumed: false,
					plan_ready: true,
					validation_ready: true,
					step_count: 3,
					running_step_count: 1,
					succeeded_step_count: 1,
					failed_step_count: 0,
					skipped_step_count: 0,
					planned_step_count: 1,
					resumed_step_count: 0,
					completed_step_count: 1,
					remaining_step_count: 2,
					progress_percent: 33,
					needs_context_count: 0,
					blocked_step_count: 0,
					active_step_number: 2,
					active_step_kind: 'invoices',
					active_step_file_name: 'invoices.csv',
					active_step_status: 'RUNNING'
				},
				remediation_actions: []
			},
			{
				id: 'run-blocked',
				tenant_id: 'tenant-1',
				summary: {
					status: 'blocked',
					confirmed: false,
					resumed: false,
					plan_ready: false,
					validation_ready: false,
					step_count: 2,
					running_step_count: 0,
					succeeded_step_count: 0,
					failed_step_count: 0,
					skipped_step_count: 0,
					planned_step_count: 0,
					resumed_step_count: 0,
					completed_step_count: 0,
					remaining_step_count: 2,
					progress_percent: 0,
					needs_context_count: 1,
					blocked_step_count: 1
				},
				remediation_actions: []
			},
			{
				id: 'run-succeeded',
				tenant_id: 'tenant-1',
				summary: {
					status: 'succeeded',
					confirmed: true,
					resumed: false,
					plan_ready: true,
					validation_ready: true,
					step_count: 1,
					running_step_count: 0,
					succeeded_step_count: 1,
					failed_step_count: 0,
					skipped_step_count: 0,
					planned_step_count: 0,
					resumed_step_count: 0,
					completed_step_count: 1,
					remaining_step_count: 0,
					progress_percent: 100,
					needs_context_count: 0,
					blocked_step_count: 0
				},
				remediation_actions: []
			}
		]);

		const snapshot = await loadTenantReviewSnapshot({
			id: 'tenant-1',
			settings: { fiscal_year_start_month: 1 }
		} as never);

		expect(apiMock.listMigrationExecutionRuns).toHaveBeenCalledWith('tenant-1', { limit: 25 });
		expect(snapshot.assignmentActions).toEqual(
			expect.arrayContaining([
				expect.objectContaining({
					source: 'migration',
					code: 'migration_execution_running',
					assignmentKey: 'migration:execution-running:run-running',
					priority: 'normal',
					severity: 'ACTION',
					entityType: 'migration_execution_run',
					entityId: 'run-running',
					uiPath: '/migration?run_id=run-running',
					cliCommand: 'oa migration runs get --run-id run-running --json',
					message: 'Migration run run-running is running at step 2 RUNNING invoices invoices.csv.'
				}),
				expect.objectContaining({
					source: 'migration',
					code: 'migration_execution_blocked',
					assignmentKey: 'migration:execution-blocked:run-blocked',
					priority: 'high',
					severity: 'BLOCKER',
					entityType: 'migration_execution_run',
					entityId: 'run-blocked',
					uiPath: '/migration?run_id=run-blocked',
					cliCommand: 'oa migration runs get --run-id run-blocked --json',
					message: 'Migration run run-blocked is blocked before execution.'
				})
			])
		);
		expect(snapshot.assignmentActions).not.toEqual(
			expect.arrayContaining([
				expect.objectContaining({
					entityId: 'run-succeeded'
				})
			])
		);
	});
});
