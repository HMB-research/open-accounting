import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
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
		sendPaymentReminder: vi.fn(),
		listPeriodCloseEvents: vi.fn(),
		listJournalEntries: vi.fn(),
		getDocumentRetentionReview: vi.fn(),
		listPayrollRuns: vi.fn(),
		listTSD: vi.fn(),
		listKMD: vi.fn(),
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
						cli_command: 'oa banking transactions review --id tx-1 --follow-up-status READY_TO_MATCH'
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
				missing_evidence: false,
				has_pending_review: true
			}
		]);
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
					code: 'document_retention_due_soon',
					severity: 'WARNING',
					scope: 'documents',
					owner_role: 'accountant',
					workspace_queue: 'document_review',
					assignment_key: 'document-review:document-retention-due-soon:bank-transaction:tx-1:doc-1',
					priority: 'normal',
					due_in_days: 3,
					message: 'Document retention date needs review.',
					action: 'Extend retention or complete the disposal workflow.',
					ui_path: '/documents?review_status=PENDING',
					cli_command: 'oa documents retention --include-missing'
				}
			]
		});
		apiMock.listPayrollRuns.mockResolvedValue([
			{
				id: 'payroll-1',
				tenant_id: 'tenant-1',
				period_year: 2026,
				period_month: 2,
				status: 'DRAFT',
				total_gross: new Decimal(0),
				total_net: new Decimal(0),
				total_employer_cost: new Decimal(0),
				remediation_actions: [
					{
						code: 'payroll_run_calculation_required',
						severity: 'ACTION',
						scope: 'payroll',
						owner_role: 'accountant',
						workspace_queue: 'payroll_runs',
						assignment_key: 'payroll-runs:payroll-run-calculation-required:payroll-run:payroll-1:2026-02',
						priority: 'high',
						due_in_days: 1,
						message: 'Payroll run needs calculation.',
						action: 'Calculate payroll before approval.',
						ui_path: '/payroll',
						cli_command: 'oa payroll runs calculate --id payroll-1'
					}
				],
				created_at: '2026-02-01T00:00:00Z',
				updated_at: '2026-02-01T00:00:00Z'
			}
		]);
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
					cli_command: 'oa close period --period-end 2025-12-31 --reviewer-sign-off --note "Fiscal-year close"'
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
		apiMock.sendPaymentReminder.mockResolvedValue({
			invoice_id: 'inv-1',
			invoice_number: 'INV-001',
			success: true,
			message: 'Sent',
			reminder_id: 'rem-1'
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
		expect(screen.getByText('Review the items that still need an accountant decision before the next close or filing window.')).toBeInTheDocument();
		expect(screen.getByText('INV-001')).toBeInTheDocument();
		expect(screen.getByText('Unknown transfer')).toBeInTheDocument();
		expect(screen.getAllByText('Evidence pending review').length).toBeGreaterThan(0);
		expect(screen.getByText('Month-end accrual')).toBeInTheDocument();
		expect(screen.getByText('Assignment queue')).toBeInTheDocument();
		expect(screen.getByText('Fiscal year ending 2025-12-31 is not closed.')).toBeInTheDocument();
		expect(screen.getByText('Bank transaction needs matching.')).toBeInTheDocument();
		expect(screen.getByText('Payroll run needs calculation.')).toBeInTheDocument();
		expect(screen.getByText(/oa close period --period-end 2025-12-31/)).toBeInTheDocument();
		expect(
			screen
				.getAllByRole('link', { name: 'Open action' })
				.some((link) => link.getAttribute('href') === '/settings/company?tenant=tenant-1#period-history')
		).toBe(true);
		expect(screen.getAllByText('Closed').length).toBeGreaterThan(0);
		expect(screen.getByRole('link', { name: 'Open reminders' })).toHaveAttribute('href', '/invoices/reminders?tenant=tenant-1');
		expect(apiMock.listBankTransactions).toHaveBeenCalledWith('tenant-1', 'bank-1', { status: 'UNMATCHED' });
		expect(apiMock.listDocumentReviewSummaries).toHaveBeenCalledWith('tenant-1', 'bank_transaction', ['tx-1']);
		expect(apiMock.getDocumentRetentionReview).toHaveBeenCalledWith('tenant-1', {
			horizon_days: 30,
			include_missing: true
		});
		expect(apiMock.listPayrollRuns).toHaveBeenCalledWith('tenant-1');
		expect(apiMock.listTSD).toHaveBeenCalledWith('tenant-1');
		expect(apiMock.listKMD).toHaveBeenCalledWith('tenant-1');
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
			tenant: createTenant({ settings: { ...createTenant().settings, period_lock_date: null } })
		});

		await waitFor(() => {
			expect(screen.getByText('No overdue invoices need attention right now.')).toBeInTheDocument();
		});

		expect(screen.getByText('No unmatched bank transactions are waiting for review.')).toBeInTheDocument();
		expect(screen.getByText('No assignment-ready remediation actions are waiting right now.')).toBeInTheDocument();
		expect(screen.getByText('No close or reopen actions have been recorded yet.')).toBeInTheDocument();
		expect(screen.getByText('No recent journal entries to review yet.')).toBeInTheDocument();
		expect(screen.getByText('No periods locked yet')).toBeInTheDocument();
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
			expect(screen.getByText('Reminder sent successfully for invoice INV-001')).toBeInTheDocument();
		});
		expect(apiMock.getOverdueInvoices).toHaveBeenCalledTimes(2);
	});
});
