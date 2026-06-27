import Decimal from 'decimal.js';
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

import {
	flattenUnmatchedTransactions,
	formatIsoDate,
	getLastCompletedFiscalYearEnd,
	getSuggestedCloseDate,
	loadTenantReviewSnapshot,
	monthEndOffset,
	needsPeriodClose,
	parseDateValue,
	toDecimal
} from '$lib/review/workspace';

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

function remediationAction(overrides: Record<string, unknown> = {}) {
	return {
		code: 'review_action',
		severity: 'ACTION',
		scope: 'review',
		owner_role: 'accountant',
		workspace_queue: 'review_queue',
		assignment_key: 'review:action',
		message: 'Review item requires attention.',
		action: 'Review the item.',
		...overrides
	};
}

function migrationSummary(overrides: Record<string, unknown> = {}) {
	return {
		status: 'running',
		confirmed: false,
		resumed: false,
		plan_ready: false,
		validation_ready: false,
		step_count: 0,
		running_step_count: 0,
		succeeded_step_count: 0,
		failed_step_count: 0,
		skipped_step_count: 0,
		planned_step_count: 0,
		resumed_step_count: 0,
		completed_step_count: 0,
		remaining_step_count: 0,
		progress_percent: 0,
		needs_context_count: 0,
		blocked_step_count: 0,
		...overrides
	};
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

	it('formats review dates, decimals, and period-close suggestions', () => {
		const existing = new Decimal('12.30');

		expect(toDecimal(existing)).toBe(existing);
		expect(toDecimal(null).toString()).toBe('0');
		expect(toDecimal('').toString()).toBe('0');
		expect(toDecimal('4.50').toString()).toBe('4.5');
		expect(parseDateValue(null)).toBeNull();
		expect(parseDateValue('not-a-date')).toBeNull();
		expect(formatIsoDate(parseDateValue('2026-06-24') as Date)).toBe('2026-06-24');
		expect(formatIsoDate(monthEndOffset(new Date('2026-01-15T12:00:00Z'), 0))).toBe(
			'2026-01-31'
		);
		expect(getSuggestedCloseDate('2026-01-31', new Date('2026-06-24T12:00:00Z'))).toBe(
			'2026-02-28'
		);
		expect(getSuggestedCloseDate(null, new Date('2026-06-24T12:00:00Z'))).toBe('2026-05-31');
		expect(needsPeriodClose(null, new Date('2026-06-24T12:00:00Z'))).toBe(true);
		expect(needsPeriodClose('2026-05-31', new Date('2026-06-24T12:00:00Z'))).toBe(false);
		expect(needsPeriodClose('2026-04-30', new Date('2026-06-24T12:00:00Z'))).toBe(true);
		expect(getLastCompletedFiscalYearEnd(undefined, new Date('2026-06-13T12:00:00Z'))).toBe(
			'2025-12-31'
		);
		expect(getLastCompletedFiscalYearEnd(13, new Date('2026-06-13T12:00:00Z'))).toBe(
			'2025-11-30'
		);
	});

	it('loads bank and journal evidence fallbacks when document summaries are unavailable', async () => {
		apiMock.listBankAccounts.mockResolvedValue([
			{ id: 'acct-a', name: 'Operating' },
			{ id: 'acct-b', name: 'Savings' },
			{ id: 'acct-c', name: 'Closed' }
		]);
		apiMock.listBankTransactions.mockImplementation(
			(_tenantId: string, accountId: string) => {
				if (accountId === 'acct-c') {
					return Promise.reject(new Error('account unavailable'));
				}
				if (accountId === 'acct-b') {
					return Promise.resolve([
						{
							id: 'tx-b-old',
							transaction_date: '2026-01-15',
							remediation_actions: []
						},
						{
							id: 'tx-b-new',
							transaction_date: '2026-03-15',
							remediation_actions: []
						}
					]);
				}
				return Promise.resolve([
					{
						id: 'tx-a',
						transaction_date: '2026-02-15',
						remediation_actions: []
					}
				]);
			}
		);
		apiMock.listDocumentReviewSummaries.mockRejectedValue(new Error('documents unavailable'));
		apiMock.listJournalEntries.mockResolvedValue([
			{
				id: 'je-old',
				entry_date: '2026-01-31',
				status: 'DRAFT',
				requires_evidence: true
			},
			{
				id: 'je-posted',
				entry_date: '2026-02-15',
				status: 'POSTED',
				requires_evidence: true
			},
			{
				id: 'je-new',
				entry_date: '2026-03-31',
				status: 'DRAFT',
				requires_evidence: true
			}
		]);

		const snapshot = await loadTenantReviewSnapshot({
			id: 'tenant-1',
			settings: { fiscal_year_start_month: 1 }
		} as never);
		const flattened = flattenUnmatchedTransactions(snapshot.bankExceptions);

		expect(snapshot.bankExceptions.map((group) => group.account.id)).toEqual(['acct-b', 'acct-a']);
		expect(flattened.map((item) => item.transaction.id)).toEqual(['tx-b-new', 'tx-a', 'tx-b-old']);
		expect(flattened[0].documentSummary).toEqual(
			expect.objectContaining({
				entity_type: 'bank_transaction',
				entity_id: 'tx-b-new',
				missing_evidence: true
			})
		);
		expect(snapshot.journalEvidence.map((item) => item.entry.id)).toEqual(['je-new', 'je-old']);
		expect(snapshot.journalEvidence[0].documentSummary).toEqual(
			expect.objectContaining({
				entity_type: 'journal_entry',
				entity_id: 'je-new',
				missing_evidence: true
			})
		);
	});

	it('counts failed snapshot loaders without blocking the review workspace', async () => {
		apiMock.getOverdueInvoices.mockRejectedValue(new Error('overdue unavailable'));
		apiMock.listBankAccounts.mockRejectedValue(new Error('accounts unavailable'));
		apiMock.listPeriodCloseEvents.mockRejectedValue(new Error('periods unavailable'));
		apiMock.listJournalEntries.mockRejectedValue(new Error('journals unavailable'));
		apiMock.getDocumentRetentionReview.mockRejectedValue(new Error('retention unavailable'));
		apiMock.listExpenses.mockRejectedValue(new Error('expenses unavailable'));
		apiMock.listPayrollRuns.mockRejectedValue(new Error('payroll unavailable'));
		apiMock.listTSD.mockRejectedValue(new Error('tsd unavailable'));
		apiMock.listKMD.mockRejectedValue(new Error('kmd unavailable'));
		apiMock.listMigrationExecutionRuns.mockRejectedValue(new Error('migration unavailable'));
		apiMock.getYearEndCloseStatus.mockRejectedValue(new Error('close unavailable'));

		const snapshot = await loadTenantReviewSnapshot({
			id: 'tenant-1',
			settings: { fiscal_year_start_month: 1 }
		} as never);

		expect(snapshot.overdueSummary).toBeNull();
		expect(snapshot.bankExceptions).toEqual([]);
		expect(snapshot.periodCloseEvents).toEqual([]);
		expect(snapshot.journalEntries).toEqual([]);
		expect(snapshot.assignmentActions).toEqual([]);
		expect(snapshot.errorCount).toBe(4);
		expect(snapshot.assignmentErrorCount).toBe(7);
	});

	it('normalizes assignment fallback severity, due dates, default links, and duplicates', async () => {
		apiMock.listExpenses.mockResolvedValue([
			{
				id: 'expense-1',
				remediation_actions: [
					remediationAction({
						code: 'missing_queue',
						workspace_queue: ' ',
						assignment_key: 'skip-me'
					}),
					remediationAction({
						code: 'expense_error_defaults',
						severity: 'ERROR',
						workspace_queue: 'expense_review',
						assignment_key: 'expense:shared',
						message: 'Expense needs immediate review.',
						action: 'Open the expense and resolve the blocker.'
					}),
					remediationAction({
						code: 'expense_error_duplicate',
						severity: 'INFO',
						workspace_queue: 'expense_review',
						assignment_key: 'expense:shared',
						message: 'Duplicate expense action.',
						action: 'This duplicate should be hidden.'
					})
				]
			},
			{
				id: 'expense-2',
				remediation_actions: [
					remediationAction({
						code: 'expense_deferred_priority',
						severity: 'WARN',
						priority: 'deferred',
						due_in_days: 9,
						workspace_queue: 'expense_review',
						assignment_key: 'expense:deferred',
						message: 'Expense can be reviewed later.',
						action: 'Schedule the expense review.'
					})
				]
			}
		]);
		apiMock.listPayrollRuns.mockResolvedValue([
			{
				id: 'payroll-1',
				remediation_actions: [
					remediationAction({
						code: 'payroll_info_defaults',
						severity: 'INFO',
						workspace_queue: 'payroll_review',
						assignment_key: 'payroll:info',
						message: 'Payroll information is ready for review.',
						action: 'Review payroll information.'
					})
				]
			}
		]);
		apiMock.listMigrationExecutionRuns.mockResolvedValue([
			{
				id: ' ',
				summary: migrationSummary({ status: 'blocked' }),
				remediation_actions: []
			},
			{
				id: 'run-custom',
				summary: migrationSummary({ status: 'paused' }),
				remediation_actions: [
					remediationAction({
						code: 'migration_custom_defaults',
						severity: 'WARN',
						workspace_queue: 'migration_cutover',
						assignment_key: 'migration:custom',
						message: 'Migration run needs manual follow-up.',
						action: 'Open the migration workbench.'
					})
				]
			},
			{
				id: 'run-no-actions',
				summary: migrationSummary({ status: 'paused' })
			}
		]);

		const snapshot = await loadTenantReviewSnapshot({
			id: 'tenant-1',
			settings: { fiscal_year_start_month: 1 }
		} as never);

		expect(snapshot.assignmentActions).not.toEqual(
			expect.arrayContaining([expect.objectContaining({ code: 'missing_queue' })])
		);
		expect(
			snapshot.assignmentActions.filter((action) => action.assignmentKey === 'expense:shared')
		).toHaveLength(1);
		expect(snapshot.assignmentActions).toEqual(
			expect.arrayContaining([
				expect.objectContaining({
					code: 'expense_error_defaults',
					priority: 'high',
					dueInDays: 1
				}),
				expect.objectContaining({
					code: 'payroll_info_defaults',
					priority: 'low',
					dueInDays: 0
				}),
				expect.objectContaining({
					code: 'migration_custom_defaults',
					priority: 'normal',
					dueInDays: 3,
					uiPath: '/migration'
				}),
				expect.objectContaining({
					code: 'expense_deferred_priority',
					priority: 'deferred',
					dueInDays: 9
				})
			])
		);
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

	it('surfaces failed and confirmation migration saved-run assignments', async () => {
		apiMock.listMigrationExecutionRuns.mockResolvedValue([
			{
				id: 'run-failed-simple',
				tenant_id: 'tenant-1',
				summary: migrationSummary({ status: 'failed' })
			},
			{
				id: 'run-failed-step',
				tenant_id: 'tenant-1',
				summary: migrationSummary({
					status: 'failed',
					failed_step_count: 2,
					active_step_number: 7
				})
			},
			{
				id: 'run-confirm',
				tenant_id: 'tenant-1',
				summary: migrationSummary({
					status: 'needs_confirmation',
					planned_step_count: 0
				})
			},
			{
				id: 'run-blocked-context',
				tenant_id: 'tenant-1',
				summary: migrationSummary({
					status: 'blocked',
					blocked_step_count: 0,
					needs_context_count: 2
				})
			}
		]);

		const snapshot = await loadTenantReviewSnapshot({
			id: 'tenant-1',
			settings: { fiscal_year_start_month: 1 }
		} as never);

		expect(snapshot.assignmentActions).toEqual(
			expect.arrayContaining([
				expect.objectContaining({
					code: 'migration_execution_failed',
					assignmentKey: 'migration:execution-failed:run-failed-simple',
					message: 'Migration run run-failed-simple failed.'
				}),
				expect.objectContaining({
					code: 'migration_execution_failed',
					assignmentKey: 'migration:execution-failed:run-failed-step',
					message: 'Migration run run-failed-step failed at step 7.'
				}),
				expect.objectContaining({
					code: 'migration_execution_needs_confirmation',
					assignmentKey: 'migration:execution-needs-confirmation:run-confirm',
					cliCommand: 'oa migration execute --resume-run-id run-confirm --confirm --json'
				}),
				expect.objectContaining({
					code: 'migration_execution_blocked',
					assignmentKey: 'migration:execution-blocked:run-blocked-context'
				})
			])
		);
	});

	it('counts tax evidence and report remediation errors while preserving fulfilled actions', async () => {
		apiMock.listTSD.mockResolvedValue([
			{
				id: 'tsd-may',
				tenant_id: 'tenant-1',
				period_year: 2026,
				period_month: 5,
				status: 'DRAFT',
				remediation_actions: []
			},
			{
				id: 'tsd-april',
				tenant_id: 'tenant-1',
				period_year: 2026,
				period_month: 4,
				status: 'DRAFT',
				remediation_actions: []
			},
			{
				id: 'tsd-accepted',
				tenant_id: 'tenant-1',
				period_year: 2026,
				period_month: 3,
				status: 'ACCEPTED',
				remediation_actions: []
			},
			{
				id: ' ',
				tenant_id: 'tenant-1',
				period_year: 2026,
				period_month: 2,
				status: 'DRAFT',
				remediation_actions: []
			}
		]);
		apiMock.listKMD.mockResolvedValue([
			{
				id: 'kmd-may',
				tenant_id: 'tenant-1',
				year: 2026,
				month: 5,
				status: 'SUBMITTED',
				remediation_actions: []
			},
			{
				id: 'kmd-feb',
				tenant_id: 'tenant-1',
				year: 2026,
				month: 2,
				status: 'DRAFT',
				remediation_actions: []
			},
			{
				id: 'kmd-may-duplicate',
				tenant_id: 'tenant-1',
				year: 2026,
				month: 5,
				status: 'DRAFT',
				remediation_actions: []
			},
			{
				id: 'kmd-invalid',
				tenant_id: 'tenant-1',
				year: 2026,
				month: 13,
				status: 'DRAFT',
				remediation_actions: []
			}
		]);
		apiMock.evaluateDocumentEvidencePolicy.mockImplementation(
			(_tenantId: string, request: { entity_type: string }) => {
				if (request.entity_type === 'tsd_declaration') {
					return Promise.reject(new Error('tsd evidence unavailable'));
				}
				return Promise.resolve([
					{
						entity_type: 'kmd_declaration',
						entity_id: 'kmd-feb',
						compliant: true
					},
					{
						entity_type: 'kmd_declaration',
						entity_id: 'kmd-may',
						compliant: false,
						remediation_actions: [
							remediationAction({
								code: 'kmd_evidence_missing',
								severity: 'ACTION',
								workspace_queue: 'document_evidence',
								assignment_key: 'document:kmd-evidence',
								entity_type: 'kmd_declaration',
								entity_id: 'kmd-may',
								document_type: 'tax_support',
								message: 'KMD declaration needs tax evidence.',
								action: 'Upload KMD tax support.'
							})
						]
					}
				]);
			}
		);
		apiMock.generateKMDINF.mockImplementation(
			(_tenantId: string, period: { year: number; month: number }) => {
				if (period.month === 5) {
					return Promise.reject(new Error('kmd inf unavailable'));
				}
				return Promise.resolve({});
			}
		);
		apiMock.generateEUVATOSS.mockResolvedValue({
			remediation_actions: [
				remediationAction({
					code: 'oss_report_gap',
					severity: 'WARN',
					workspace_queue: 'tax_reports',
					assignment_key: 'tax-report:oss-gap',
					entity_type: 'tax_report',
					entity_id: 'oss-q2',
					message: 'OSS report has a review gap.',
					action: 'Review OSS report inputs.'
				})
			]
		});

		const snapshot = await loadTenantReviewSnapshot({
			id: 'tenant-1',
			settings: { fiscal_year_start_month: 1 }
		} as never);

		expect(apiMock.evaluateDocumentEvidencePolicy).toHaveBeenCalledWith(
			'tenant-1',
			expect.objectContaining({
				entity_type: 'tsd_declaration',
				entity_ids: ['tsd-may', 'tsd-april']
			})
		);
		expect(apiMock.generateKMDINF.mock.calls.map(([, period]) => period)).toEqual([
			{ year: 2026, month: 5 },
			{ year: 2026, month: 2 }
		]);
		expect(apiMock.generateEUVATOSS.mock.calls.map(([, quarter]) => quarter)).toEqual([
			{ year: 2026, quarter: 2 },
			{ year: 2026, quarter: 1 }
		]);
		expect(snapshot.assignmentErrorCount).toBe(2);
		expect(snapshot.assignmentActions).toEqual(
			expect.arrayContaining([
				expect.objectContaining({
					source: 'documents',
					code: 'kmd_evidence_missing',
					entityType: 'kmd_declaration',
					entityId: 'kmd-may'
				}),
				expect.objectContaining({
					source: 'tax_reports',
					code: 'oss_report_gap',
					entityType: 'tax_report',
					entityId: 'oss-q2'
				})
			])
		);
	});
});
