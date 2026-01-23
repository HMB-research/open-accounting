<script lang="ts">
	/**
	 * Modal dialog for creating new contacts (customers/suppliers).
	 * Provides a form with all contact fields including name, type,
	 * contact info, address, and payment terms.
	 *
	 * @example
	 * ```svelte
	 * <ContactFormModal
	 *   open={showModal}
	 *   tenantId={selectedTenant}
	 *   onSave={(contact) => contacts = [...contacts, contact]}
	 *   onClose={() => showModal = false}
	 * />
	 * ```
	 */
	import { api, type Contact, type ContactType } from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';

	/**
	 * Props for ContactFormModal component
	 */
	interface Props {
		/** Whether the modal is visible */
		open: boolean;
		/** ID of the tenant to create the contact for */
		tenantId: string;
		/** Callback when contact is successfully created */
		onSave: (contact: Contact) => void;
		/** Callback when modal is closed (cancel or backdrop click) */
		onClose: () => void;
	}

	let { open, tenantId, onSave, onClose }: Props = $props();

	let error = $state('');
	let isSubmitting = $state(false);

	// Form fields
	let newName = $state('');
	let newType = $state<ContactType>('CUSTOMER');
	let newEmail = $state('');
	let newPhone = $state('');
	let newVatNumber = $state('');
	let newAddress = $state('');
	let newCity = $state('');
	let newPostalCode = $state('');
	let newCountry = $state('EE');
	let newPaymentDays = $state(14);

	function resetForm() {
		newName = '';
		newType = 'CUSTOMER';
		newEmail = '';
		newPhone = '';
		newVatNumber = '';
		newAddress = '';
		newCity = '';
		newPostalCode = '';
		newCountry = 'EE';
		newPaymentDays = 14;
		error = '';
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();
		if (!tenantId || isSubmitting) return;

		isSubmitting = true;
		error = '';

		try {
			const contact = await api.createContact(tenantId, {
				name: newName,
				contact_type: newType,
				email: newEmail || undefined,
				phone: newPhone || undefined,
				vat_number: newVatNumber || undefined,
				address_line1: newAddress || undefined,
				city: newCity || undefined,
				postal_code: newPostalCode || undefined,
				country_code: newCountry,
				payment_terms_days: newPaymentDays
			});
			resetForm();
			onSave(contact);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create contact';
		} finally {
			isSubmitting = false;
		}
	}

	function handleClose() {
		resetForm();
		onClose();
	}
</script>

{#if open}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={handleClose} role="presentation">
		<div class="modal card" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-labelledby="create-contact-title" tabindex="-1">
			<h2 id="create-contact-title">{m.contacts_newContact()}</h2>

			{#if error}
				<div class="alert alert-error">{error}</div>
			{/if}

			<form onsubmit={handleSubmit}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="contact-name">{m.common_name()} *</label>
						<input
							class="input"
							type="text"
							id="contact-name"
							bind:value={newName}
							required
							placeholder={m.contacts_companyOrPerson()}
						/>
					</div>
					<div class="form-group">
						<label class="label" for="contact-type">{m.contacts_type()}</label>
						<select class="input" id="contact-type" bind:value={newType}>
							<option value="CUSTOMER">{m.contacts_customer()}</option>
							<option value="SUPPLIER">{m.contacts_supplier()}</option>
							<option value="BOTH">{m.contacts_both()}</option>
						</select>
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="contact-email">{m.common_email()}</label>
						<input
							class="input"
							type="email"
							id="contact-email"
							bind:value={newEmail}
							placeholder="email@example.com"
						/>
					</div>
					<div class="form-group">
						<label class="label" for="contact-phone">{m.common_phone()}</label>
						<input
							class="input"
							type="tel"
							id="contact-phone"
							bind:value={newPhone}
							placeholder="+372 5551234"
						/>
					</div>
				</div>

				<div class="form-group">
					<label class="label" for="contact-vat">{m.contacts_vatNumber()}</label>
					<input
						class="input"
						type="text"
						id="contact-vat"
						bind:value={newVatNumber}
						placeholder="EE123456789"
					/>
				</div>

				<div class="form-group">
					<label class="label" for="contact-address">{m.common_address()}</label>
					<input
						class="input"
						type="text"
						id="contact-address"
						bind:value={newAddress}
						placeholder={m.contacts_streetAddress()}
					/>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="contact-city">{m.contacts_city()}</label>
						<input class="input" type="text" id="contact-city" bind:value={newCity} placeholder="Tallinn" />
					</div>
					<div class="form-group">
						<label class="label" for="contact-postal">{m.contacts_postalCode()}</label>
						<input
							class="input"
							type="text"
							id="contact-postal"
							bind:value={newPostalCode}
							placeholder="10111"
						/>
					</div>
					<div class="form-group">
						<label class="label" for="contact-country">{m.contacts_country()}</label>
						<select class="input" id="contact-country" bind:value={newCountry}>
							<option value="EE">Estonia</option>
							<option value="LV">Latvia</option>
							<option value="LT">Lithuania</option>
							<option value="FI">Finland</option>
							<option value="DE">Germany</option>
						</select>
					</div>
				</div>

				<div class="form-group">
					<label class="label" for="contact-payment-days">{m.contacts_paymentTermsDays()}</label>
					<input
						class="input"
						type="number"
						id="contact-payment-days"
						bind:value={newPaymentDays}
						min="0"
						max="365"
					/>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={handleClose} disabled={isSubmitting}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary" disabled={isSubmitting}>
						{isSubmitting ? m.common_loading() : m.common_create()}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<style>
	.modal-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.5);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 200;
		padding: 1rem;
	}

	.modal {
		width: 100%;
		max-width: 600px;
		max-height: 90vh;
		overflow-y: auto;
	}

	.modal h2 {
		margin-bottom: 1.5rem;
	}

	.alert-error {
		background: #fee2e2;
		color: #dc2626;
		padding: 0.75rem;
		border-radius: 0.375rem;
		margin-bottom: 1rem;
	}

	.form-row {
		display: flex;
		gap: 1rem;
		flex-wrap: wrap;
	}

	.form-row .form-group {
		flex: 1;
		min-width: 150px;
	}

	.form-group {
		margin-bottom: 1rem;
	}

	.label {
		display: block;
		margin-bottom: 0.25rem;
		font-weight: 500;
		font-size: 0.875rem;
	}

	.input {
		width: 100%;
		padding: 0.5rem;
		border: 1px solid var(--color-border, #e5e7eb);
		border-radius: 0.375rem;
		font-size: 1rem;
	}

	.input:focus {
		outline: none;
		border-color: var(--color-primary, #3b82f6);
		box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
	}

	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1.5rem;
	}

	.btn {
		padding: 0.5rem 1rem;
		border-radius: 0.375rem;
		font-weight: 500;
		cursor: pointer;
		border: none;
	}

	.btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.btn-primary {
		background: var(--color-primary, #3b82f6);
		color: white;
	}

	.btn-primary:hover:not(:disabled) {
		background: var(--color-primary-dark, #2563eb);
	}

	.btn-secondary {
		background: #6b7280;
		color: white;
	}

	.btn-secondary:hover:not(:disabled) {
		background: #4b5563;
	}

	/* Mobile responsive */
	@media (max-width: 768px) {
		.modal-backdrop {
			padding: 0;
			align-items: flex-end;
		}

		.modal {
			max-width: 100%;
			max-height: 95vh;
			border-radius: 1rem 1rem 0 0;
		}

		.modal h2 {
			font-size: 1.25rem;
		}

		.form-row {
			flex-direction: column;
			gap: 0;
		}

		.form-row .form-group {
			width: 100%;
			min-width: unset;
		}

		.modal-actions {
			flex-direction: column-reverse;
		}

		.modal-actions button {
			width: 100%;
			min-height: 44px;
		}
	}
</style>
