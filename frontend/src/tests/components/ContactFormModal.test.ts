import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { baseLocale, setLocale } from '$lib/paraglide/runtime.js';
import type { Contact } from '$lib/api';

const { apiMock } = vi.hoisted(() => ({
	apiMock: {
		createContact: vi.fn(),
		updateContact: vi.fn()
	}
}));

vi.mock('$lib/api', async () => {
	const actual = await vi.importActual<typeof import('$lib/api')>('$lib/api');
	return {
		...actual,
		api: apiMock
	};
});

import ContactFormModal from '$lib/components/ContactFormModal.svelte';

function createContact(overrides: Partial<Contact> = {}): Contact {
	return {
		id: 'contact-1',
		tenant_id: 'tenant-1',
		name: 'Baltic Commerce',
		contact_type: 'CUSTOMER',
		email: 'billing@baltic.example',
		phone: '+372 5551234',
		vat_number: 'EE123456789',
		address_line1: 'Harju 1',
		city: 'Tallinn',
		postal_code: '10111',
		country_code: 'EE',
		payment_terms_days: 14,
		is_active: true,
		created_at: '2026-06-01T00:00:00Z',
		updated_at: '2026-06-01T00:00:00Z',
		...overrides
	};
}

describe('ContactFormModal', () => {
	beforeEach(() => {
		setLocale(baseLocale, { reload: false });
		vi.clearAllMocks();
		apiMock.createContact.mockResolvedValue(createContact({ id: 'created-contact' }));
		apiMock.updateContact.mockResolvedValue(createContact({ id: 'updated-contact' }));
	});

	afterEach(() => {
		cleanup();
	});

	it('creates a contact with optional fields normalized for the API payload', async () => {
		const onSave = vi.fn();

		render(ContactFormModal, {
			open: true,
			tenantId: 'tenant-1',
			onSave,
			onClose: vi.fn()
		});

		await fireEvent.input(screen.getByLabelText(/Name/i), {
			target: { value: 'Nordic Supplies' }
		});
		await fireEvent.change(screen.getByLabelText('Type'), {
			target: { value: 'SUPPLIER' }
		});
		await fireEvent.input(screen.getByLabelText('Email'), {
			target: { value: 'payables@nordic.example' }
		});
		await fireEvent.input(screen.getByLabelText('Payment Terms (days)'), {
			target: { value: '30' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Create' }));

		await waitFor(() => {
			expect(apiMock.createContact).toHaveBeenCalledWith('tenant-1', {
				name: 'Nordic Supplies',
				email: 'payables@nordic.example',
				phone: undefined,
				vat_number: undefined,
				address_line1: undefined,
				city: undefined,
				postal_code: undefined,
				country_code: 'EE',
				payment_terms_days: 30,
				contact_type: 'SUPPLIER'
			});
		});
		expect(onSave).toHaveBeenCalledWith(createContact({ id: 'created-contact' }));
	});

	it('updates an existing contact without changing its read-only type', async () => {
		const onSave = vi.fn();

		render(ContactFormModal, {
			open: true,
			tenantId: 'tenant-1',
			contact: createContact({
				id: 'supplier-1',
				name: 'Existing Supplier',
				contact_type: 'SUPPLIER',
				payment_terms_days: 21
			}),
			onSave,
			onClose: vi.fn()
		});

		const typeSelect = screen.getByLabelText('Type') as HTMLSelectElement;
		expect(typeSelect).toBeDisabled();
		expect(typeSelect.value).toBe('SUPPLIER');
		expect(
			screen.getByText('Create a new contact if the customer/supplier role needs to change.')
		).toBeInTheDocument();

		await fireEvent.input(screen.getByLabelText(/Name/i), {
			target: { value: 'Updated Supplier' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Save' }));

		await waitFor(() => {
			expect(apiMock.updateContact).toHaveBeenCalledWith('tenant-1', 'supplier-1', {
				name: 'Updated Supplier',
				email: 'billing@baltic.example',
				phone: '+372 5551234',
				vat_number: 'EE123456789',
				address_line1: 'Harju 1',
				city: 'Tallinn',
				postal_code: '10111',
				country_code: 'EE',
				payment_terms_days: 21
			});
		});
		expect(onSave).toHaveBeenCalledWith(createContact({ id: 'updated-contact' }));
	});

	it('surfaces API errors and keeps the modal open', async () => {
		apiMock.createContact.mockRejectedValueOnce(new Error('Duplicate contact code'));

		render(ContactFormModal, {
			open: true,
			tenantId: 'tenant-1',
			onSave: vi.fn(),
			onClose: vi.fn()
		});

		await fireEvent.input(screen.getByLabelText(/Name/i), {
			target: { value: 'Duplicate Customer' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Create' }));

		expect(await screen.findByText('Duplicate contact code')).toBeInTheDocument();
		expect(screen.getByRole('dialog', { name: 'New Contact' })).toBeInTheDocument();
	});

	it('resets form state before closing from cancel', async () => {
		const onClose = vi.fn();

		render(ContactFormModal, {
			open: true,
			tenantId: 'tenant-1',
			onSave: vi.fn(),
			onClose
		});

		await fireEvent.input(screen.getByLabelText(/Name/i), {
			target: { value: 'Draft Contact' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));

		expect(onClose).toHaveBeenCalledTimes(1);
	});
});
