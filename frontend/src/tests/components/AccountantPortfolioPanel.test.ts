import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@testing-library/svelte';
import Decimal from 'decimal.js';
import { baseLocale, setLocale } from '$lib/paraglide/runtime.js';
import type { TenantMembership } from '$lib/api';

const { apiMock } = vi.hoisted(() => ({
	apiMock: {
		getOverdueInvoices: vi.fn(),
		listBankAccounts: vi.fn(),
		listBankTransactions: vi.fn(),
		listDocumentReviewSummaries: vi.fn(),
		listPeriodCloseEvents: vi.fn(),
		listJournalEntries: vi.fn(),
		getDocumentRetentionReview: vi.fn(),
		listExpenses: vi.fn(),
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

import AccountantPortfolioPanel from '$lib/components/AccountantPortfolioPanel.svelte';

function createMembership(
	tenantId: string,
	name: string,
	overrides: Partial<TenantMembership> = {}
): TenantMembership {
	return {
		tenant: {
			id: tenantId,
			name,
			slug: name.toLowerCase().replace(/\s+/g, '-'),
			schema_name: `tenant_${tenantId}`,
			settings: {
				default_currency: 'EUR',
				country_code: 'EE',
				timezone: 'Europe/Tallinn',
				date_format: 'YYYY-MM-DD',
				decimal_sep: '.',
				thousands_sep: ',',
				fiscal_year_start_month: 1,
				period_lock_date: tenantId === 'tenant-1' ? '2026-01-31' : null
			},
			is_active: true,
			onboarding_completed: true,
			created_at: '2026-01-01T00:00:00Z',
			updated_at: '2026-01-01T00:00:00Z'
		},
		role: 'accountant',
		is_default: tenantId === 'tenant-1',
		...overrides
	};
}

function getPreviousMonthEndIso(today: Date = new Date()): string {
	const previousMonthEnd = new Date(Date.UTC(today.getUTCFullYear(), today.getUTCMonth(), 0));
	const year = previousMonthEnd.getUTCFullYear();
	const month = String(previousMonthEnd.getUTCMonth() + 1).padStart(2, '0');
	const day = String(previousMonthEnd.getUTCDate()).padStart(2, '0');
	return `${year}-${month}-${day}`;
}

describe('AccountantPortfolioPanel', () => {
	afterEach(() => {
		cleanup();
	});

	beforeEach(() => {
		setLocale(baseLocale, { reload: false });
		vi.clearAllMocks();

		apiMock.getOverdueInvoices.mockImplementation(async (tenantId: string) => {
			if (tenantId === 'tenant-1') {
				return {
					total_overdue: '4200',
					invoice_count: 2,
					contact_count: 1,
					average_days_overdue: 21,
					invoices: []
				};
			}

			return {
				total_overdue: '0',
				invoice_count: 0,
				contact_count: 0,
				average_days_overdue: 0,
				invoices: []
			};
		});

		apiMock.listBankAccounts.mockImplementation(async (tenantId: string) => {
			if (tenantId === 'tenant-1') {
				return [
					{
						id: 'bank-1',
						tenant_id: tenantId,
						name: 'Main bank',
						account_number: 'EE111',
						currency: 'EUR',
						balance: new Decimal(0),
						is_active: true,
						created_at: '2026-01-01T00:00:00Z',
						updated_at: '2026-01-01T00:00:00Z'
					}
				];
			}

			return [];
		});

		apiMock.listBankTransactions.mockImplementation(async (tenantId: string, accountId: string) => {
			if (tenantId === 'tenant-1' && accountId === 'bank-1') {
				return [
					{
						id: 'tx-1',
						tenant_id: tenantId,
						bank_account_id: accountId,
						transaction_date: '2026-02-09',
						description: 'Unmatched transfer',
						amount: new Decimal('-640'),
						currency: 'EUR',
						status: 'UNMATCHED',
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
								action: 'Match the transaction or mark follow-up.',
								ui_path: '/banking',
								cli_command: 'oa banking transactions review --id tx-1 --follow-up-status READY_TO_MATCH'
							}
						],
						created_at: '2026-02-09T00:00:00Z'
					},
					{
						id: 'tx-2',
						tenant_id: tenantId,
						bank_account_id: accountId,
						transaction_date: '2026-02-08',
						description: 'Receipt pending',
						amount: new Decimal('180'),
						currency: 'EUR',
						status: 'UNMATCHED',
						created_at: '2026-02-08T00:00:00Z'
					}
				];
			}

			return [];
		});
		apiMock.listDocumentReviewSummaries.mockImplementation(
			async (tenantId: string, entityType: string) => {
				if (tenantId === 'tenant-1' && entityType === 'bank_transaction') {
					return [
						{
							entity_type: 'bank_transaction',
							entity_id: 'tx-1',
							total_count: 0,
							pending_review_count: 0,
							reviewed_count: 0,
							approved_count: 0,
							rejected_count: 0,
							missing_evidence: true,
							has_pending_review: false,
							has_rejected: false
						},
						{
							entity_type: 'bank_transaction',
							entity_id: 'tx-2',
							total_count: 1,
							pending_review_count: 1,
							reviewed_count: 0,
							approved_count: 0,
							rejected_count: 0,
							missing_evidence: false,
							has_pending_review: true,
							has_rejected: false
						}
					];
				}

				if (tenantId === 'tenant-1' && entityType === 'journal_entry') {
					return [
						{
							entity_type: 'journal_entry',
							entity_id: 'journal-1',
							total_count: 1,
							pending_review_count: 1,
							reviewed_count: 0,
							approved_count: 0,
							rejected_count: 0,
							missing_evidence: false,
							has_pending_review: true,
							has_rejected: false
						}
					];
				}

				return [];
			}
		);

		apiMock.listPeriodCloseEvents.mockImplementation(async (tenantId: string) => {
			if (tenantId === 'tenant-1') {
				return [
					{
						id: 'close-1',
						tenant_id: tenantId,
						action: 'close',
						close_kind: 'month_end',
						period_end_date: '2026-01-31',
						lock_date_after: '2026-01-31',
						performed_by: 'user-1',
						created_at: '2026-02-02T00:00:00Z'
					}
				];
			}

			return [];
		});

		apiMock.listJournalEntries.mockImplementation(async (tenantId: string) => {
			if (tenantId === 'tenant-1') {
				return [
					{
						id: 'journal-1',
						tenant_id: tenantId,
						entry_number: 'JE-001',
						entry_date: '2026-02-10',
						description: 'Manual accrual',
						requires_evidence: true,
						status: 'DRAFT',
						lines: [],
						created_at: '2026-02-10T00:00:00Z',
						created_by: 'user-1'
					}
				];
			}

			return [];
		});
		apiMock.getDocumentRetentionReview.mockImplementation(async (tenantId: string) => ({
			as_of_date: '2026-02-11',
			cutoff_date: '2026-03-13',
			total_count: tenantId === 'tenant-1' ? 1 : 0,
			expired_count: 0,
			due_soon_count: tenantId === 'tenant-1' ? 1 : 0,
			missing_retention_count: 0,
			pending_review_count: 0,
			rejected_count: 0,
			documents: [],
			remediation_actions:
				tenantId === 'tenant-1'
					? [
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
								action: 'Review retention.',
								ui_path: '/documents?review_status=PENDING',
								cli_command: 'oa documents retention --include-missing'
							}
						]
					: []
		}));
		apiMock.listExpenses.mockImplementation(async (tenantId: string) =>
			tenantId === 'tenant-1'
				? [
						{
							id: 'expense-1',
							tenant_id: tenantId,
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
									assignment_key: 'expense-claims:expense-receipt-approval-required:expense:expense-1:EXP-001:SUBMITTED',
									priority: 'high',
									due_in_days: 1,
									message: 'Expense EXP-001 is submitted and receipt-backed.',
									action: 'Confirm the receipt before approval.',
									entity_type: 'expense',
									entity_id: 'expense-1',
									expense_number: 'EXP-001',
									status: 'SUBMITTED',
									ui_path: '/expenses?expense_id=expense-1',
									cli_command: 'oa documents review-queue --entity-type expense --document-type receipt --status PENDING'
								}
							],
							created_at: '2026-02-09T00:00:00Z',
							created_by: 'user-1',
							updated_at: '2026-02-09T00:00:00Z'
						}
					]
				: []
		);
		apiMock.listPayrollRuns.mockImplementation(async (tenantId: string) =>
			tenantId === 'tenant-1'
				? [
						{
							id: 'payroll-1',
							tenant_id: tenantId,
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
					]
				: []
		);
		apiMock.listTSD.mockResolvedValue([]);
		apiMock.listKMD.mockResolvedValue([]);
		apiMock.getYearEndCloseStatus.mockImplementation(async (tenantId: string) => ({
			period_end_date: '2025-12-31',
			fiscal_year_label: '2025',
			fiscal_year_start_date: '2025-01-01',
			fiscal_year_end_date: '2025-12-31',
			carry_forward_date: '2026-01-01',
			is_fiscal_year_end: true,
			period_closed: tenantId !== 'tenant-1',
			has_profit_and_loss_activity: tenantId === 'tenant-1',
			carry_forward_needed: tenantId === 'tenant-1',
			carry_forward_ready: false,
			has_retained_earnings_account: true,
			net_income: new Decimal(tenantId === 'tenant-1' ? 1200 : 0),
			remediation_actions:
				tenantId === 'tenant-1'
					? [
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
								action: 'Close fiscal year.',
								ui_path: '/settings/company#period-history',
								cli_command: 'oa close period --period-end 2025-12-31 --reviewer-sign-off'
							}
						]
					: []
		}));
	});

	it('loads and renders the cross-tenant review rollup', async () => {
		render(AccountantPortfolioPanel, {
			memberships: [createMembership('tenant-1', 'Acme Corp'), createMembership('tenant-2', 'Beta Ltd')],
			currentTenantId: 'tenant-2'
		});

		await waitFor(() => {
			expect(apiMock.getOverdueInvoices).toHaveBeenCalledWith('tenant-1');
			expect(apiMock.getOverdueInvoices).toHaveBeenCalledWith('tenant-2');
		});

		await waitFor(() => {
			expect(screen.getByText('Acme Corp')).toBeInTheDocument();
			expect(screen.getByText('Beta Ltd')).toBeInTheDocument();
		});

		expect(screen.getByText('See what needs attention across your companies')).toBeInTheDocument();
		expect(screen.getByText('2 overdue')).toBeInTheDocument();
		expect(screen.getByText('2 banking')).toBeInTheDocument();
		expect(screen.getAllByText('1 missing evidence').length).toBeGreaterThan(0);
		expect(screen.getAllByText('1 pending review').length).toBeGreaterThan(0);
		expect(screen.getAllByText('1 accounting evidence').length).toBeGreaterThan(0);
		expect(screen.getByText('5 assignments')).toBeInTheDocument();
		expect(screen.getByText('workspace assignments')).toBeInTheDocument();
		expect(screen.getByText('Draft journal entries needing approved evidence: 1')).toBeInTheDocument();
		expect(screen.getAllByText('Close due').length).toBeGreaterThan(0);
		expect(screen.getAllByText('Current workspace').length).toBeGreaterThan(0);
		expect(screen.getByRole('link', { name: 'Open workspace' })).toHaveAttribute('href', '/dashboard?tenant=tenant-1');
		expect(screen.getByRole('link', { name: 'Open reminders' })).toHaveAttribute('href', '/invoices/reminders?tenant=tenant-1');
		expect(screen.getByRole('link', { name: 'Open banking' })).toHaveAttribute('href', '/banking?tenant=tenant-1');
		expect(screen.getByRole('link', { name: 'Review evidence' })).toHaveAttribute(
			'href',
			'/documents?tenant=tenant-1&entity_type=bank_transaction&review_status=PENDING'
		);
		expect(apiMock.listDocumentReviewSummaries).toHaveBeenCalledWith('tenant-1', 'journal_entry', ['journal-1']);
		expect(apiMock.listExpenses).toHaveBeenCalledWith('tenant-1', { limit: 100 });
		expect(screen.getByRole('link', { name: 'Open journal' })).toHaveAttribute('href', '/journal?tenant=tenant-1');
		expect(screen.getByRole('link', { name: 'Review journal evidence' })).toHaveAttribute(
			'href',
			'/documents?tenant=tenant-1&entity_type=journal_entry&review_status=PENDING'
		);
		expect(screen.getByRole('link', { name: 'Open assignments' })).toHaveAttribute(
			'href',
			'/dashboard?tenant=tenant-1#assignment-queue'
		);
		expect(
			screen
				.getAllByRole('link', { name: 'Close controls' })
				.some((link) => link.getAttribute('href') === '/settings/company?tenant=tenant-1#period-history')
		).toBe(true);
	});

	it('shows an empty state when no tenant needs review attention', async () => {
		const lockedThrough = getPreviousMonthEndIso();
		const acme = createMembership('tenant-1', 'Acme Corp', {
			tenant: {
				...createMembership('tenant-1', 'Acme Corp').tenant,
				settings: {
					...createMembership('tenant-1', 'Acme Corp').tenant.settings,
					period_lock_date: lockedThrough
				}
			}
		});
		const beta = createMembership('tenant-2', 'Beta Ltd', {
			tenant: {
				...createMembership('tenant-2', 'Beta Ltd').tenant,
				settings: {
					...createMembership('tenant-2', 'Beta Ltd').tenant.settings,
					period_lock_date: lockedThrough
				}
			}
		});

		apiMock.getOverdueInvoices.mockResolvedValue({
			total_overdue: '0',
			invoice_count: 0,
			contact_count: 0,
			average_days_overdue: 0,
			invoices: []
		});
		apiMock.listBankAccounts.mockResolvedValue([]);
		apiMock.listBankTransactions.mockResolvedValue([]);
		apiMock.listDocumentReviewSummaries.mockResolvedValue([]);
		apiMock.listJournalEntries.mockResolvedValue([]);
		apiMock.listPeriodCloseEvents.mockResolvedValue([
			{
				id: 'close-2',
				tenant_id: 'tenant-1',
				action: 'close',
				close_kind: 'month_end',
				period_end_date: lockedThrough,
				lock_date_after: lockedThrough,
				performed_by: 'user-1',
				created_at: '2026-03-01T00:00:00Z'
			}
		]);
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

		render(AccountantPortfolioPanel, {
			memberships: [acme, beta],
			currentTenantId: 'tenant-1'
		});

		await waitFor(() => {
			expect(screen.getByText('Nothing urgent is drifting across your current tenant portfolio.')).toBeInTheDocument();
		});
	});
});
