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
						opening_balance: new Decimal(0),
						current_balance: new Decimal(0),
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
						created_at: '2026-02-09T00:00:00Z'
					}
				];
			}

			return [];
		});
		apiMock.listDocumentReviewSummaries.mockImplementation(async (tenantId: string) => {
			if (tenantId === 'tenant-1') {
				return [
					{
						entity_type: 'bank_transaction',
						entity_id: 'tx-1',
						total_count: 0,
						pending_review_count: 0,
						reviewed_count: 0,
						missing_evidence: true,
						has_pending_review: false
					}
				];
			}

			return [];
		});

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

		apiMock.listJournalEntries.mockResolvedValue([]);
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
		expect(screen.getByText('1 banking')).toBeInTheDocument();
		expect(screen.getAllByText('1 missing evidence').length).toBeGreaterThan(0);
		expect(screen.getAllByText('Close due').length).toBeGreaterThan(0);
		expect(screen.getAllByText('Current workspace').length).toBeGreaterThan(0);
		expect(screen.getByRole('link', { name: 'Open workspace' })).toHaveAttribute('href', '/dashboard?tenant=tenant-1');
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

		render(AccountantPortfolioPanel, {
			memberships: [acme, beta],
			currentTenantId: 'tenant-1'
		});

		await waitFor(() => {
			expect(screen.getByText('Nothing urgent is drifting across your current tenant portfolio.')).toBeInTheDocument();
		});
	});
});
