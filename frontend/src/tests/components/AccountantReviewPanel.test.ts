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
		listPeriodCloseEvents: vi.fn(),
		listJournalEntries: vi.fn()
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
					issue_date: '2026-01-01',
					due_date: '2026-01-15',
					total: '1200',
					amount_paid: '0',
					outstanding_amount: '1200',
					currency: 'EUR',
					days_overdue: 27,
					reminder_count: 1
				}
			]
		});
		apiMock.listBankAccounts.mockResolvedValue([
			{
				id: 'bank-1',
				tenant_id: 'tenant-1',
				name: 'Main bank',
				account_number: 'EE111',
				currency: 'EUR',
				opening_balance: new Decimal(0),
				current_balance: new Decimal(0),
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
		expect(screen.getAllByText('Closed').length).toBeGreaterThan(0);
		expect(screen.getByRole('link', { name: 'Open reminders' })).toHaveAttribute('href', '/invoices/reminders?tenant=tenant-1');
		expect(apiMock.listBankTransactions).toHaveBeenCalledWith('tenant-1', 'bank-1', { status: 'UNMATCHED' });
		expect(apiMock.listDocumentReviewSummaries).toHaveBeenCalledWith('tenant-1', 'bank_transaction', ['tx-1']);
	});

	it('shows empty-state guidance when no review items are pending', async () => {
		apiMock.getOverdueInvoices.mockResolvedValue({
			total_overdue: '0',
			invoice_count: 0,
			contact_count: 0,
			average_days_overdue: 0,
			invoices: []
		});
		apiMock.listBankTransactions.mockResolvedValue([]);
		apiMock.listDocumentReviewSummaries.mockResolvedValue([]);
		apiMock.listPeriodCloseEvents.mockResolvedValue([]);
		apiMock.listJournalEntries.mockResolvedValue([]);

		render(AccountantReviewPanel, {
			tenant: createTenant({ settings: { ...createTenant().settings, period_lock_date: null } })
		});

		await waitFor(() => {
			expect(screen.getByText('No overdue invoices need attention right now.')).toBeInTheDocument();
		});

		expect(screen.getByText('No unmatched bank transactions are waiting for review.')).toBeInTheDocument();
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
});
