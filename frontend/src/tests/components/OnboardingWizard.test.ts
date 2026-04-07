import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { baseLocale, setLocale } from '$lib/paraglide/runtime.js';
import type { Tenant } from '$lib/api';

const { apiMock } = vi.hoisted(() => ({
	apiMock: {
		updateTenant: vi.fn(),
		createContact: vi.fn(),
		completeOnboarding: vi.fn()
	}
}));

vi.mock('$lib/api', async () => {
	const actual = await vi.importActual<typeof import('$lib/api')>('$lib/api');
	return {
		...actual,
		api: apiMock
	};
});

import OnboardingWizard from '$lib/components/OnboardingWizard.svelte';

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

describe('OnboardingWizard', () => {
	afterEach(() => {
		cleanup();
	});

	beforeEach(() => {
		setLocale(baseLocale, { reload: false });
		vi.clearAllMocks();
		apiMock.updateTenant.mockResolvedValue({});
		apiMock.createContact.mockResolvedValue({});
		apiMock.completeOnboarding.mockResolvedValue({});
	});

	it('blocks progression when company name is empty', async () => {
		render(OnboardingWizard, {
			tenant: createTenant({ name: '' }),
			oncomplete: vi.fn()
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Continue' }));

		expect(screen.getByText('Company name is required')).toBeInTheDocument();
		expect(apiMock.updateTenant).not.toHaveBeenCalled();
	});

	it('saves tenant settings and advances to optional branding', async () => {
		render(OnboardingWizard, {
			tenant: createTenant(),
			oncomplete: vi.fn()
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Continue' }));

		await waitFor(() => {
			expect(apiMock.updateTenant).toHaveBeenCalledWith('tenant-1', {
				name: 'Acme Corp',
				settings: {
					reg_code: '',
					vat_number: '',
					address: '',
					email: '',
					phone: '',
					logo: '',
					pdf_primary_color: '#4f46e5',
					bank_details: '',
					invoice_terms: ''
				}
			});
		});
		expect(screen.getAllByText('Branding')[0]).toBeInTheDocument();
		expect(screen.getAllByText('Optional').length).toBeGreaterThan(0);
	});

	it('creates the first contact even when email is omitted', async () => {
		render(OnboardingWizard, {
			tenant: createTenant(),
			oncomplete: vi.fn()
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Continue' }));
		await waitFor(() => expect(apiMock.updateTenant).toHaveBeenCalledTimes(1));

		await fireEvent.click(screen.getByRole('button', { name: 'Continue' }));
		await waitFor(() => expect(apiMock.updateTenant).toHaveBeenCalledTimes(2));

		await fireEvent.input(screen.getByLabelText('Customer Name'), {
			target: { value: 'First Customer' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Add & Continue' }));

		await waitFor(() => {
			expect(apiMock.createContact).toHaveBeenCalledWith('tenant-1', {
				name: 'First Customer',
				email: undefined,
				contact_type: 'CUSTOMER'
			});
		});
		expect(screen.getByRole('link', { name: /Import Opening Balances/i })).toBeInTheDocument();
	});

	it('completes onboarding when setup is skipped', async () => {
		const oncomplete = vi.fn();

		render(OnboardingWizard, {
			tenant: createTenant(),
			oncomplete
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Skip Setup' }));

		await waitFor(() => {
			expect(apiMock.completeOnboarding).toHaveBeenCalledWith('tenant-1');
		});
		expect(oncomplete).toHaveBeenCalledTimes(1);
	});
});
