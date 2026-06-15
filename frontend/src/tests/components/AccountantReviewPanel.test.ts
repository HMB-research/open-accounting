import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import Decimal from 'decimal.js';
import { baseLocale, setLocale } from '$lib/paraglide/runtime.js';
import type { Tenant } from '$lib/api';

const { apiMock } = vi.hoisted(() => ({
	apiMock: {
		getOverdueInvoices: vi.fn(),
		listBankAccounts: vi.fn(),
		listBankTransactions: vi.fn(),
		listDocumentReviewSummaries: vi.fn(),
		reviewBankTransaction: vi.fn(),
		reviewDocument: vi.fn(),
		uploadDocument: vi.fn(),
		updateDocumentRetention: vi.fn(),
		updatePayrollPaymentDate: vi.fn(),
		calculatePayroll: vi.fn(),
		approvePayroll: vi.fn(),
		generateTSD: vi.fn(),
		downloadTSDXml: vi.fn(),
		markTSDSubmitted: vi.fn(),
		markTSDAccepted: vi.fn(),
		executeMigration: vi.fn(),
		generateKMD: vi.fn(),
		generateKMDINF: vi.fn(),
		generateEUVATOSS: vi.fn(),
		downloadKMDXml: vi.fn(),
		markKMDSubmitted: vi.fn(),
		markKMDAccepted: vi.fn(),
		submitExpense: vi.fn(),
		approveExpense: vi.fn(),
		postExpense: vi.fn(),
		closePeriod: vi.fn(),
		createYearEndCarryForward: vi.fn(),
		reverseYearEndCarryForward: vi.fn(),
		sendPaymentReminder: vi.fn(),
		listPeriodCloseEvents: vi.fn(),
		listJournalEntries: vi.fn(),
		getDocumentRetentionReview: vi.fn(),
		evaluateDocumentEvidencePolicy: vi.fn(),
		listExpenses: vi.fn(),
		listPayrollRuns: vi.fn(),
		listTSD: vi.fn(),
		listKMD: vi.fn(),
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

import AccountantReviewPanel from '$lib/components/AccountantReviewPanel.svelte';

function createTenant(overrides: Partial<Tenant> = {}): Tenant {
	return {
		id: 'tenant-1',
		name: 'Acme Corp',
		slug: 'acme',
		schema_name: 'tenant_acme',
		settings: {
			default_currency: 'EUR',
			country_code: 'EE',
			timezone: 'Europe/Tallinn',
			date_format: 'YYYY-MM-DD',
			decimal_sep: '.',
			thousands_sep: ',',
			fiscal_year_start_month: 1,
			period_lock_date: '2026-01-31'
		},
		is_active: true,
		onboarding_completed: true,
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

type PayrollCalculateActionCode = 'payroll_run_calculate' | 'payroll_no_payslips';

function createPayrollRunWithCalculateAssignment(
	code: PayrollCalculateActionCode,
	runId: string,
	period: string,
	message: string
) {
	const [year, month] = period.split('-').map(Number);

	return {
		id: runId,
		tenant_id: 'tenant-1',
		period_year: year,
		period_month: month,
		status: 'DRAFT',
		total_gross: new Decimal(0),
		total_net: new Decimal(0),
		total_employer_cost: new Decimal(0),
		remediation_actions: [
			{
				code,
				severity: 'ACTION',
				scope: 'payroll',
				owner_role: 'accountant',
				workspace_queue: 'payroll_runs',
				assignment_key: `payroll-runs:${code.replaceAll('_', '-')}:payroll-run:${runId}:${period}`,
				priority: 'high',
				due_in_days: 1,
				message,
				action:
					code === 'payroll_no_payslips'
						? 'Recalculate payroll so payslips can be generated and reviewed.'
						: 'Calculate payroll totals and payslips for accountant review.',
				entity_type: 'payroll_run',
				entity_id: runId,
				period,
				ui_path: `/payroll?run_id=${runId}`,
				cli_command: `oa payroll runs calculate --id ${runId}`
			}
		],
		created_at: `${period}-01T00:00:00Z`,
		updated_at: `${period}-01T00:00:00Z`
	};
}

describe('AccountantReviewPanel', () => {
	afterEach(() => {
		cleanup();
	});

	beforeEach(() => {
		setLocale(baseLocale, { reload: false });
		vi.clearAllMocks();
		apiMock.getOverdueInvoices.mockResolvedValue({
			total_overdue: '3200',
			invoice_count: 3,
			contact_count: 2,
			average_days_overdue: 18,
			invoices: [
				{
					id: 'inv-1',
					invoice_number: 'INV-001',
					contact_id: 'contact-1',
					contact_name: 'Northwind',
					contact_email: 'billing@northwind.example',
					issue_date: '2026-01-01',
					due_date: '2026-01-15',
					total: '1200',
					amount_paid: '0',
					outstanding_amount: '1200',
					currency: 'EUR',
					days_overdue: 27,
					reminder_count: 1
				}
			],
			generated_at: '2026-02-11T00:00:00Z'
		});
		apiMock.listBankAccounts.mockResolvedValue([
			{
				id: 'bank-1',
				tenant_id: 'tenant-1',
				name: 'Main bank',
				account_number: 'EE111',
				currency: 'EUR',
				balance: new Decimal(0),
				is_active: true,
				created_at: '2026-01-01T00:00:00Z',
				updated_at: '2026-01-01T00:00:00Z'
			}
		]);
		apiMock.listBankTransactions.mockResolvedValue([
			{
				id: 'tx-1',
				tenant_id: 'tenant-1',
				bank_account_id: 'bank-1',
				transaction_date: '2026-02-08',
				description: 'Unknown transfer',
				amount: new Decimal('-550'),
				currency: 'EUR',
				status: 'UNMATCHED',
				follow_up_status: 'NONE',
				remediation_actions: [
					{
						code: 'bank_transaction_unmatched',
						severity: 'ACTION',
						scope: 'banking',
						owner_role: 'accountant',
						workspace_queue: 'banking_followup',
						assignment_key: 'banking-followup:bank-transaction-unmatched:bank-transaction:tx-1',
						priority: 'high',
						due_in_days: 1,
						message: 'Bank transaction needs matching.',
						action: 'Match the transaction or mark the needed follow-up.',
						ui_path: '/banking',
						cli_command:
							'oa banking transactions review --id tx-1 --follow-up-status READY_TO_MATCH'
					}
				],
				created_at: '2026-02-08T00:00:00Z'
			}
		]);
		apiMock.listDocumentReviewSummaries.mockResolvedValue([
			{
				entity_type: 'bank_transaction',
				entity_id: 'tx-1',
				total_count: 1,
				pending_review_count: 1,
				reviewed_count: 0,
				approved_count: 0,
				rejected_count: 0,
				missing_evidence: false,
				has_pending_review: true
			}
		]);
		apiMock.reviewDocument.mockResolvedValue({
			id: 'doc-1',
			tenant_id: 'tenant-1',
			entity_type: 'bank_transaction',
			entity_id: 'tx-1',
			document_type: 'reconciliation_evidence',
			file_name: 'bank-evidence.pdf',
			file_size: 2048,
			mime_type: 'application/pdf',
			storage_path: 'tenant-1/doc-1.pdf',
			review_status: 'APPROVED',
			retention_until: '2028-12-31T00:00:00Z',
			created_at: '2026-02-01T00:00:00Z'
		});
		apiMock.listPeriodCloseEvents.mockResolvedValue([
			{
				id: 'evt-1',
				tenant_id: 'tenant-1',
				action: 'close',
				close_kind: 'month_end',
				period_end_date: '2026-01-31',
				lock_date_after: '2026-01-31',
				performed_by: 'user-1',
				created_at: '2026-02-02T09:00:00Z'
			}
		]);
		apiMock.listJournalEntries.mockResolvedValue([
			{
				id: 'je-1',
				tenant_id: 'tenant-1',
				entry_number: 'JE-2026-001',
				entry_date: '2026-02-10',
				description: 'Month-end accrual',
				status: 'DRAFT',
				lines: [
					{
						id: 'line-1',
						account_id: '4000',
						debit_amount: new Decimal(900),
						credit_amount: new Decimal(0),
						currency: 'EUR',
						exchange_rate: new Decimal(1),
						base_debit: new Decimal(900),
						base_credit: new Decimal(0)
					}
				],
				created_at: '2026-02-10T00:00:00Z',
				created_by: 'user-1'
			}
		]);
		apiMock.getDocumentRetentionReview.mockResolvedValue({
			as_of_date: '2026-02-11',
			cutoff_date: '2026-03-13',
			total_count: 1,
			expired_count: 0,
			due_soon_count: 1,
			missing_retention_count: 0,
			pending_review_count: 0,
			rejected_count: 0,
			documents: [],
			remediation_actions: [
				{
					code: 'document_review_pending',
					severity: 'ACTION',
					scope: 'documents',
					owner_role: 'accountant',
					workspace_queue: 'document_review',
					assignment_key:
						'document-review:document-review-pending:bank-transaction:tx-1:reconciliation-evidence:doc-1',
					priority: 'high',
					due_in_days: 1,
					message: 'Document bank-evidence.pdf is still pending review.',
					action: 'Review the attachment and approve, reject, or mark it reviewed.',
					entity_type: 'bank_transaction',
					entity_id: 'tx-1',
					document_id: 'doc-1',
					document_type: 'reconciliation_evidence',
					file_name: 'bank-evidence.pdf',
					ui_path: '/documents?entity_type=bank_transaction&entity_id=tx-1&document_id=doc-1',
					cli_command: 'oa documents review --id doc-1 --status approved'
				}
			]
		});
		apiMock.listExpenses.mockResolvedValue([
			{
				id: 'expense-1',
				tenant_id: 'tenant-1',
				expense_number: 'EXP-001',
				expense_date: '2026-02-09',
				merchant: 'Taxi Co',
				expense_account_id: 'expense-account',
				payment_account_id: 'cash-account',
				amount: new Decimal(35),
				currency: 'EUR',
				exchange_rate: new Decimal(1),
				base_amount: new Decimal(35),
				requires_receipt: true,
				status: 'SUBMITTED',
				remediation_actions: [
					{
						code: 'expense_receipt_approval_required',
						severity: 'ACTION',
						scope: 'expenses',
						owner_role: 'accountant',
						workspace_queue: 'expense_claims',
						assignment_key:
							'expense-claims:expense-receipt-approval-required:expense:expense-1:EXP-001:SUBMITTED',
						priority: 'high',
						due_in_days: 1,
						message: 'Expense EXP-001 is submitted and receipt-backed.',
						action: 'Confirm a linked receipt exists and is approved before approving the expense.',
						entity_type: 'expense',
						entity_id: 'expense-1',
						expense_number: 'EXP-001',
						status: 'SUBMITTED',
						ui_path: '/expenses?expense_id=expense-1',
						cli_command:
							'oa documents review-queue --entity-type expense --document-type receipt --status PENDING'
					}
				],
				created_at: '2026-02-09T00:00:00Z',
				created_by: 'user-1',
				updated_at: '2026-02-09T00:00:00Z'
			}
		]);
		apiMock.listPayrollRuns.mockResolvedValue([
			{
				id: 'payroll-1',
				tenant_id: 'tenant-1',
				period_year: 2026,
				period_month: 2,
				status: 'CALCULATED',
				total_gross: new Decimal(4200),
				total_net: new Decimal(3260),
				total_employer_cost: new Decimal(5628),
				remediation_actions: [
					{
						code: 'payroll_run_approve',
						severity: 'ACTION',
						scope: 'payroll',
						owner_role: 'accountant',
						workspace_queue: 'payroll_runs',
						assignment_key: 'payroll-runs:payroll-run-approve:payroll-run:payroll-1:2026-02',
						priority: 'high',
						due_in_days: 1,
						message: 'Payroll run 2026-02 is calculated and awaiting approval.',
						action: 'Review payroll totals and payslips, then approve the run.',
						entity_type: 'payroll_run',
						entity_id: 'payroll-1',
						period: '2026-02',
						ui_path: '/payroll?run_id=payroll-1',
						cli_command: 'oa payroll runs approve --id payroll-1'
					}
				],
				created_at: '2026-02-01T00:00:00Z',
				updated_at: '2026-02-01T00:00:00Z'
			}
		]);
		apiMock.listTSD.mockResolvedValue([]);
		apiMock.listKMD.mockResolvedValue([]);
		apiMock.listMigrationExecutionRuns.mockResolvedValue([]);
		apiMock.getYearEndCloseStatus.mockResolvedValue({
			period_end_date: '2025-12-31',
			fiscal_year_label: '2025',
			fiscal_year_start_date: '2025-01-01',
			fiscal_year_end_date: '2025-12-31',
			carry_forward_date: '2026-01-01',
			is_fiscal_year_end: true,
			period_closed: false,
			has_profit_and_loss_activity: true,
			carry_forward_needed: true,
			carry_forward_ready: false,
			has_retained_earnings_account: true,
			net_income: new Decimal(1200),
			remediation_actions: [
				{
					code: 'fiscal_year_not_closed',
					severity: 'BLOCKER',
					scope: 'close',
					owner_role: 'accountant',
					workspace_queue: 'year_end_close',
					assignment_key: 'year-end-close:fiscal-year-not-closed:close:2025-12-31',
					priority: 'high',
					due_in_days: 1,
					message: 'Fiscal year ending 2025-12-31 is not closed.',
					action: 'Close the fiscal year with reviewer sign-off before posting carry-forward.',
					ui_path: '/settings/company#period-history',
					cli_command:
						'oa close period --period-end 2025-12-31 --reviewer-sign-off --note "Fiscal-year close"'
				}
			]
		});
		apiMock.reviewBankTransaction.mockResolvedValue({
			id: 'tx-1',
			tenant_id: 'tenant-1',
			bank_account_id: 'bank-1',
			transaction_date: '2026-02-08',
			description: 'Unknown transfer',
			amount: new Decimal('-550'),
			currency: 'EUR',
			status: 'UNMATCHED',
			follow_up_status: 'EVIDENCE_REQUIRED',
			review_note: 'Request signed receipt',
			reviewed_by: 'user-1',
			reviewed_at: '2026-02-09T09:00:00Z',
			created_at: '2026-02-08T00:00:00Z'
		});
		apiMock.updateDocumentRetention.mockResolvedValue({
			id: 'doc-retention-1',
			tenant_id: 'tenant-1',
			entity_type: 'expense',
			entity_id: 'expense-1',
			document_type: 'receipt',
			file_name: 'receipt.pdf',
			content_type: 'application/pdf',
			file_size: 1200,
			retention_until: '2027-03-01T00:00:00Z',
			review_status: 'APPROVED',
			uploaded_by: 'user-1',
			created_at: '2026-02-01T00:00:00Z'
		});
		apiMock.uploadDocument.mockResolvedValue({
			id: 'doc-uploaded',
			tenant_id: 'tenant-1',
			entity_type: 'bank_transaction',
			entity_id: 'tx-evidence',
			document_type: 'reconciliation_evidence',
			file_name: 'evidence.pdf',
			content_type: 'application/pdf',
			file_size: 1200,
			review_status: 'PENDING',
			uploaded_by: 'user-1',
			created_at: '2026-02-01T00:00:00Z'
		});
		apiMock.updatePayrollPaymentDate.mockResolvedValue({
			id: 'payroll-1',
			tenant_id: 'tenant-1',
			period_year: 2026,
			period_month: 2,
			status: 'CALCULATED',
			payment_date: '2026-02-28T00:00:00Z',
			total_gross: new Decimal(4200),
			total_net: new Decimal(3260),
			total_employer_cost: new Decimal(5628),
			remediation_actions: [],
			created_at: '2026-02-01T00:00:00Z',
			updated_at: '2026-02-01T00:00:00Z'
		});
		apiMock.calculatePayroll.mockResolvedValue({
			id: 'payroll-calculated-1',
			tenant_id: 'tenant-1',
			period_year: 2026,
			period_month: 2,
			status: 'CALCULATED',
			total_gross: new Decimal(4200),
			total_net: new Decimal(3260),
			total_employer_cost: new Decimal(5628),
			remediation_actions: [],
			created_at: '2026-02-01T00:00:00Z',
			updated_at: '2026-02-01T00:00:00Z'
		});
		apiMock.approvePayroll.mockResolvedValue({ status: 'approved' });
		apiMock.generateTSD.mockResolvedValue({ id: 'tsd-1' });
		apiMock.downloadTSDXml.mockResolvedValue(undefined);
		apiMock.markTSDSubmitted.mockResolvedValue({ status: 'submitted' });
		apiMock.markTSDAccepted.mockResolvedValue({ status: 'accepted' });
		apiMock.executeMigration.mockResolvedValue({ id: 'run-executed' });
		apiMock.generateKMD.mockResolvedValue({ id: 'kmd-1' });
		apiMock.generateKMDINF.mockResolvedValue({
			tenant_id: 'tenant-1',
			year: 2026,
			month: 3,
			threshold: new Decimal(1000),
			generated_at: '2026-03-31T00:00:00Z',
			summary: [],
			rows: [],
			remediation_actions: []
		});
		apiMock.generateEUVATOSS.mockResolvedValue({
			tenant_id: 'tenant-1',
			year: 2026,
			quarter: 1,
			period_start: '2026-01-01T00:00:00Z',
			period_end: '2026-03-31T23:59:59Z',
			scheme: 'UNION',
			currency: 'EUR',
			include_b2b: false,
			generated_at: '2026-03-31T00:00:00Z',
			summary: [],
			rows: [],
			taxable_amount: new Decimal(0),
			vat_amount: new Decimal(0),
			total_amount: new Decimal(0),
			invoice_count: 0,
			line_count: 0,
			remediation_actions: []
		});
		apiMock.downloadKMDXml.mockResolvedValue(undefined);
		apiMock.markKMDSubmitted.mockResolvedValue({ status: 'submitted' });
		apiMock.markKMDAccepted.mockResolvedValue({ status: 'accepted' });
		apiMock.submitExpense.mockResolvedValue({
			id: 'expense-draft-1',
			status: 'SUBMITTED'
		});
		apiMock.approveExpense.mockResolvedValue({
			id: 'expense-submitted-1',
			status: 'APPROVED'
		});
		apiMock.postExpense.mockResolvedValue({
			id: 'expense-approved-1',
			status: 'POSTED'
		});
		apiMock.closePeriod.mockResolvedValue({
			tenant: createTenant({
				settings: {
					...createTenant().settings,
					period_lock_date: '2025-12-31'
				}
			}),
			event: {
				id: 'evt-year-close',
				tenant_id: 'tenant-1',
				action: 'close',
				close_kind: 'year_end',
				period_end_date: '2025-12-31',
				lock_date_after: '2025-12-31',
				reviewer_sign_off: true,
				performed_by: 'user-1',
				created_at: '2026-01-02T09:00:00Z'
			}
		});
		apiMock.createYearEndCarryForward.mockResolvedValue({
			journal_entry: { id: 'je-carry-forward', entry_number: 'JE-2026-001' },
			status: { period_end_date: '2025-12-31' }
		});
		apiMock.reverseYearEndCarryForward.mockResolvedValue({
			reversal_journal_entry: { id: 'je-reversal', entry_number: 'JE-2026-002' },
			status: { period_end_date: '2025-12-31' }
		});
		apiMock.sendPaymentReminder.mockResolvedValue({
			invoice_id: 'inv-1',
			invoice_number: 'INV-001',
			success: true,
			message: 'Sent',
			reminder_id: 'rem-1'
		});
		apiMock.evaluateDocumentEvidencePolicy.mockResolvedValue([]);
	});

	it('reverses already-posted carry-forward assignments from the workspace', async () => {
		apiMock.listBankTransactions.mockResolvedValue([]);
		apiMock.listDocumentReviewSummaries.mockResolvedValue([]);
		apiMock.getDocumentRetentionReview.mockResolvedValue({
			as_of_date: '2026-02-11',
			cutoff_date: '2026-03-13',
			total_count: 0,
			expired_count: 0,
			due_soon_count: 0,
			missing_retention_count: 0,
			pending_review_count: 0,
			rejected_count: 0,
			documents: [],
			remediation_actions: []
		});
		apiMock.listExpenses.mockResolvedValue([]);
		apiMock.listPayrollRuns.mockResolvedValue([]);
		apiMock.listTSD.mockResolvedValue([]);
		apiMock.listKMD.mockResolvedValue([]);
		apiMock.getYearEndCloseStatus.mockResolvedValue({
			period_end_date: '2025-12-31',
			fiscal_year_label: '2025',
			fiscal_year_start_date: '2025-01-01',
			fiscal_year_end_date: '2025-12-31',
			carry_forward_date: '2026-01-01',
			is_fiscal_year_end: true,
			period_closed: true,
			has_profit_and_loss_activity: true,
			carry_forward_needed: false,
			carry_forward_ready: false,
			has_retained_earnings_account: true,
			net_income: new Decimal(1200),
			existing_carry_forward: {
				id: 'je-carry-forward',
				entry_number: 'JE-2026-001',
				entry_date: '2026-01-01',
				description: 'Year-end carry-forward',
				status: 'POSTED'
			},
			remediation_actions: [
				{
					code: 'carry_forward_already_posted',
					severity: 'INFO',
					scope: 'close',
					owner_role: 'accountant',
					workspace_queue: 'year_end_close',
					assignment_key:
						'year-end-close:carry-forward-already-posted:journal-entry:je-carry-forward:2025-12-31',
					priority: 'low',
					due_in_days: 0,
					message: 'Carry-forward journal JE-2026-001 already exists.',
					action:
						'Review the posted carry-forward; reverse it only when approved late corrections require a controlled repost.',
					entity_type: 'journal_entry',
					entity_id: 'je-carry-forward',
					ui_path: '/journal',
					cli_command:
						'oa close reverse-carry-forward --period-end 2025-12-31 --reason "Approved late correction"'
				}
			]
		});

		render(AccountantReviewPanel, {
			tenant: createTenant()
		});

		const row = (await screen.findByText('Carry-forward journal JE-2026-001 already exists.')).closest(
			'li'
		) as HTMLElement;
		const reverseButton = within(row).getByRole('button', { name: 'Reverse carry-forward' });
		expect(reverseButton).toBeDisabled();

		await fireEvent.input(within(row).getByLabelText('Reversal reason'), {
			target: { value: 'Late supplier accrual' }
		});

		await waitFor(() => {
			expect(reverseButton).not.toBeDisabled();
		});

		await fireEvent.click(reverseButton);

		await waitFor(() => {
			expect(apiMock.reverseYearEndCarryForward).toHaveBeenCalledWith('tenant-1', {
				period_end_date: '2025-12-31',
				reason: 'Late supplier accrual'
			});
			expect(screen.getByText('Carry-forward reversed from workspace.')).toBeInTheDocument();
		});
	});

	it('completes close and carry-forward assignments from the workspace', async () => {
		apiMock.listBankTransactions.mockResolvedValue([]);
		apiMock.listDocumentReviewSummaries.mockResolvedValue([]);
		apiMock.getDocumentRetentionReview.mockResolvedValue({
			as_of_date: '2026-02-11',
			cutoff_date: '2026-03-13',
			total_count: 0,
			expired_count: 0,
			due_soon_count: 0,
			missing_retention_count: 0,
			pending_review_count: 0,
			rejected_count: 0,
			documents: [],
			remediation_actions: []
		});
		apiMock.listExpenses.mockResolvedValue([]);
		apiMock.listPayrollRuns.mockResolvedValue([]);
		apiMock.listTSD.mockResolvedValue([]);
		apiMock.listKMD.mockResolvedValue([]);
		apiMock.getYearEndCloseStatus.mockResolvedValue({
			period_end_date: '2025-12-31',
			fiscal_year_label: '2025',
			fiscal_year_start_date: '2025-01-01',
			fiscal_year_end_date: '2025-12-31',
			carry_forward_date: '2026-01-01',
			is_fiscal_year_end: true,
			period_closed: false,
			has_profit_and_loss_activity: true,
			carry_forward_needed: true,
			carry_forward_ready: true,
			has_retained_earnings_account: true,
			net_income: new Decimal(1200),
			remediation_actions: [
				{
					code: 'fiscal_year_not_closed',
					severity: 'BLOCKER',
					scope: 'close',
					owner_role: 'accountant',
					workspace_queue: 'year_end_close',
					assignment_key: 'year-end-close:fiscal-year-not-closed:close:2025-12-31',
					priority: 'high',
					due_in_days: 1,
					message: 'Fiscal year ending 2025-12-31 is not closed.',
					action: 'Close the fiscal year with reviewer sign-off before posting carry-forward.',
					ui_path: '/settings/company#period-history',
					cli_command:
						'oa close period --period-end 2025-12-31 --reviewer-sign-off --note "Fiscal-year close"'
				},
				{
					code: 'ready_to_post_carry_forward',
					severity: 'ACTION',
					scope: 'close',
					owner_role: 'accountant',
					workspace_queue: 'year_end_close',
					assignment_key: 'year-end-close:ready-to-post-carry-forward:close:2025-12-31',
					priority: 'high',
					due_in_days: 1,
					message: 'Year-end close is ready for carry-forward posting.',
					action: 'Post the retained-earnings carry-forward journal.',
					ui_path: '/settings/company#year-end',
					cli_command: 'oa close carry-forward --period-end 2025-12-31'
				}
			]
		});

		render(AccountantReviewPanel, {
			tenant: createTenant({
				settings: {
					...createTenant().settings,
					inventory_valuation_method: 'fifo'
				}
			})
		});

		await waitFor(() => {
			expect(screen.getByRole('button', { name: 'Close fiscal year' })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: 'Post carry-forward' })).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Close fiscal year' }));

		await waitFor(() => {
			expect(apiMock.closePeriod).toHaveBeenCalledWith('tenant-1', {
				period_end_date: '2025-12-31',
				note: 'Fiscal-year close from accountant workspace',
				reviewer_sign_off: true,
				inventory_valuation_method: 'fifo'
			});
			expect(screen.getByText('Fiscal year closed from workspace.')).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Post carry-forward' }));

		await waitFor(() => {
			expect(apiMock.createYearEndCarryForward).toHaveBeenCalledWith('tenant-1', {
				period_end_date: '2025-12-31',
				inventory_valuation_method: 'fifo'
			});
			expect(screen.getByText('Carry-forward posted from workspace.')).toBeInTheDocument();
		});
	});

	it('loads and renders the accountant review queues', async () => {
		render(AccountantReviewPanel, {
			tenant: createTenant()
		});

		await waitFor(() => {
			expect(apiMock.getOverdueInvoices).toHaveBeenCalledWith('tenant-1');
		});

		await waitFor(() => {
			expect(screen.getByText('Outstanding balance')).toBeInTheDocument();
		});

		expect(screen.getByText('Accountant review')).toBeInTheDocument();
		expect(
			screen.getByText(
				'Review the items that still need an accountant decision before the next close or filing window.'
			)
		).toBeInTheDocument();
		expect(screen.getByText('INV-001')).toBeInTheDocument();
		expect(screen.getByText('Unknown transfer')).toBeInTheDocument();
		expect(screen.getAllByText('Evidence pending review').length).toBeGreaterThan(0);
		expect(screen.getByText('Month-end accrual')).toBeInTheDocument();
		expect(screen.getByText('Assignment queue')).toBeInTheDocument();
		expect(screen.getByText('Fiscal year ending 2025-12-31 is not closed.')).toBeInTheDocument();
		expect(screen.getByText('Bank transaction needs matching.')).toBeInTheDocument();
		expect(
			screen.getByText('Document bank-evidence.pdf is still pending review.')
		).toBeInTheDocument();
		expect(
			screen.getByText('Expense EXP-001 is submitted and receipt-backed.')
		).toBeInTheDocument();
		expect(
			screen.getByText('Payroll run 2026-02 is calculated and awaiting approval.')
		).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Approve document' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Approve payroll' })).toBeInTheDocument();
		expect(screen.getByText(/oa documents review-queue --entity-type expense/)).toBeInTheDocument();
		expect(screen.getByText(/oa payroll runs approve --id payroll-1/)).toBeInTheDocument();
		expect(screen.getByText(/oa close period --period-end 2025-12-31/)).toBeInTheDocument();
		expect(
			screen
				.getAllByRole('link', { name: 'Open action' })
				.some(
					(link) => link.getAttribute('href') === '/settings/company?tenant=tenant-1#period-history'
				)
		).toBe(true);
		expect(
			screen
				.getAllByRole('link', { name: 'Open action' })
				.some(
					(link) => link.getAttribute('href') === '/expenses?expense_id=expense-1&tenant=tenant-1'
				)
		).toBe(true);
		expect(screen.getAllByText('Closed').length).toBeGreaterThan(0);
		expect(screen.getByRole('link', { name: 'Open reminders' })).toHaveAttribute(
			'href',
			'/invoices/reminders?tenant=tenant-1'
		);
		expect(apiMock.listBankTransactions).toHaveBeenCalledWith('tenant-1', 'bank-1', {
			status: 'UNMATCHED'
		});
		expect(apiMock.listDocumentReviewSummaries).toHaveBeenCalledWith(
			'tenant-1',
			'bank_transaction',
			['tx-1']
		);
		expect(apiMock.getDocumentRetentionReview).toHaveBeenCalledWith('tenant-1', {
			horizon_days: 30,
			include_missing: true
		});
		expect(apiMock.listExpenses).toHaveBeenCalledWith('tenant-1', {
			limit: 100
		});
		expect(apiMock.listPayrollRuns).toHaveBeenCalledWith('tenant-1');
		expect(apiMock.listTSD).toHaveBeenCalledWith('tenant-1');
		expect(apiMock.listKMD).toHaveBeenCalledWith('tenant-1');

		await fireEvent.click(screen.getByRole('button', { name: 'Approve document' }));

		await waitFor(() => {
			expect(apiMock.reviewDocument).toHaveBeenCalledWith('tenant-1', 'doc-1', {
				review_status: 'APPROVED'
			});
			expect(screen.getByText('Document approved from workspace.')).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Approve payroll' }));

		await waitFor(() => {
			expect(apiMock.approvePayroll).toHaveBeenCalledWith('tenant-1', 'payroll-1');
			expect(screen.getByText('Payroll run approved from workspace.')).toBeInTheDocument();
		});
	});

	it('sets document retention assignment rows from the workspace', async () => {
		apiMock.getOverdueInvoices.mockResolvedValue({
			total_overdue: '0',
			invoice_count: 0,
			contact_count: 0,
			average_days_overdue: 0,
			invoices: [],
			generated_at: '2026-02-11T00:00:00Z'
		});
		apiMock.listBankAccounts.mockResolvedValue([]);
		apiMock.listDocumentReviewSummaries.mockResolvedValue([]);
		apiMock.listPeriodCloseEvents.mockResolvedValue([]);
		apiMock.listJournalEntries.mockResolvedValue([]);
		apiMock.getDocumentRetentionReview.mockResolvedValue({
			as_of_date: '2026-02-11',
			cutoff_date: '2026-03-13',
			total_count: 1,
			expired_count: 0,
			due_soon_count: 1,
			missing_retention_count: 0,
			pending_review_count: 0,
			rejected_count: 0,
			documents: [],
			remediation_actions: [
				{
					code: 'document_retention_due_soon',
					severity: 'INFO',
					scope: 'documents',
					owner_role: 'accountant',
					workspace_queue: 'document_review',
					assignment_key: 'document-review:document-retention-due-soon:expense:expense-1:receipt:doc-retention-1',
					priority: 'medium',
					due_in_days: 15,
					message: 'Document receipt.pdf retention is due soon.',
					action: 'Review the document before the retention date and either extend retention or complete the disposal workflow.',
					entity_type: 'expense',
					entity_id: 'expense-1',
					document_id: 'doc-retention-1',
					document_type: 'receipt',
					file_name: 'receipt.pdf',
					due_date: '2026-03-01',
					days_until_retention: 15,
					ui_path: '/documents?entity_type=expense&entity_id=expense-1&document_id=doc-retention-1',
					cli_command: 'oa documents retention-set --id doc-retention-1 --retention-until <YYYY-MM-DD>'
				}
			]
		});
		apiMock.listExpenses.mockResolvedValue([]);
		apiMock.listPayrollRuns.mockResolvedValue([]);
		apiMock.listTSD.mockResolvedValue([]);
		apiMock.listKMD.mockResolvedValue([]);
		apiMock.listMigrationExecutionRuns.mockResolvedValue([]);
		apiMock.getYearEndCloseStatus.mockResolvedValue({
			period_end_date: '2025-12-31',
			fiscal_year_label: '2025',
			fiscal_year_start_date: '2025-01-01',
			fiscal_year_end_date: '2025-12-31',
			carry_forward_date: '2026-01-01',
			is_fiscal_year_end: true,
			period_closed: true,
			has_profit_and_loss_activity: false,
			carry_forward_needed: false,
			carry_forward_ready: false,
			has_retained_earnings_account: true,
			net_income: new Decimal(0),
			remediation_actions: []
		});

		render(AccountantReviewPanel, {
			tenant: createTenant()
		});

		await waitFor(() => {
			expect(screen.getByText('Document receipt.pdf retention is due soon.')).toBeInTheDocument();
		});

		const retentionInput = screen.getByLabelText('Retention date') as HTMLInputElement;
		expect(retentionInput.value).toBe('2027-03-01');

		await fireEvent.input(retentionInput, { target: { value: '2028-04-30' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Set retention' }));

		await waitFor(() => {
			expect(apiMock.updateDocumentRetention).toHaveBeenCalledWith('tenant-1', 'doc-retention-1', {
				retention_until: '2028-04-30'
			});
			expect(screen.getByText('Document retention set from workspace.')).toBeInTheDocument();
		});
	});

	it('uploads evidence and replacements from assignment rows', async () => {
		apiMock.getOverdueInvoices.mockResolvedValue({
			total_overdue: '0',
			invoice_count: 0,
			contact_count: 0,
			average_days_overdue: 0,
			invoices: [],
			generated_at: '2026-02-11T00:00:00Z'
		});
		apiMock.listBankAccounts.mockResolvedValue([
			{
				id: 'bank-1',
				tenant_id: 'tenant-1',
				name: 'Main bank',
				account_number: 'EE111',
				currency: 'EUR',
				balance: new Decimal(0),
				is_active: true,
				created_at: '2026-01-01T00:00:00Z',
				updated_at: '2026-01-01T00:00:00Z'
			}
		]);
		apiMock.listBankTransactions.mockResolvedValue([
			{
				id: 'tx-evidence',
				tenant_id: 'tenant-1',
				bank_account_id: 'bank-1',
				transaction_date: '2026-02-08',
				description: 'Evidence required transfer',
				amount: new Decimal('-550'),
				currency: 'EUR',
				status: 'UNMATCHED',
				follow_up_status: 'EVIDENCE_REQUIRED',
				remediation_actions: [
					{
						code: 'bank_evidence_required',
						severity: 'ACTION',
						scope: 'banking',
						owner_role: 'accountant',
						workspace_queue: 'banking_followup',
						assignment_key: 'banking-followup:bank-evidence-required:bank-transaction:tx-evidence:UNMATCHED:EVIDENCE_REQUIRED',
						priority: 'high',
						due_in_days: 1,
						message: 'Bank transaction tx-evidence requires approved reconciliation evidence.',
						action: 'Upload and approve reconciliation evidence before completing the reconciliation.',
						entity_type: 'bank_transaction',
						entity_id: 'tx-evidence',
						ui_path: '/banking?transaction_id=tx-evidence',
						cli_command:
							'oa documents upload --entity-type bank_transaction --entity-id tx-evidence --document-type reconciliation_evidence --file <file>'
					}
				],
				created_at: '2026-02-08T00:00:00Z'
			}
		]);
		apiMock.listDocumentReviewSummaries.mockResolvedValue([
			{
				entity_type: 'bank_transaction',
				entity_id: 'tx-evidence',
				total_count: 0,
				pending_review_count: 0,
				reviewed_count: 0,
				approved_count: 0,
				rejected_count: 0,
				missing_evidence: true,
				has_pending_review: false,
				has_rejected: false
			}
		]);
		apiMock.listPeriodCloseEvents.mockResolvedValue([]);
		apiMock.listJournalEntries.mockResolvedValue([]);
		apiMock.getDocumentRetentionReview.mockResolvedValue({
			as_of_date: '2026-02-11',
			cutoff_date: '2026-03-13',
			total_count: 1,
			expired_count: 0,
			due_soon_count: 0,
			missing_retention_count: 0,
			pending_review_count: 0,
			rejected_count: 1,
			documents: [],
			remediation_actions: [
				{
					code: 'document_evidence_missing',
					severity: 'ACTION',
					scope: 'documents',
					owner_role: 'accountant',
					workspace_queue: 'document_evidence',
					assignment_key: 'document-evidence:document-evidence-missing:payment:pay-1:receipt',
					priority: 'high',
					due_in_days: 0,
					message: 'Payment pay-1 is missing required receipt evidence.',
					action: 'Upload required evidence before continuing the protected workflow.',
					entity_type: 'payment',
					entity_id: 'pay-1',
					document_type: 'receipt',
					ui_path: '/documents?entity_type=payment&entity_id=pay-1',
					cli_command: 'oa documents upload --entity-type payment --entity-id pay-1 --document-type receipt --file <file>'
				},
				{
					code: 'document_review_rejected',
					severity: 'ACTION',
					scope: 'documents',
					owner_role: 'accountant',
					workspace_queue: 'document_review',
					assignment_key: 'document-review:document-review-rejected:expense:expense-1:receipt:doc-rejected-1',
					priority: 'high',
					due_in_days: 0,
					message: 'Document rejected-receipt.pdf was rejected and needs replacement or correction.',
					action: 'Upload corrected evidence or approve the existing document after the rejection has been resolved.',
					entity_type: 'expense',
					entity_id: 'expense-1',
					document_id: 'doc-rejected-1',
					document_type: 'receipt',
					file_name: 'rejected-receipt.pdf',
					ui_path: '/documents?entity_type=expense&entity_id=expense-1&document_id=doc-rejected-1',
					cli_command: 'oa documents upload --entity-type expense --entity-id expense-1 --document-type receipt --file <replacement-file>'
				},
				{
					code: 'document_evidence_missing',
					severity: 'ACTION',
					scope: 'documents',
					owner_role: 'accountant',
					workspace_queue: 'document_evidence',
					assignment_key: 'document-evidence:document-evidence-missing:tsd-declaration:tsd-1:tax-support',
					priority: 'high',
					due_in_days: 0,
					message: 'TSD declaration tsd-1 is missing required tax support evidence.',
					action: 'Upload required tax/support evidence before submitting or accepting the declaration.',
					entity_type: 'tsd_declaration',
					entity_id: 'tsd-1',
					document_type: 'tax_support',
					ui_path: '/documents?entity_type=tsd_declaration&entity_id=tsd-1',
					cli_command: 'oa documents upload --entity-type tsd_declaration --entity-id tsd-1 --document-type tax_support --file <file>'
				},
				{
					code: 'document_evidence_policy_violation',
					severity: 'ACTION',
					scope: 'documents',
					owner_role: 'accountant',
					workspace_queue: 'document_evidence',
					assignment_key: 'document-evidence:document-evidence-policy-violation:kmd-declaration:kmd-1:tax-support',
					priority: 'high',
					due_in_days: 0,
					message: 'KMD declaration kmd-1 needs one more approved tax support document.',
					action: 'Upload another matching tax/support document before accepting the declaration.',
					entity_type: 'kmd_declaration',
					entity_id: 'kmd-1',
					document_type: 'tax_support',
					ui_path: '/documents?entity_type=kmd_declaration&entity_id=kmd-1',
					cli_command: 'oa documents upload --entity-type kmd_declaration --entity-id kmd-1 --document-type tax_support --file <file>'
				},
				{
					code: 'document_evidence_unapproved',
					severity: 'ACTION',
					scope: 'documents',
					owner_role: 'accountant',
					workspace_queue: 'document_review',
					assignment_key: 'document-review:document-evidence-unapproved:payment:pay-2:receipt:doc-pending-receipt',
					priority: 'high',
					due_in_days: 1,
					message: 'Payment pay-2 has matching evidence, but not enough approved documents.',
					action: 'Review and approve enough matching evidence documents to satisfy the workflow policy.',
					entity_type: 'payment',
					entity_id: 'pay-2',
					document_id: 'doc-pending-receipt',
					document_type: 'receipt',
					file_name: 'receipt-draft.pdf',
					ui_path: '/documents?entity_type=payment&entity_id=pay-2&document_id=doc-pending-receipt',
					cli_command: 'oa documents review --id doc-pending-receipt --status approved'
				}
			]
		});
		apiMock.listExpenses.mockResolvedValue([]);
		apiMock.listPayrollRuns.mockResolvedValue([]);
		apiMock.listTSD.mockResolvedValue([]);
		apiMock.listKMD.mockResolvedValue([]);
		apiMock.listMigrationExecutionRuns.mockResolvedValue([]);
		apiMock.getYearEndCloseStatus.mockResolvedValue({
			period_end_date: '2025-12-31',
			fiscal_year_label: '2025',
			fiscal_year_start_date: '2025-01-01',
			fiscal_year_end_date: '2025-12-31',
			carry_forward_date: '2026-01-01',
			is_fiscal_year_end: true,
			period_closed: true,
			has_profit_and_loss_activity: false,
			carry_forward_needed: false,
			carry_forward_ready: false,
			has_retained_earnings_account: true,
			net_income: new Decimal(0),
			remediation_actions: []
		});

		render(AccountantReviewPanel, {
			tenant: createTenant()
		});

		await waitFor(() => {
			expect(
				screen.getByText('Bank transaction tx-evidence requires approved reconciliation evidence.')
			).toBeInTheDocument();
			expect(screen.getByText('Payment pay-1 is missing required receipt evidence.')).toBeInTheDocument();
			expect(
				screen.getByText('Document rejected-receipt.pdf was rejected and needs replacement or correction.')
			).toBeInTheDocument();
			expect(screen.getByText('TSD declaration tsd-1 is missing required tax support evidence.')).toBeInTheDocument();
			expect(
				screen.getByText('KMD declaration kmd-1 needs one more approved tax support document.')
			).toBeInTheDocument();
			expect(
				screen.getByText('Payment pay-2 has matching evidence, but not enough approved documents.')
			).toBeInTheDocument();
		});

		const uploadFromAssignmentRow = async (message: string, buttonLabel: string, file: File) => {
			const row = screen.getByText(message).closest('.review-list-item-assignment');
			expect(row).not.toBeNull();
			const input = row!.querySelector('input[type="file"]');
			expect(input).not.toBeNull();
			await fireEvent.change(input!, {
				target: { files: [file] }
			});
			const button = Array.from(row!.querySelectorAll('button')).find(
				(candidate) => candidate.textContent?.trim() === buttonLabel
			);
			expect(button).toBeDefined();
			await fireEvent.click(button!);
		};

		const evidenceFile = new File(['evidence'], 'evidence.pdf', { type: 'application/pdf' });
		await uploadFromAssignmentRow(
			'Bank transaction tx-evidence requires approved reconciliation evidence.',
			'Upload evidence',
			evidenceFile
		);

		await waitFor(() => {
			expect(apiMock.uploadDocument).toHaveBeenCalledWith('tenant-1', 'bank_transaction', 'tx-evidence', evidenceFile, {
				document_type: 'reconciliation_evidence',
				notes: 'Uploaded from accountant workspace assignment'
			});
			expect(screen.getByText('Evidence uploaded from workspace.')).toBeInTheDocument();
		});

		const missingEvidenceFile = new File(['receipt'], 'receipt.pdf', { type: 'application/pdf' });
		await uploadFromAssignmentRow(
			'Payment pay-1 is missing required receipt evidence.',
			'Upload evidence',
			missingEvidenceFile
		);

		await waitFor(() => {
			expect(apiMock.uploadDocument).toHaveBeenCalledWith('tenant-1', 'payment', 'pay-1', missingEvidenceFile, {
				document_type: 'receipt',
				notes: 'Uploaded from accountant workspace assignment'
			});
			expect(screen.getByText('Evidence uploaded from workspace.')).toBeInTheDocument();
		});

		const tsdEvidenceFile = new File(['tsd'], 'tsd-tax-support.pdf', { type: 'application/pdf' });
		await uploadFromAssignmentRow(
			'TSD declaration tsd-1 is missing required tax support evidence.',
			'Upload evidence',
			tsdEvidenceFile
		);

		await waitFor(() => {
			expect(apiMock.uploadDocument).toHaveBeenCalledWith('tenant-1', 'tsd_declaration', 'tsd-1', tsdEvidenceFile, {
				document_type: 'tax_support',
				notes: 'Uploaded from accountant workspace assignment'
			});
			expect(screen.getByText('Evidence uploaded from workspace.')).toBeInTheDocument();
		});

		const kmdEvidenceFile = new File(['kmd'], 'kmd-tax-support.pdf', { type: 'application/pdf' });
		await uploadFromAssignmentRow(
			'KMD declaration kmd-1 needs one more approved tax support document.',
			'Upload evidence',
			kmdEvidenceFile
		);

		await waitFor(() => {
			expect(apiMock.uploadDocument).toHaveBeenCalledWith('tenant-1', 'kmd_declaration', 'kmd-1', kmdEvidenceFile, {
				document_type: 'tax_support',
				notes: 'Uploaded from accountant workspace assignment'
			});
			expect(screen.getByText('Evidence uploaded from workspace.')).toBeInTheDocument();
		});

		const replacementFile = new File(['replacement'], 'replacement.pdf', { type: 'application/pdf' });
		await uploadFromAssignmentRow(
			'Document rejected-receipt.pdf was rejected and needs replacement or correction.',
			'Upload replacement',
			replacementFile
		);

		await waitFor(() => {
			expect(apiMock.uploadDocument).toHaveBeenCalledWith('tenant-1', 'expense', 'expense-1', replacementFile, {
				document_type: 'receipt',
				notes: 'Replacement uploaded from accountant workspace assignment',
				replaces_document_id: 'doc-rejected-1',
				replacement_note: 'Replacement uploaded from accountant workspace assignment'
			});
			expect(
				screen.getByText('Payment pay-2 has matching evidence, but not enough approved documents.')
			).toBeInTheDocument();
		});

		const unapprovedEvidenceRow = screen
			.getByText('Payment pay-2 has matching evidence, but not enough approved documents.')
			.closest('.review-list-item-assignment');
		expect(unapprovedEvidenceRow).not.toBeNull();
		const approveButton = Array.from(unapprovedEvidenceRow!.querySelectorAll('button')).find(
			(candidate) => candidate.textContent?.trim() === 'Approve document'
		);
		expect(approveButton).toBeDefined();
		await fireEvent.click(approveButton!);

		await waitFor(() => {
			expect(apiMock.reviewDocument).toHaveBeenCalledWith('tenant-1', 'doc-pending-receipt', {
				review_status: 'APPROVED'
			});
			expect(screen.getByText('Document approved from workspace.')).toBeInTheDocument();
		});
	});

	it('shows empty-state guidance when no review items are pending', async () => {
		apiMock.getOverdueInvoices.mockResolvedValue({
			total_overdue: '0',
			invoice_count: 0,
			contact_count: 0,
			average_days_overdue: 0,
			invoices: [],
			generated_at: '2026-02-11T00:00:00Z'
		});
		apiMock.listBankTransactions.mockResolvedValue([]);
		apiMock.listDocumentReviewSummaries.mockResolvedValue([]);
		apiMock.listPeriodCloseEvents.mockResolvedValue([]);
		apiMock.listJournalEntries.mockResolvedValue([]);
		apiMock.getDocumentRetentionReview.mockResolvedValue({
			as_of_date: '2026-02-11',
			cutoff_date: '2026-03-13',
			total_count: 0,
			expired_count: 0,
			due_soon_count: 0,
			missing_retention_count: 0,
			pending_review_count: 0,
			rejected_count: 0,
			documents: [],
			remediation_actions: []
		});
		apiMock.listExpenses.mockResolvedValue([]);
		apiMock.listPayrollRuns.mockResolvedValue([]);
		apiMock.listTSD.mockResolvedValue([]);
		apiMock.listKMD.mockResolvedValue([]);
		apiMock.getYearEndCloseStatus.mockResolvedValue({
			period_end_date: '2025-12-31',
			fiscal_year_label: '2025',
			fiscal_year_start_date: '2025-01-01',
			fiscal_year_end_date: '2025-12-31',
			carry_forward_date: '2026-01-01',
			is_fiscal_year_end: true,
			period_closed: true,
			has_profit_and_loss_activity: false,
			carry_forward_needed: false,
			carry_forward_ready: false,
			has_retained_earnings_account: true,
			net_income: new Decimal(0),
			remediation_actions: []
		});

		render(AccountantReviewPanel, {
			tenant: createTenant({
				settings: { ...createTenant().settings, period_lock_date: null }
			})
		});

		await waitFor(() => {
			expect(screen.getByText('No overdue invoices need attention right now.')).toBeInTheDocument();
		});

		expect(
			screen.getByText('No unmatched bank transactions are waiting for review.')
		).toBeInTheDocument();
		expect(
			screen.getByText('No assignment-ready remediation actions are waiting right now.')
		).toBeInTheDocument();
		expect(
			screen.getByText('No close or reopen actions have been recorded yet.')
		).toBeInTheDocument();
		expect(screen.getByText('No recent journal entries to review yet.')).toBeInTheDocument();
		expect(screen.getByText('No periods locked yet')).toBeInTheDocument();
	});

	it('surfaces migration cutover run assignments from the workspace', async () => {
		apiMock.listBankTransactions.mockResolvedValue([]);
		apiMock.listDocumentReviewSummaries.mockResolvedValue([]);
		apiMock.getDocumentRetentionReview.mockResolvedValue({
			as_of_date: '2026-02-11',
			cutoff_date: '2026-03-13',
			total_count: 0,
			expired_count: 0,
			due_soon_count: 0,
			missing_retention_count: 0,
			pending_review_count: 0,
			rejected_count: 0,
			documents: [],
			remediation_actions: []
		});
		apiMock.listExpenses.mockResolvedValue([]);
		apiMock.listPayrollRuns.mockResolvedValue([]);
		apiMock.listTSD.mockResolvedValue([]);
		apiMock.listKMD.mockResolvedValue([]);
		apiMock.getYearEndCloseStatus.mockResolvedValue({
			period_end_date: '2025-12-31',
			fiscal_year_label: '2025',
			fiscal_year_start_date: '2025-01-01',
			fiscal_year_end_date: '2025-12-31',
			carry_forward_date: '2026-01-01',
			is_fiscal_year_end: true,
			period_closed: true,
			has_profit_and_loss_activity: false,
			carry_forward_needed: false,
			carry_forward_ready: false,
			has_retained_earnings_account: true,
			net_income: new Decimal(0),
			remediation_actions: []
		});
		apiMock.listMigrationExecutionRuns.mockResolvedValue([
			{
				id: 'run-failed',
				tenant_id: 'tenant-1',
				summary: {
					status: 'failed',
					confirmed: true,
					resumed: false,
					plan_ready: true,
					validation_ready: true,
					step_count: 2,
					running_step_count: 0,
					succeeded_step_count: 1,
					failed_step_count: 1,
					skipped_step_count: 0,
					planned_step_count: 0,
					resumed_step_count: 0,
					completed_step_count: 2,
					remaining_step_count: 0,
					progress_percent: 100,
					needs_context_count: 0,
					blocked_step_count: 0,
					active_step_number: 2,
					active_step_kind: 'contacts',
					active_step_file_name: 'contacts.csv',
					active_step_status: 'FAILED'
				},
				remediation_actions: []
			},
			{
				id: 'run-ready',
				tenant_id: 'tenant-1',
				summary: {
					status: 'needs_confirmation',
					confirmed: false,
					resumed: false,
					plan_ready: true,
					validation_ready: true,
					step_count: 2,
					running_step_count: 0,
					succeeded_step_count: 0,
					failed_step_count: 0,
					skipped_step_count: 0,
					planned_step_count: 2,
					resumed_step_count: 0,
					completed_step_count: 0,
					remaining_step_count: 2,
					progress_percent: 0,
					needs_context_count: 0,
					blocked_step_count: 0
				},
				remediation_actions: []
			},
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
			}
		]);

		render(AccountantReviewPanel, { tenant: createTenant() });

		await waitFor(() =>
			expect(apiMock.listMigrationExecutionRuns).toHaveBeenCalledWith('tenant-1', { limit: 25 })
		);
		expect(
			await screen.findByText('Migration run run-failed failed at step 2 FAILED contacts contacts.csv.')
		).toBeInTheDocument();
		expect(screen.getAllByText('Migration · migration_cutover · ACTION')).toHaveLength(3);
		expect(screen.getByText('Migration · migration_cutover · BLOCKER')).toBeInTheDocument();
		expect(
			screen.getByText('CLI: oa migration runs get --run-id run-failed --json')
		).toBeInTheDocument();
		expect(
			screen.getByText('Migration run run-ready is ready for accountant confirmation.')
		).toBeInTheDocument();
		expect(
			screen.getByText('CLI: oa migration execute --resume-run-id run-ready --confirm --json')
		).toBeInTheDocument();
		expect(
			screen.getByText('Migration run run-running is running at step 2 RUNNING invoices invoices.csv.')
		).toBeInTheDocument();
		expect(
			screen.getByText('CLI: oa migration runs get --run-id run-running --json')
		).toBeInTheDocument();
		expect(screen.getByText('Migration run run-blocked is blocked before execution.')).toBeInTheDocument();
		expect(screen.getByText('CLI: oa migration runs get --run-id run-blocked --json')).toBeInTheDocument();
		const actionLinks = screen.getAllByRole('link', { name: 'Open action' });
		expect(
			actionLinks.some(
				(link) => link.getAttribute('href') === '/migration?run_id=run-failed&tenant=tenant-1'
			)
		).toBe(true);
		expect(
			actionLinks.some(
				(link) => link.getAttribute('href') === '/migration?run_id=run-ready&tenant=tenant-1'
			)
		).toBe(true);
		expect(
			actionLinks.some(
				(link) => link.getAttribute('href') === '/migration?run_id=run-running&tenant=tenant-1'
			)
		).toBe(true);
		expect(
			actionLinks.some(
				(link) => link.getAttribute('href') === '/migration?run_id=run-blocked&tenant=tenant-1'
			)
		).toBe(true);
		expect(screen.getAllByRole('button', { name: 'Execute migration' })).toHaveLength(1);

		await fireEvent.click(screen.getByRole('button', { name: 'Execute migration' }));

		await waitFor(() => {
			expect(apiMock.executeMigration).toHaveBeenCalledWith('tenant-1', {
				files: [],
				confirm: true,
				resume_from_run_id: 'run-ready'
			});
			expect(screen.getByText('Migration executed from workspace.')).toBeInTheDocument();
		});
	});

	it.each([
		[
			'payroll_run_calculate',
			'payroll-calculate-1',
			'2026-05',
			'Payroll run 2026-05 is ready for calculation.'
		],
		[
			'payroll_no_payslips',
			'payroll-no-payslips-1',
			'2026-06',
			'Payroll run 2026-06 has no payslips and needs recalculation.'
		]
	] as const)('calculates %s assignment rows from the workspace', async (code, runId, period, message) => {
		apiMock.listPayrollRuns.mockResolvedValue([
			createPayrollRunWithCalculateAssignment(code, runId, period, message)
		]);

		render(AccountantReviewPanel, {
			tenant: createTenant()
		});

		await waitFor(() => {
			expect(screen.getByText(message)).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Calculate payroll' }));

		await waitFor(() => {
			expect(apiMock.calculatePayroll).toHaveBeenCalledWith('tenant-1', runId);
			expect(screen.getByText('Payroll run calculated from workspace.')).toBeInTheDocument();
		});
	});

	it('shows assignment errors when payroll calculation fails from the workspace', async () => {
		const runId = 'payroll-no-payslips-error-1';
		const message = 'Payroll run 2026-07 has no payslips and needs recalculation.';
		apiMock.listPayrollRuns.mockResolvedValue([
			createPayrollRunWithCalculateAssignment('payroll_no_payslips', runId, '2026-07', message)
		]);
		apiMock.calculatePayroll.mockRejectedValueOnce(new Error('Payroll calculation failed'));

		render(AccountantReviewPanel, {
			tenant: createTenant()
		});

		await waitFor(() => {
			expect(screen.getByText(message)).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Calculate payroll' }));

		await waitFor(() => {
			expect(apiMock.calculatePayroll).toHaveBeenCalledWith('tenant-1', runId);
			expect(screen.getByText('Payroll calculation failed')).toBeInTheDocument();
		});
	});

	it('generates TSD from approved payroll assignment rows', async () => {
		apiMock.listPayrollRuns.mockResolvedValue([
			{
				id: 'payroll-approved-1',
				tenant_id: 'tenant-1',
				period_year: 2026,
				period_month: 3,
				status: 'APPROVED',
				total_gross: new Decimal(5100),
				total_net: new Decimal(3978),
				total_employer_cost: new Decimal(6834),
				remediation_actions: [
					{
						code: 'payroll_generate_tsd',
						severity: 'ACTION',
						scope: 'payroll',
						owner_role: 'accountant',
						workspace_queue: 'payroll_runs',
						assignment_key:
							'payroll-runs:payroll-generate-tsd:payroll-run:payroll-approved-1:2026-03',
						priority: 'high',
						due_in_days: 1,
						message: 'Payroll run 2026-03 is approved and ready for TSD generation.',
						action: 'Generate the TSD declaration, export it, and file it through e-MTA.',
						entity_type: 'payroll_run',
						entity_id: 'payroll-approved-1',
						period: '2026-03',
						ui_path: '/payroll?run_id=payroll-approved-1',
						cli_command: 'oa tsd generate --run-id payroll-approved-1'
					}
				],
				created_at: '2026-03-01T00:00:00Z',
				updated_at: '2026-03-01T00:00:00Z'
			}
		]);

		render(AccountantReviewPanel, {
			tenant: createTenant()
		});

		await waitFor(() => {
			expect(
				screen.getByText('Payroll run 2026-03 is approved and ready for TSD generation.')
			).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Generate TSD' }));

		await waitFor(() => {
			expect(apiMock.generateTSD).toHaveBeenCalledWith('tenant-1', 'payroll-approved-1');
			expect(screen.getByText('TSD generated from workspace.')).toBeInTheDocument();
		});
	});

	it('generates TSD from paid payroll follow-up assignment rows', async () => {
		apiMock.listBankTransactions.mockResolvedValue([]);
		apiMock.getDocumentRetentionReview.mockResolvedValue({
			as_of_date: '2026-04-11',
			cutoff_date: '2026-05-11',
			total_count: 0,
			expired_count: 0,
			due_soon_count: 0,
			missing_retention_count: 0,
			pending_review_count: 0,
			rejected_count: 0,
			documents: [],
			remediation_actions: []
		});
		apiMock.listExpenses.mockResolvedValue([]);
		apiMock.listTSD.mockResolvedValue([]);
		apiMock.listKMD.mockResolvedValue([]);
		apiMock.getYearEndCloseStatus.mockResolvedValue({
			period_end_date: '2025-12-31',
			fiscal_year_label: '2025',
			fiscal_year_start_date: '2025-01-01',
			fiscal_year_end_date: '2025-12-31',
			carry_forward_date: '2026-01-01',
			is_fiscal_year_end: true,
			period_closed: true,
			has_profit_and_loss_activity: false,
			carry_forward_needed: false,
			carry_forward_ready: false,
			has_retained_earnings_account: true,
			net_income: new Decimal(0),
			remediation_actions: []
		});
		apiMock.listPayrollRuns.mockResolvedValue([
			{
				id: 'payroll-paid-1',
				tenant_id: 'tenant-1',
				period_year: 2026,
				period_month: 4,
				status: 'PAID',
				total_gross: new Decimal(5400),
				total_net: new Decimal(4212),
				total_employer_cost: new Decimal(7236),
				payment_date: '2026-04-30',
				remediation_actions: [
					{
						code: 'payroll_paid_tsd_followup',
						severity: 'ACTION',
						scope: 'payroll',
						owner_role: 'accountant',
						workspace_queue: 'payroll_runs',
						assignment_key:
							'payroll-runs:payroll-paid-tsd-followup:payroll-run:payroll-paid-1:2026-04',
						priority: 'high',
						due_in_days: 1,
						message: 'Payroll run 2026-04 is paid and still needs declaration follow-up.',
						action: 'Reconcile salary payments, generate the TSD declaration, and retain payment evidence.',
						entity_type: 'payroll_run',
						entity_id: 'payroll-paid-1',
						period: '2026-04',
						ui_path: '/payroll?run_id=payroll-paid-1',
						cli_command: 'oa tsd generate --run-id payroll-paid-1'
					}
				],
				created_at: '2026-04-01T00:00:00Z',
				updated_at: '2026-04-30T00:00:00Z'
			}
		]);

		render(AccountantReviewPanel, {
			tenant: createTenant()
		});

		await waitFor(() => {
			expect(
				screen.getByText('Payroll run 2026-04 is paid and still needs declaration follow-up.')
			).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Generate TSD' }));

		await waitFor(() => {
			expect(apiMock.generateTSD).toHaveBeenCalledWith('tenant-1', 'payroll-paid-1');
			expect(screen.getByText('TSD generated from workspace.')).toBeInTheDocument();
		});
	});

	it('exports TSD XML from declared payroll archive assignment rows', async () => {
		apiMock.listBankTransactions.mockResolvedValue([]);
		apiMock.getDocumentRetentionReview.mockResolvedValue({
			as_of_date: '2026-04-11',
			cutoff_date: '2026-05-11',
			total_count: 0,
			expired_count: 0,
			due_soon_count: 0,
			missing_retention_count: 0,
			pending_review_count: 0,
			rejected_count: 0,
			documents: [],
			remediation_actions: []
		});
		apiMock.listExpenses.mockResolvedValue([]);
		apiMock.listTSD.mockResolvedValue([]);
		apiMock.listKMD.mockResolvedValue([]);
		apiMock.getYearEndCloseStatus.mockResolvedValue({
			period_end_date: '2025-12-31',
			fiscal_year_label: '2025',
			fiscal_year_start_date: '2025-01-01',
			fiscal_year_end_date: '2025-12-31',
			carry_forward_date: '2026-01-01',
			is_fiscal_year_end: true,
			period_closed: true,
			has_profit_and_loss_activity: false,
			carry_forward_needed: false,
			carry_forward_ready: false,
			has_retained_earnings_account: true,
			net_income: new Decimal(0),
			remediation_actions: []
		});
		apiMock.listPayrollRuns.mockResolvedValue([
			{
				id: 'payroll-declared-1',
				tenant_id: 'tenant-1',
				period_year: 2026,
				period_month: 4,
				status: 'DECLARED',
				total_gross: new Decimal(5400),
				total_net: new Decimal(4212),
				total_employer_cost: new Decimal(7236),
				payment_date: '2026-04-30',
				remediation_actions: [
					{
						code: 'payroll_declared_archive',
						severity: 'INFO',
						scope: 'payroll',
						owner_role: 'accountant',
						workspace_queue: 'payroll_runs',
						assignment_key:
							'payroll-runs:payroll-declared-archive:payroll-run:payroll-declared-1:2026-04',
						priority: 'low',
						due_in_days: 14,
						message: 'Payroll run 2026-04 is declared.',
						action:
							'Archive the accepted TSD export, payslips, and salary payment evidence with the monthly close support.',
						entity_type: 'payroll_run',
						entity_id: 'payroll-declared-1',
						period: '2026-04',
						ui_path: '/payroll?run_id=payroll-declared-1',
						cli_command: 'oa tsd export-xml --year 2026 --month 4 --output ./tsd-2026-04.xml'
					}
				],
				created_at: '2026-04-01T00:00:00Z',
				updated_at: '2026-05-10T00:00:00Z'
			}
		]);

		render(AccountantReviewPanel, {
			tenant: createTenant()
		});

		await waitFor(() => {
			expect(screen.getByText('Payroll run 2026-04 is declared.')).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Export TSD XML' }));

		await waitFor(() => {
			expect(apiMock.downloadTSDXml).toHaveBeenCalledWith('tenant-1', 2026, 4);
			expect(screen.getByText('TSD XML exported from workspace.')).toBeInTheDocument();
		});
	});

	it('exports TSD XML from declaration assignment rows', async () => {
		apiMock.listBankTransactions.mockResolvedValue([]);
		apiMock.getDocumentRetentionReview.mockResolvedValue({
			as_of_date: '2026-03-11',
			cutoff_date: '2026-04-10',
			total_count: 0,
			expired_count: 0,
			due_soon_count: 0,
			missing_retention_count: 0,
			pending_review_count: 0,
			rejected_count: 0,
			documents: [],
			remediation_actions: []
		});
		apiMock.listExpenses.mockResolvedValue([]);
		apiMock.listPayrollRuns.mockResolvedValue([]);
		apiMock.listKMD.mockResolvedValue([]);
		apiMock.getYearEndCloseStatus.mockResolvedValue({
			period_end_date: '2025-12-31',
			fiscal_year_label: '2025',
			fiscal_year_start_date: '2025-01-01',
			fiscal_year_end_date: '2025-12-31',
			carry_forward_date: '2026-01-01',
			is_fiscal_year_end: true,
			period_closed: true,
			has_profit_and_loss_activity: false,
			carry_forward_needed: false,
			carry_forward_ready: false,
			has_retained_earnings_account: true,
			net_income: new Decimal(0),
			remediation_actions: []
		});
		apiMock.listTSD.mockResolvedValue([
			{
				id: 'tsd-ready-1',
				tenant_id: 'tenant-1',
				period_year: 2026,
				period_month: 3,
				total_payments: new Decimal(5100),
				total_income_tax: new Decimal(900),
				total_social_tax: new Decimal(1683),
				total_unemployment_employer: new Decimal(41),
				total_unemployment_employee: new Decimal(82),
				total_funded_pension: new Decimal(102),
				status: 'DRAFT',
				remediation_actions: [
					{
						code: 'tsd_export_and_submit',
						severity: 'ACTION',
						scope: 'tax',
						owner_role: 'accountant',
						workspace_queue: 'tsd_declarations',
						assignment_key:
							'tsd-declarations:tsd-export-and-submit:tsd-declaration:tsd-ready-1:2026-03',
						priority: 'high',
						due_in_days: 1,
						message: 'TSD 2026-03 is ready for export and submission review.',
						action:
							'Review declaration totals, export XML or CSV, submit through e-MTA, and mark the declaration submitted with the e-MTA reference.',
						entity_type: 'tsd_declaration',
						entity_id: 'tsd-ready-1',
						period: '2026-03',
						ui_path: '/tsd?year=2026&month=3',
						cli_command: 'oa tsd export-xml --year 2026 --month 3 --output ./tsd-2026-03.xml'
					}
				],
				created_at: '2026-03-31T00:00:00Z',
				updated_at: '2026-03-31T00:00:00Z'
			}
		]);

		render(AccountantReviewPanel, {
			tenant: createTenant()
		});

		await waitFor(() => {
			expect(
				screen.getByText('TSD 2026-03 is ready for export and submission review.')
			).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Export TSD XML' }));

		await waitFor(() => {
			expect(apiMock.downloadTSDXml).toHaveBeenCalledWith('tenant-1', 2026, 3);
			expect(screen.getByText('TSD XML exported from workspace.')).toBeInTheDocument();
		});

		await fireEvent.input(screen.getByLabelText('e-MTA reference'), {
			target: { value: 'EMTA-2026-03' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Mark TSD submitted' }));

		await waitFor(() => {
			expect(apiMock.markTSDSubmitted).toHaveBeenCalledWith('tenant-1', 2026, 3, 'EMTA-2026-03');
			expect(screen.getByText('TSD marked submitted from workspace.')).toBeInTheDocument();
		});
	});

	it('marks submitted TSD assignment rows accepted from the workspace', async () => {
		apiMock.listBankTransactions.mockResolvedValue([]);
		apiMock.getDocumentRetentionReview.mockResolvedValue({
			as_of_date: '2026-03-11',
			cutoff_date: '2026-04-10',
			total_count: 0,
			expired_count: 0,
			due_soon_count: 0,
			missing_retention_count: 0,
			pending_review_count: 0,
			rejected_count: 0,
			documents: [],
			remediation_actions: []
		});
		apiMock.listExpenses.mockResolvedValue([]);
		apiMock.listPayrollRuns.mockResolvedValue([]);
		apiMock.listKMD.mockResolvedValue([]);
		apiMock.getYearEndCloseStatus.mockResolvedValue({
			period_end_date: '2025-12-31',
			fiscal_year_label: '2025',
			fiscal_year_start_date: '2025-01-01',
			fiscal_year_end_date: '2025-12-31',
			carry_forward_date: '2026-01-01',
			is_fiscal_year_end: true,
			period_closed: true,
			has_profit_and_loss_activity: false,
			carry_forward_needed: false,
			carry_forward_ready: false,
			has_retained_earnings_account: true,
			net_income: new Decimal(0),
			remediation_actions: []
		});
		apiMock.listTSD.mockResolvedValue([
			{
				id: 'tsd-submitted-1',
				tenant_id: 'tenant-1',
				period_year: 2026,
				period_month: 3,
				total_payments: new Decimal(5100),
				total_income_tax: new Decimal(900),
				total_social_tax: new Decimal(1683),
				total_unemployment_employer: new Decimal(41),
				total_unemployment_employee: new Decimal(82),
				total_funded_pension: new Decimal(102),
				status: 'SUBMITTED',
				submitted_at: '2026-04-10T09:30:00Z',
				emta_reference: 'EMTA-2026-03',
				remediation_actions: [
					{
						code: 'tsd_awaiting_authority_acceptance',
						severity: 'ACTION',
						scope: 'tax',
						owner_role: 'accountant',
						workspace_queue: 'tsd_declarations',
						assignment_key:
							'tsd-declarations:tsd-awaiting-authority-acceptance:tsd-declaration:tsd-submitted-1:2026-03',
						priority: 'high',
						due_in_days: 1,
						message: 'TSD 2026-03 has been submitted and is awaiting authority acceptance.',
						action:
							'Monitor e-MTA acceptance, mark the declaration accepted or rejected, and retain the accepted confirmation.',
						entity_type: 'tsd_declaration',
						entity_id: 'tsd-submitted-1',
						period: '2026-03',
						ui_path: '/tsd?year=2026&month=3',
						cli_command: 'oa tsd mark-accepted --year 2026 --month 3'
					}
				],
				created_at: '2026-03-31T00:00:00Z',
				updated_at: '2026-04-10T09:30:00Z'
			}
		]);

		render(AccountantReviewPanel, {
			tenant: createTenant()
		});

		await waitFor(() => {
			expect(
				screen.getByText('TSD 2026-03 has been submitted and is awaiting authority acceptance.')
			).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Mark TSD accepted' }));

		await waitFor(() => {
			expect(apiMock.markTSDAccepted).toHaveBeenCalledWith('tenant-1', 2026, 3);
			expect(screen.getByText('TSD marked accepted from workspace.')).toBeInTheDocument();
		});
	});

	it('sets missing payroll payment dates from assignment rows', async () => {
		apiMock.listBankTransactions.mockResolvedValue([]);
		apiMock.getDocumentRetentionReview.mockResolvedValue({
			as_of_date: '2026-02-11',
			cutoff_date: '2026-03-13',
			total_count: 0,
			expired_count: 0,
			due_soon_count: 0,
			missing_retention_count: 0,
			pending_review_count: 0,
			rejected_count: 0,
			documents: [],
			remediation_actions: []
		});
		apiMock.listExpenses.mockResolvedValue([]);
		apiMock.listTSD.mockResolvedValue([]);
		apiMock.listKMD.mockResolvedValue([]);
		apiMock.getYearEndCloseStatus.mockResolvedValue({
			period_end_date: '2025-12-31',
			fiscal_year_label: '2025',
			fiscal_year_start_date: '2025-01-01',
			fiscal_year_end_date: '2025-12-31',
			carry_forward_date: '2026-01-01',
			is_fiscal_year_end: true,
			period_closed: true,
			has_profit_and_loss_activity: false,
			carry_forward_needed: false,
			carry_forward_ready: false,
			has_retained_earnings_account: true,
			net_income: new Decimal(0),
			remediation_actions: []
		});
		apiMock.listPayrollRuns.mockResolvedValue([
			{
				id: 'payroll-missing-date-1',
				tenant_id: 'tenant-1',
				period_year: 2026,
				period_month: 2,
				status: 'CALCULATED',
				total_gross: new Decimal(4200),
				total_net: new Decimal(3260),
				total_employer_cost: new Decimal(5628),
				remediation_actions: [
					{
						code: 'payroll_payment_date_missing',
						severity: 'WARNING',
						scope: 'payroll',
						owner_role: 'accountant',
						workspace_queue: 'payroll_runs',
						assignment_key:
							'payroll-runs:payroll-payment-date-missing:payroll-run:payroll-missing-date-1:2026-02',
						priority: 'normal',
						due_in_days: 3,
						message: 'Payroll run 2026-02 has no payment date.',
						action:
							'Confirm the intended salary payment date before approving payroll or filing TSD.',
						entity_type: 'payroll_run',
						entity_id: 'payroll-missing-date-1',
						period: '2026-02',
						ui_path: '/payroll?run_id=payroll-missing-date-1',
						cli_command:
							'oa payroll runs set-payment-date --id payroll-missing-date-1 --payment-date <YYYY-MM-DD>'
					}
				],
				created_at: '2026-02-01T00:00:00Z',
				updated_at: '2026-02-01T00:00:00Z'
			}
		]);

		render(AccountantReviewPanel, {
			tenant: createTenant()
		});

		await waitFor(() => {
			expect(screen.getByText('Payroll run 2026-02 has no payment date.')).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Set payment date' }));

		await waitFor(() => {
			expect(apiMock.updatePayrollPaymentDate).toHaveBeenCalledWith(
				'tenant-1',
				'payroll-missing-date-1',
				{
					payment_date: '2026-02-28'
				}
			);
			expect(screen.getByText('Payroll payment date set from workspace.')).toBeInTheDocument();
		});
	});

	it('executes KMD assignment rows from the workspace', async () => {
		apiMock.listBankTransactions.mockResolvedValue([]);
		apiMock.getDocumentRetentionReview.mockResolvedValue({
			as_of_date: '2026-04-11',
			cutoff_date: '2026-05-11',
			total_count: 0,
			expired_count: 0,
			due_soon_count: 0,
			missing_retention_count: 0,
			pending_review_count: 0,
			rejected_count: 0,
			documents: [],
			remediation_actions: []
		});
		apiMock.listExpenses.mockResolvedValue([]);
		apiMock.listPayrollRuns.mockResolvedValue([]);
		apiMock.listTSD.mockResolvedValue([]);
		apiMock.getYearEndCloseStatus.mockResolvedValue({
			period_end_date: '2025-12-31',
			fiscal_year_label: '2025',
			fiscal_year_start_date: '2025-01-01',
			fiscal_year_end_date: '2025-12-31',
			carry_forward_date: '2026-01-01',
			is_fiscal_year_end: true,
			period_closed: true,
			has_profit_and_loss_activity: false,
			carry_forward_needed: false,
			carry_forward_ready: false,
			has_retained_earnings_account: true,
			net_income: new Decimal(0),
			remediation_actions: []
		});
		apiMock.listKMD.mockResolvedValue([
			{
				id: 'kmd-empty-1',
				tenant_id: 'tenant-1',
				year: 2026,
				month: 4,
				status: 'DRAFT',
				total_output_vat: new Decimal(0),
				total_input_vat: new Decimal(0),
				rows: [],
				remediation_actions: [
					{
						code: 'kmd_no_vat_rows',
						severity: 'WARNING',
						scope: 'tax',
						owner_role: 'accountant',
						workspace_queue: 'kmd_declarations',
						assignment_key: 'kmd-declarations:kmd-no-vat-rows:kmd-declaration:kmd-empty-1:2026-04',
						priority: 'normal',
						due_in_days: 3,
						message: 'KMD 2026-04 has no VAT rows or totals.',
						action:
							'Confirm the period has no VAT activity, or post missing VAT-bearing invoices and regenerate KMD before export.',
						entity_type: 'kmd_declaration',
						entity_id: 'kmd-empty-1',
						period: '2026-04',
						ui_path: '/tax/kmd?year=2026&month=4',
						cli_command: 'oa tax kmd generate --year 2026 --month 4'
					}
				],
				created_at: '2026-04-30T00:00:00Z',
				updated_at: '2026-04-30T00:00:00Z'
			},
			{
				id: 'kmd-payable-1',
				tenant_id: 'tenant-1',
				year: 2026,
				month: 5,
				status: 'DRAFT',
				total_output_vat: new Decimal(220),
				total_input_vat: new Decimal(30),
				rows: [
					{
						code: '1',
						description: 'Standard rate sales',
						tax_base: new Decimal(1000),
						tax_amount: new Decimal(220)
					}
				],
				remediation_actions: [
					{
						code: 'kmd_payable_review',
						severity: 'ACTION',
						scope: 'tax',
						owner_role: 'accountant',
						workspace_queue: 'kmd_declarations',
						assignment_key:
							'kmd-declarations:kmd-payable-review:kmd-declaration:kmd-payable-1:2026-05',
						priority: 'high',
						due_in_days: 1,
						message: 'KMD 2026-05 has VAT payable of 190.',
						action:
							'Review output/input VAT totals, generate KMD INF when needed, export XML, and submit the declaration in e-MTA.',
						entity_type: 'kmd_declaration',
						entity_id: 'kmd-payable-1',
						period: '2026-05',
						ui_path: '/tax/kmd?year=2026&month=5',
						cli_command: 'oa tax kmd export-xml --year 2026 --month 5 --output ./kmd-2026-05.xml'
					}
				],
				created_at: '2026-05-31T00:00:00Z',
				updated_at: '2026-05-31T00:00:00Z'
			},
			{
				id: 'kmd-submitted-1',
				tenant_id: 'tenant-1',
				year: 2026,
				month: 6,
				status: 'SUBMITTED',
				total_output_vat: new Decimal(220),
				total_input_vat: new Decimal(30),
				rows: [
					{
						code: '1',
						description: 'Standard rate sales',
						tax_base: new Decimal(1000),
						tax_amount: new Decimal(220)
					}
				],
				submitted_at: '2026-06-20T00:00:00Z',
				remediation_actions: [
					{
						code: 'kmd_awaiting_authority_acceptance',
						severity: 'ACTION',
						scope: 'tax',
						owner_role: 'accountant',
						workspace_queue: 'kmd_declarations',
						assignment_key:
							'kmd-declarations:kmd-awaiting-authority-acceptance:kmd-declaration:kmd-submitted-1:2026-06',
						priority: 'high',
						due_in_days: 1,
						message: 'KMD 2026-06 has been submitted and is awaiting authority acceptance.',
						action: 'Monitor e-MTA acceptance and retain the accepted confirmation with supporting VAT evidence.',
						entity_type: 'kmd_declaration',
						entity_id: 'kmd-submitted-1',
						period: '2026-06',
						ui_path: '/tax/kmd?year=2026&month=6',
						cli_command: 'oa tax kmd mark-accepted --year 2026 --month 6'
					}
				],
				created_at: '2026-06-30T00:00:00Z',
				updated_at: '2026-06-30T00:00:00Z'
			}
		]);

		render(AccountantReviewPanel, {
			tenant: createTenant()
		});

		await waitFor(() => {
			expect(screen.getByText('KMD 2026-04 has no VAT rows or totals.')).toBeInTheDocument();
			expect(screen.getByText('KMD 2026-05 has VAT payable of 190.')).toBeInTheDocument();
			expect(
				screen.getByText('KMD 2026-06 has been submitted and is awaiting authority acceptance.')
			).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Regenerate KMD' }));

		await waitFor(() => {
			expect(apiMock.generateKMD).toHaveBeenCalledWith('tenant-1', {
				year: 2026,
				month: 4
			});
			expect(screen.getByText('KMD regenerated from workspace.')).toBeInTheDocument();
		});

		const payableKmdRow = screen.getByText('KMD 2026-05 has VAT payable of 190.').closest('li');
		expect(payableKmdRow).not.toBeNull();
		await fireEvent.click(within(payableKmdRow as HTMLElement).getByRole('button', { name: 'Export KMD XML' }));

		await waitFor(() => {
			expect(apiMock.downloadKMDXml).toHaveBeenCalledWith('tenant-1', 2026, 5);
			expect(screen.getByText('KMD XML exported from workspace.')).toBeInTheDocument();
		});

		const getPayableKmdRow = () =>
			screen.getByText('KMD 2026-05 has VAT payable of 190.').closest('li') as HTMLElement;

		await waitFor(() => {
			expect(
				within(getPayableKmdRow()).getByRole('button', { name: 'Mark KMD submitted' })
			).toBeEnabled();
		});
		await fireEvent.click(within(getPayableKmdRow()).getByRole('button', { name: 'Mark KMD submitted' }));

		await waitFor(() => {
			expect(apiMock.markKMDSubmitted).toHaveBeenCalledWith('tenant-1', 2026, 5);
			expect(screen.getByText('KMD marked submitted from workspace.')).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Mark KMD accepted' }));

		await waitFor(() => {
			expect(apiMock.markKMDAccepted).toHaveBeenCalledWith('tenant-1', 2026, 6);
			expect(screen.getByText('KMD marked accepted from workspace.')).toBeInTheDocument();
		});
	});

	it('executes tax report assignment rows from the workspace', async () => {
		apiMock.listBankTransactions.mockResolvedValue([]);
		apiMock.getDocumentRetentionReview.mockResolvedValue({
			as_of_date: '2026-03-31',
			cutoff_date: '2026-04-30',
			total_count: 0,
			expired_count: 0,
			due_soon_count: 0,
			missing_retention_count: 0,
			pending_review_count: 0,
			rejected_count: 0,
			documents: [],
			remediation_actions: []
		});
		apiMock.listExpenses.mockResolvedValue([]);
		apiMock.listPayrollRuns.mockResolvedValue([]);
		apiMock.listTSD.mockResolvedValue([]);
		apiMock.getYearEndCloseStatus.mockResolvedValue({
			period_end_date: '2025-12-31',
			fiscal_year_label: '2025',
			fiscal_year_start_date: '2025-01-01',
			fiscal_year_end_date: '2025-12-31',
			carry_forward_date: '2026-01-01',
			is_fiscal_year_end: true,
			period_closed: true,
			has_profit_and_loss_activity: false,
			carry_forward_needed: false,
			carry_forward_ready: false,
			has_retained_earnings_account: true,
			net_income: new Decimal(0),
			remediation_actions: []
		});
		apiMock.listKMD.mockResolvedValue([
			{
				id: 'kmd-2026-03',
				tenant_id: 'tenant-1',
				year: 2026,
				month: 3,
				status: 'DRAFT',
				total_output_vat: new Decimal(220),
				total_input_vat: new Decimal(80),
				rows: [],
				remediation_actions: [],
				created_at: '2026-03-31T00:00:00Z',
				updated_at: '2026-03-31T00:00:00Z'
			}
		]);
		apiMock.generateKMDINF.mockResolvedValue({
			tenant_id: 'tenant-1',
			year: 2026,
			month: 3,
			threshold: new Decimal(1000),
			generated_at: '2026-03-31T00:00:00Z',
			summary: [],
			rows: [],
			remediation_actions: [
				{
					code: 'kmd_inf_review_required',
					severity: 'ACTION',
					scope: 'tax',
					owner_role: 'accountant',
					workspace_queue: 'tax_reports',
					assignment_key: 'tax-reports:kmd-inf-review-required:kmd-inf-report:2026-03:2026-03',
					priority: 'high',
					due_in_days: 1,
					message: 'KMD INF 2026-03 has 2 threshold invoice rows.',
					action: 'Review partner-period threshold totals and archive the report.',
					entity_type: 'kmd_inf_report',
					entity_id: '2026-03',
					period: '2026-03',
					ui_path: '/tax/kmd?year=2026&month=3&view=inf',
					cli_command: 'oa tax kmd inf --year 2026 --month 3 --threshold 1000 --json'
				}
			]
		});
		apiMock.generateEUVATOSS.mockResolvedValue({
			tenant_id: 'tenant-1',
			year: 2026,
			quarter: 1,
			period_start: '2026-01-01T00:00:00Z',
			period_end: '2026-03-31T23:59:59Z',
			scheme: 'UNION',
			currency: 'EUR',
			include_b2b: false,
			generated_at: '2026-03-31T00:00:00Z',
			summary: [],
			rows: [],
			taxable_amount: new Decimal(100),
			vat_amount: new Decimal(19),
			total_amount: new Decimal(119),
			invoice_count: 1,
			line_count: 1,
			remediation_actions: [
				{
					code: 'eu_vat_oss_review_required',
					severity: 'ACTION',
					scope: 'tax',
					owner_role: 'accountant',
					workspace_queue: 'tax_reports',
					assignment_key: 'tax-reports:eu-vat-oss-review-required:eu-vat-oss-report:2026-q1:2026-q1',
					priority: 'high',
					due_in_days: 1,
					message: 'EU VAT OSS 2026-Q1 has VAT due of 19.',
					action: 'Review destination-country VAT totals and file the OSS return manually.',
					entity_type: 'eu_vat_oss_report',
					entity_id: '2026-Q1',
					period: '2026-Q1',
					ui_path: '/tax/oss?year=2026&quarter=1',
					cli_command: 'oa tax oss report --year 2026 --quarter 1 --json'
				}
			]
		});

		render(AccountantReviewPanel, {
			tenant: createTenant()
		});

		await waitFor(() => {
			expect(screen.getByText('KMD INF 2026-03 has 2 threshold invoice rows.')).toBeInTheDocument();
			expect(screen.getByText('EU VAT OSS 2026-Q1 has VAT due of 19.')).toBeInTheDocument();
		});

		const kmdInfRow = screen
			.getByText('KMD INF 2026-03 has 2 threshold invoice rows.')
			.closest('li');
		expect(kmdInfRow).not.toBeNull();
		await fireEvent.click(within(kmdInfRow as HTMLElement).getByRole('button', { name: 'Generate KMD INF' }));

		await waitFor(() => {
			expect(apiMock.generateKMDINF).toHaveBeenCalledWith('tenant-1', {
				year: 2026,
				month: 3
			});
			expect(screen.getByText('KMD INF generated from workspace.')).toBeInTheDocument();
		});

		const ossRow = screen.getByText('EU VAT OSS 2026-Q1 has VAT due of 19.').closest('li');
		expect(ossRow).not.toBeNull();
		await fireEvent.click(within(ossRow as HTMLElement).getByRole('button', { name: 'Generate OSS report' }));

		await waitFor(() => {
			expect(apiMock.generateEUVATOSS).toHaveBeenCalledWith('tenant-1', {
				year: 2026,
				quarter: 1
			});
			expect(screen.getByText('OSS report generated from workspace.')).toBeInTheDocument();
		});
	});

	it('completes expense assignment rows from the workspace', async () => {
		apiMock.listBankTransactions.mockResolvedValue([]);
		apiMock.getDocumentRetentionReview.mockResolvedValue({
			as_of_date: '2026-02-11',
			cutoff_date: '2026-03-13',
			total_count: 0,
			expired_count: 0,
			due_soon_count: 0,
			missing_retention_count: 0,
			pending_review_count: 0,
			rejected_count: 0,
			documents: [],
			remediation_actions: []
		});
		apiMock.listPayrollRuns.mockResolvedValue([]);
		apiMock.getYearEndCloseStatus.mockResolvedValue({
			period_end_date: '2025-12-31',
			fiscal_year_label: '2025',
			fiscal_year_start_date: '2025-01-01',
			fiscal_year_end_date: '2025-12-31',
			carry_forward_date: '2026-01-01',
			is_fiscal_year_end: true,
			period_closed: true,
			has_profit_and_loss_activity: false,
			carry_forward_needed: false,
			carry_forward_ready: false,
			has_retained_earnings_account: true,
			net_income: new Decimal(0),
			remediation_actions: []
		});
		apiMock.listExpenses.mockResolvedValue([
			{
				id: 'expense-draft-1',
				tenant_id: 'tenant-1',
				expense_number: 'EXP-DRAFT',
				expense_date: '2026-02-09',
				merchant: 'Taxi Co',
				expense_account_id: 'expense-account',
				payment_account_id: 'cash-account',
				amount: new Decimal(35),
				currency: 'EUR',
				exchange_rate: new Decimal(1),
				base_amount: new Decimal(35),
				requires_receipt: false,
				status: 'DRAFT',
				remediation_actions: [
					{
						code: 'expense_submit_for_approval',
						severity: 'ACTION',
						scope: 'expenses',
						owner_role: 'accountant',
						workspace_queue: 'expense_claims',
						assignment_key:
							'expense-claims:expense-submit-for-approval:expense:expense-draft-1:EXP-DRAFT:DRAFT',
						priority: 'high',
						due_in_days: 1,
						message: 'Expense EXP-DRAFT is still in draft.',
						action:
							'Review the merchant, accounts, amount, and evidence requirements, then submit it for approval.',
						entity_type: 'expense',
						entity_id: 'expense-draft-1',
						expense_number: 'EXP-DRAFT',
						status: 'DRAFT',
						ui_path: '/expenses?expense_id=expense-draft-1',
						cli_command: 'oa expenses submit --id expense-draft-1'
					}
				],
				created_at: '2026-02-09T00:00:00Z',
				created_by: 'user-1',
				updated_at: '2026-02-09T00:00:00Z'
			},
			{
				id: 'expense-submitted-1',
				tenant_id: 'tenant-1',
				expense_number: 'EXP-SUBMITTED',
				expense_date: '2026-02-10',
				merchant: 'Hotel Co',
				expense_account_id: 'expense-account',
				payment_account_id: 'cash-account',
				amount: new Decimal(120),
				currency: 'EUR',
				exchange_rate: new Decimal(1),
				base_amount: new Decimal(120),
				requires_receipt: false,
				status: 'SUBMITTED',
				remediation_actions: [
					{
						code: 'expense_approve_or_reject',
						severity: 'ACTION',
						scope: 'expenses',
						owner_role: 'accountant',
						workspace_queue: 'expense_claims',
						assignment_key:
							'expense-claims:expense-approve-or-reject:expense:expense-submitted-1:EXP-SUBMITTED:SUBMITTED',
						priority: 'high',
						due_in_days: 1,
						message: 'Expense EXP-SUBMITTED is awaiting approval.',
						action:
							'Approve the expense when policy evidence is complete, or reject it with a reason for correction.',
						entity_type: 'expense',
						entity_id: 'expense-submitted-1',
						expense_number: 'EXP-SUBMITTED',
						status: 'SUBMITTED',
						ui_path: '/expenses?expense_id=expense-submitted-1',
						cli_command: 'oa expenses approve --id expense-submitted-1'
					}
				],
				created_at: '2026-02-10T00:00:00Z',
				created_by: 'user-1',
				updated_at: '2026-02-10T00:00:00Z'
			},
			{
				id: 'expense-approved-1',
				tenant_id: 'tenant-1',
				expense_number: 'EXP-APPROVED',
				expense_date: '2026-02-11',
				merchant: 'Office Co',
				expense_account_id: 'expense-account',
				payment_account_id: 'cash-account',
				amount: new Decimal(64),
				currency: 'EUR',
				exchange_rate: new Decimal(1),
				base_amount: new Decimal(64),
				requires_receipt: false,
				status: 'APPROVED',
				remediation_actions: [
					{
						code: 'expense_post_to_ledger',
						severity: 'ACTION',
						scope: 'expenses',
						owner_role: 'accountant',
						workspace_queue: 'expense_claims',
						assignment_key:
							'expense-claims:expense-post-to-ledger:expense:expense-approved-1:EXP-APPROVED:APPROVED',
						priority: 'high',
						due_in_days: 1,
						message: 'Expense EXP-APPROVED is approved but not posted.',
						action:
							'Post the approved expense to create the balanced ledger entry before closing the period.',
						entity_type: 'expense',
						entity_id: 'expense-approved-1',
						expense_number: 'EXP-APPROVED',
						status: 'APPROVED',
						ui_path: '/expenses?expense_id=expense-approved-1',
						cli_command: 'oa expenses post --id expense-approved-1'
					}
				],
				created_at: '2026-02-11T00:00:00Z',
				created_by: 'user-1',
				updated_at: '2026-02-11T00:00:00Z'
			}
		]);

		render(AccountantReviewPanel, {
			tenant: createTenant()
		});

		await waitFor(() => {
			expect(screen.getByText('Expense EXP-DRAFT is still in draft.')).toBeInTheDocument();
			expect(screen.getByText('Expense EXP-SUBMITTED is awaiting approval.')).toBeInTheDocument();
			expect(
				screen.getByText('Expense EXP-APPROVED is approved but not posted.')
			).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Submit expense' }));

		await waitFor(() => {
			expect(apiMock.submitExpense).toHaveBeenCalledWith('tenant-1', 'expense-draft-1');
			expect(screen.getByText('Expense submitted from workspace.')).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Approve expense' }));

		await waitFor(() => {
			expect(apiMock.approveExpense).toHaveBeenCalledWith('tenant-1', 'expense-submitted-1');
			expect(screen.getByText('Expense approved from workspace.')).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Post expense' }));

		await waitFor(() => {
			expect(apiMock.postExpense).toHaveBeenCalledWith('tenant-1', 'expense-approved-1');
			expect(screen.getByText('Expense posted from workspace.')).toBeInTheDocument();
		});
	});

	it('saves follow-up updates from the review queue', async () => {
		render(AccountantReviewPanel, {
			tenant: createTenant()
		});

		await waitFor(() => {
			expect(screen.getByText('Unknown transfer')).toBeInTheDocument();
		});

		await fireEvent.change(screen.getByLabelText('Follow-up'), {
			target: { value: 'EVIDENCE_REQUIRED' }
		});
		await fireEvent.input(screen.getByLabelText('Review note'), {
			target: { value: 'Request signed receipt' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Save review' }));

		await waitFor(() => {
			expect(apiMock.reviewBankTransaction).toHaveBeenCalledWith('tenant-1', 'tx-1', {
				follow_up_status: 'EVIDENCE_REQUIRED',
				review_note: 'Request signed receipt'
			});
		});
	});

	it('sends overdue invoice reminders from the review queue', async () => {
		render(AccountantReviewPanel, {
			tenant: createTenant()
		});

		await waitFor(() => {
			expect(screen.getByRole('button', { name: 'Send Reminder' })).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Send Reminder' }));

		await waitFor(() => {
			expect(apiMock.sendPaymentReminder).toHaveBeenCalledWith('tenant-1', 'inv-1', undefined);
		});
		await waitFor(() => {
			expect(
				screen.getByText('Reminder sent successfully for invoice INV-001')
			).toBeInTheDocument();
		});
		expect(apiMock.getOverdueInvoices).toHaveBeenCalledTimes(2);
	});
});
