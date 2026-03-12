import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import Decimal from 'decimal.js';
import { baseLocale, setLocale } from '$lib/paraglide/runtime.js';
import SetupCenter from '$lib/components/SetupCenter.svelte';
import type { DashboardSummary, Tenant } from '$lib/api';

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
			fiscal_year_start_month: 1
		},
		is_active: true,
		onboarding_completed: false,
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

function createSummary(overrides: Partial<DashboardSummary> = {}): DashboardSummary {
	return {
		total_revenue: new Decimal(0),
		total_expenses: new Decimal(0),
		net_income: new Decimal(0),
		revenue_change: new Decimal(0),
		expenses_change: new Decimal(0),
		total_receivables: new Decimal(0),
		total_payables: new Decimal(0),
		overdue_receivables: new Decimal(0),
		overdue_payables: new Decimal(0),
		draft_invoices: 0,
		pending_invoices: 0,
		overdue_invoices: 0,
		period_start: '2026-01-01',
		period_end: '2026-01-31',
		...overrides
	};
}

describe('SetupCenter', () => {
	afterEach(() => {
		cleanup();
	});

	beforeEach(() => {
		setLocale(baseLocale, { reload: false });
	});

	it('highlights the next setup task and reopens guided setup', async () => {
		const onopenwalkthrough = vi.fn();

		render(SetupCenter, {
			tenant: createTenant(),
			summary: null,
			onopenwalkthrough
		});

		expect(screen.getByText('Setup center')).toBeInTheDocument();
		expect(screen.getByText('0/5')).toBeInTheDocument();
		expect(screen.getByText('Company profile')).toBeInTheDocument();
		expect(screen.getByText('Next')).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Continue guided setup' }));
		expect(onopenwalkthrough).toHaveBeenCalledTimes(1);
	});

	it('marks completed setup areas and hides the walkthrough action once onboarding is done', () => {
		render(SetupCenter, {
			tenant: createTenant({
				onboarding_completed: true,
				settings: {
					default_currency: 'EUR',
					country_code: 'EE',
					timezone: 'Europe/Tallinn',
					date_format: 'YYYY-MM-DD',
					decimal_sep: '.',
					thousands_sep: ',',
					fiscal_year_start_month: 1,
					reg_code: '12345678',
					vat_number: 'EE123456789',
					address: 'Main Street 1',
					email: 'hello@example.com',
					phone: '+37255550000',
					logo: 'data:image/png;base64,abc',
					bank_details: 'IBAN EE123',
					invoice_terms: 'Payment due in 14 days',
					pdf_primary_color: '#0f766e',
					period_lock_date: '2026-01-31'
				}
			}),
			summary: createSummary({
				total_revenue: new Decimal(1200),
				total_expenses: new Decimal(400),
				total_receivables: new Decimal(300),
				pending_invoices: 1
			})
		});

		expect(screen.getByText('5/5')).toBeInTheDocument();
		expect(screen.getAllByText('Done')).toHaveLength(5);
		expect(screen.queryByRole('button', { name: 'Continue guided setup' })).not.toBeInTheDocument();
	});
});
