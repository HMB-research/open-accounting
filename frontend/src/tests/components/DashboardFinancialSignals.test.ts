import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@testing-library/svelte';
import Decimal from 'decimal.js';
import { baseLocale, setLocale } from '$lib/paraglide/runtime.js';
import DashboardFinancialSignals from '$lib/components/DashboardFinancialSignals.svelte';
import type { DashboardSummary } from '$lib/api';

function createSummary(overrides: Partial<DashboardSummary> = {}): DashboardSummary {
	return {
		total_revenue: new Decimal(0),
		total_expenses: new Decimal(0),
		net_income: new Decimal(1400),
		revenue_change: new Decimal(0),
		expenses_change: new Decimal(0),
		total_receivables: new Decimal(3200),
		total_payables: new Decimal(900),
		overdue_receivables: new Decimal(1200),
		overdue_payables: new Decimal(430),
		draft_invoices: 0,
		pending_invoices: 3,
		overdue_invoices: 2,
		period_start: '2026-01-01',
		period_end: '2026-01-31',
		...overrides
	};
}

function currency(value: number): string {
	return new Intl.NumberFormat('et-EE', {
		style: 'currency',
		currency: 'EUR',
		maximumFractionDigits: 0
	}).format(value);
}

function expectCurrency(value: number) {
	const normalized = currency(value).replace(/\s/g, ' ');
	expect(screen.getByText((content) => content.replace(/\s/g, ' ') === normalized)).toBeInTheDocument();
}

describe('DashboardFinancialSignals', () => {
	afterEach(() => {
		cleanup();
	});

	beforeEach(() => {
		setLocale(baseLocale, { reload: false });
	});

	it('surfaces receivables and payables from the dashboard summary', () => {
		render(DashboardFinancialSignals, {
			summary: createSummary()
		});

		expect(screen.getByLabelText('Financial signals')).toBeInTheDocument();
		expect(screen.getByText('Receivables')).toBeInTheDocument();
		expectCurrency(3200);
		expect(screen.getByText('Payables')).toBeInTheDocument();
		expectCurrency(900);
		expect(screen.getByText('Overdue receivables')).toBeInTheDocument();
		expectCurrency(1200);
		expect(screen.getByText('Overdue payables')).toBeInTheDocument();
		expectCurrency(430);
		expect(screen.getByText('Pending')).toBeInTheDocument();
		expect(screen.getByText('3')).toBeInTheDocument();
	});

	it('keeps payables visible while the dashboard summary is loading', () => {
		render(DashboardFinancialSignals, {
			summary: null
		});

		expect(screen.getByText('Payables')).toBeInTheDocument();
		expect(screen.getByText('Overdue payables')).toBeInTheDocument();
		expect(screen.getAllByText('--')).toHaveLength(6);
	});
});
