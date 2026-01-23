<script lang="ts">
	import { api, type Tenant, type TenantSettings } from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';

	interface Props {
		tenant: Tenant;
		oncomplete: () => void;
	}

	let { tenant, oncomplete }: Props = $props();

	let currentStep = $state(1);
	let isSubmitting = $state(false);
	let error = $state('');

	// Step 1: Company Profile
	let companyName = $state(tenant.name || '');
	let regCode = $state(tenant.settings?.reg_code || '');
	let vatNumber = $state(tenant.settings?.vat_number || '');
	let address = $state(tenant.settings?.address || '');
	let email = $state(tenant.settings?.email || '');
	let phone = $state(tenant.settings?.phone || '');

	// Step 2: Branding (optional)
	let logo = $state(tenant.settings?.logo || '');
	let pdfPrimaryColor = $state(tenant.settings?.pdf_primary_color || '#4f46e5');
	let bankDetails = $state(tenant.settings?.bank_details || '');
	let invoiceTerms = $state(tenant.settings?.invoice_terms || '');

	// Step 3: First Contact (optional)
	let contactName = $state('');
	let contactEmail = $state('');
	let contactType = $state<'CUSTOMER' | 'SUPPLIER'>('CUSTOMER');

	const totalSteps = 4;

	async function saveProgress() {
		isSubmitting = true;
		error = '';

		try {
			const settings: Partial<TenantSettings> = {
				reg_code: regCode,
				vat_number: vatNumber,
				address: address,
				email: email,
				phone: phone,
				logo: logo,
				pdf_primary_color: pdfPrimaryColor,
				bank_details: bankDetails,
				invoice_terms: invoiceTerms
			};

			await api.updateTenant(tenant.id, {
				name: companyName,
				settings
			});
		} catch (err) {
			error = err instanceof Error ? err.message : m.onboarding_failedToSave();
			isSubmitting = false;
			return false;
		}

		isSubmitting = false;
		return true;
	}

	async function nextStep() {
		if (currentStep === 1 || currentStep === 2) {
			const saved = await saveProgress();
			if (!saved) return;
		}

		if (currentStep === 3 && contactName && contactEmail) {
			// Create the contact
			try {
				await api.createContact(tenant.id, {
					name: contactName,
					email: contactEmail,
					contact_type: contactType
				});
			} catch (err) {
				// Don't block on contact creation failure
				console.error('Failed to create contact:', err);
			}
		}

		if (currentStep < totalSteps) {
			currentStep++;
		}
	}

	function prevStep() {
		if (currentStep > 1) {
			currentStep--;
		}
	}

	async function completeOnboarding() {
		isSubmitting = true;
		error = '';

		try {
			await api.completeOnboarding(tenant.id);
			oncomplete();
		} catch (err) {
			error = err instanceof Error ? err.message : m.onboarding_failedToComplete();
			isSubmitting = false;
		}
	}

	async function skipOnboarding() {
		await completeOnboarding();
	}

	function handleLogoUpload(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;

		if (file.size > 500 * 1024) {
			error = m.onboarding_logoTooLarge();
			return;
		}

		const reader = new FileReader();
		reader.onload = () => {
			logo = reader.result as string;
		};
		reader.readAsDataURL(file);
	}
</script>

<div class="onboarding-overlay">
	<div class="onboarding-wizard card">
		<div class="wizard-header">
			<h1>{m.onboarding_welcome()}</h1>
			<p class="subtitle">{m.onboarding_subtitle()}</p>

			<!-- Progress bar -->
			<div class="progress-bar">
				{#each Array(totalSteps) as _, i}
					<div class="progress-step" class:active={i + 1 <= currentStep} class:current={i + 1 === currentStep}>
						<div class="step-dot">{i + 1}</div>
						<span class="step-label">
							{#if i === 0}{m.onboarding_step1Title()}{:else if i === 1}{m.onboarding_step2Title()}{:else if i === 2}{m.onboarding_step3Title()}{:else}{m.onboarding_step4Title()}{/if}
						</span>
					</div>
					{#if i < totalSteps - 1}
						<div class="progress-line" class:active={i + 1 < currentStep}></div>
					{/if}
				{/each}
			</div>
		</div>

		{#if error}
			<div class="alert alert-error">{error}</div>
		{/if}

		<div class="wizard-content">
			{#if currentStep === 1}
				<!-- Step 1: Company Profile -->
				<div class="step-content">
					<h2>{m.onboarding_companyInfo()}</h2>
					<p class="step-description">{m.onboarding_companyInfoDesc()}</p>

					<div class="form-grid">
						<div class="form-group">
							<label class="label" for="companyName">{m.onboarding_companyName()}</label>
							<input class="input" type="text" id="companyName" bind:value={companyName} required />
						</div>
						<div class="form-group">
							<label class="label" for="regCode">{m.onboarding_regCode()}</label>
							<input class="input" type="text" id="regCode" bind:value={regCode} placeholder={m.onboarding_regCodePlaceholder()} />
						</div>
						<div class="form-group">
							<label class="label" for="vatNumber">{m.onboarding_vatNumber()}</label>
							<input class="input" type="text" id="vatNumber" bind:value={vatNumber} placeholder={m.onboarding_vatNumberPlaceholder()} />
						</div>
						<div class="form-group">
							<label class="label" for="email">{m.common_email()}</label>
							<input class="input" type="email" id="email" bind:value={email} placeholder={m.onboarding_emailPlaceholder()} />
						</div>
						<div class="form-group">
							<label class="label" for="phone">{m.common_phone()}</label>
							<input class="input" type="tel" id="phone" bind:value={phone} placeholder={m.onboarding_phonePlaceholder()} />
						</div>
						<div class="form-group full-width">
							<label class="label" for="address">{m.common_address()}</label>
							<textarea class="input" id="address" bind:value={address} rows="2" placeholder={m.onboarding_addressPlaceholder()}></textarea>
						</div>
					</div>
				</div>

			{:else if currentStep === 2}
				<!-- Step 2: Branding -->
				<div class="step-content">
					<h2>{m.onboarding_brandingTitle()}</h2>
					<p class="step-description">{m.onboarding_brandingDesc()}</p>

					<div class="form-grid">
						<div class="form-group">
							<span class="label">{m.onboarding_companyLogo()}</span>
							<div class="logo-upload">
								{#if logo}
									<div class="logo-preview">
										<img src={logo} alt="Logo" />
										<button type="button" class="btn btn-sm btn-secondary" onclick={() => (logo = '')}>{m.common_remove()}</button>
									</div>
								{:else}
									<div class="logo-placeholder">
										<input type="file" accept="image/png,image/jpeg,image/svg+xml" onchange={handleLogoUpload} id="logoUpload" />
										<label for="logoUpload" class="btn btn-secondary">{m.onboarding_uploadLogo()}</label>
										<span class="help-text">{m.onboarding_logoHelp()}</span>
									</div>
								{/if}
							</div>
						</div>
						<div class="form-group">
							<label class="label" for="pdfPrimaryColor">{m.onboarding_brandColor()}</label>
							<div class="color-input">
								<input type="color" id="pdfPrimaryColor" bind:value={pdfPrimaryColor} />
								<input class="input" type="text" bind:value={pdfPrimaryColor} placeholder="#4f46e5" />
							</div>
						</div>
						<div class="form-group full-width">
							<label class="label" for="bankDetails">{m.onboarding_bankDetails()}</label>
							<textarea class="input" id="bankDetails" bind:value={bankDetails} rows="2" placeholder={m.onboarding_bankDetailsPlaceholder()}></textarea>
							<span class="help-text">{m.onboarding_bankDetailsHelp()}</span>
						</div>
						<div class="form-group full-width">
							<label class="label" for="invoiceTerms">{m.onboarding_invoiceTerms()}</label>
							<textarea class="input" id="invoiceTerms" bind:value={invoiceTerms} rows="2" placeholder={m.onboarding_invoiceTermsPlaceholder()}></textarea>
						</div>
					</div>
				</div>

			{:else if currentStep === 3}
				<!-- Step 3: First Contact -->
				<div class="step-content">
					<h2>{m.onboarding_addFirstContact()}</h2>
					<p class="step-description">{m.onboarding_addFirstContactDesc()}</p>

					<div class="contact-type-selector">
						<button type="button" class="type-btn" class:active={contactType === 'CUSTOMER'} onclick={() => (contactType = 'CUSTOMER')}>
							<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
								<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
								<circle cx="9" cy="7" r="4" />
								<path d="M23 21v-2a4 4 0 0 0-3-3.87" />
								<path d="M16 3.13a4 4 0 0 1 0 7.75" />
							</svg>
							<span>{m.onboarding_customer()}</span>
						</button>
						<button type="button" class="type-btn" class:active={contactType === 'SUPPLIER'} onclick={() => (contactType = 'SUPPLIER')}>
							<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
								<path d="M6 22V4a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v18Z" />
								<path d="M6 12H4a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h2" />
								<path d="M18 9h2a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2h-2" />
							</svg>
							<span>{m.onboarding_supplier()}</span>
						</button>
					</div>

					<div class="form-grid">
						<div class="form-group">
							<label class="label" for="contactName">{contactType === 'CUSTOMER' ? m.onboarding_customerName() : m.onboarding_supplierName()}</label>
							<input class="input" type="text" id="contactName" bind:value={contactName} placeholder={m.onboarding_contactNamePlaceholder()} />
						</div>
						<div class="form-group">
							<label class="label" for="contactEmail">{m.common_email()}</label>
							<input class="input" type="email" id="contactEmail" bind:value={contactEmail} placeholder={m.onboarding_contactEmailPlaceholder()} />
						</div>
					</div>
				</div>

			{:else}
				<!-- Step 4: Complete -->
				<div class="step-content complete-step">
					<div class="complete-icon">
						<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
							<polyline points="22 4 12 14.01 9 11.01" />
						</svg>
					</div>
					<h2>{m.onboarding_allSet()}</h2>
					<p class="step-description">{m.onboarding_allSetDesc()}</p>

					<div class="quick-actions">
						<a href="/invoices?tenant={tenant.id}" class="quick-action-card">
							<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
								<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
								<polyline points="14 2 14 8 20 8" />
								<line x1="16" y1="13" x2="8" y2="13" />
								<line x1="16" y1="17" x2="8" y2="17" />
							</svg>
							<span>{m.onboarding_createInvoice()}</span>
						</a>
						<a href="/contacts?tenant={tenant.id}" class="quick-action-card">
							<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
								<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
								<circle cx="9" cy="7" r="4" />
								<path d="M23 21v-2a4 4 0 0 0-3-3.87" />
								<path d="M16 3.13a4 4 0 0 1 0 7.75" />
							</svg>
							<span>{m.onboarding_addContacts()}</span>
						</a>
						<a href="/accounts?tenant={tenant.id}" class="quick-action-card">
							<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
								<line x1="12" y1="1" x2="12" y2="23" />
								<path d="M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6" />
							</svg>
							<span>{m.onboarding_viewAccounts()}</span>
						</a>
						<a href="/settings/company?tenant={tenant.id}" class="quick-action-card">
							<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
								<circle cx="12" cy="12" r="3" />
								<path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z" />
							</svg>
							<span>{m.onboarding_moreSettings()}</span>
						</a>
					</div>
				</div>
			{/if}
		</div>

		<div class="wizard-footer">
			{#if currentStep < totalSteps}
				<button type="button" class="btn btn-ghost" onclick={skipOnboarding}>
					{m.onboarding_skip()}
				</button>
			{/if}

			<div class="footer-actions">
				{#if currentStep > 1 && currentStep < totalSteps}
					<button type="button" class="btn btn-secondary" onclick={prevStep}>
						{m.onboarding_back()}
					</button>
				{/if}

				{#if currentStep < totalSteps}
					<button type="button" class="btn btn-primary" onclick={nextStep} disabled={isSubmitting}>
						{isSubmitting ? m.onboarding_saving() : currentStep === 3 ? (contactName ? m.onboarding_addAndContinue() : m.onboarding_skipAndContinue()) : m.onboarding_continue()}
					</button>
				{:else}
					<button type="button" class="btn btn-primary" onclick={completeOnboarding} disabled={isSubmitting}>
						{isSubmitting ? m.onboarding_finishing() : m.onboarding_goToDashboard()}
					</button>
				{/if}
			</div>
		</div>
	</div>
</div>

<style>
	.onboarding-overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.6);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 1000;
		padding: 1rem;
	}

	.onboarding-wizard {
		width: 100%;
		max-width: 700px;
		max-height: 90vh;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
	}

	.wizard-header {
		text-align: center;
		padding-bottom: 1.5rem;
		border-bottom: 1px solid var(--color-border);
	}

	.wizard-header h1 {
		font-size: 1.5rem;
		margin: 0 0 0.5rem;
	}

	.subtitle {
		color: var(--color-text-muted);
		margin: 0 0 1.5rem;
	}

	.progress-bar {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0;
	}

	.progress-step {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.25rem;
	}

	.step-dot {
		width: 32px;
		height: 32px;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		font-weight: 600;
		font-size: 0.875rem;
		background: var(--color-bg);
		border: 2px solid var(--color-border);
		color: var(--color-text-muted);
		transition: all 0.3s;
	}

	.progress-step.active .step-dot {
		background: var(--color-primary);
		border-color: var(--color-primary);
		color: white;
	}

	.progress-step.current .step-dot {
		box-shadow: 0 0 0 4px rgba(79, 70, 229, 0.2);
	}

	.step-label {
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}

	.progress-step.active .step-label {
		color: var(--color-primary);
	}

	.progress-line {
		width: 40px;
		height: 2px;
		background: var(--color-border);
		margin: 0 0.5rem;
		margin-bottom: 1.25rem;
		transition: background 0.3s;
	}

	.progress-line.active {
		background: var(--color-primary);
	}

	.wizard-content {
		padding: 1.5rem 0;
		flex: 1;
	}

	.step-content h2 {
		font-size: 1.25rem;
		margin: 0 0 0.5rem;
	}

	.step-description {
		color: var(--color-text-muted);
		margin-bottom: 1.5rem;
	}

	.form-grid {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: 1rem;
	}

	.form-group.full-width {
		grid-column: 1 / -1;
	}

	.help-text {
		display: block;
		margin-top: 0.25rem;
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}

	.logo-upload {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.logo-preview {
		display: flex;
		align-items: center;
		gap: 1rem;
	}

	.logo-preview img {
		max-width: 120px;
		max-height: 48px;
		object-fit: contain;
		border: 1px solid var(--color-border);
		border-radius: 4px;
		padding: 0.25rem;
	}

	.logo-placeholder {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.logo-placeholder input[type='file'] {
		display: none;
	}

	.color-input {
		display: flex;
		gap: 0.5rem;
		align-items: center;
	}

	.color-input input[type='color'] {
		width: 48px;
		height: 38px;
		padding: 2px;
		border: 1px solid var(--color-border);
		border-radius: 4px;
		cursor: pointer;
	}

	.color-input input[type='text'] {
		flex: 1;
	}

	.contact-type-selector {
		display: flex;
		gap: 1rem;
		margin-bottom: 1.5rem;
	}

	.type-btn {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.5rem;
		padding: 1rem;
		border: 2px solid var(--color-border);
		border-radius: 8px;
		background: var(--color-bg);
		cursor: pointer;
		transition: all 0.2s;
	}

	.type-btn:hover {
		border-color: var(--color-primary);
	}

	.type-btn.active {
		border-color: var(--color-primary);
		background: var(--color-primary-light, #e0e7ff);
	}

	.type-btn svg {
		color: var(--color-text-muted);
	}

	.type-btn.active svg {
		color: var(--color-primary);
	}

	.complete-step {
		text-align: center;
	}

	.complete-icon {
		color: var(--color-success, #10b981);
		margin-bottom: 1rem;
	}

	.quick-actions {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: 1rem;
		margin-top: 1.5rem;
	}

	.quick-action-card {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.5rem;
		padding: 1rem;
		border: 1px solid var(--color-border);
		border-radius: 8px;
		text-decoration: none;
		color: inherit;
		transition: all 0.2s;
	}

	.quick-action-card:hover {
		border-color: var(--color-primary);
		background: var(--color-primary-light, #e0e7ff);
	}

	.quick-action-card svg {
		color: var(--color-primary);
	}

	.quick-action-card span {
		font-size: 0.875rem;
		font-weight: 500;
	}

	.wizard-footer {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding-top: 1rem;
		border-top: 1px solid var(--color-border);
	}

	.footer-actions {
		display: flex;
		gap: 0.5rem;
		margin-left: auto;
	}

	.btn-ghost {
		background: none;
		border: none;
		color: var(--color-text-muted);
		cursor: pointer;
		padding: 0.5rem 1rem;
	}

	.btn-ghost:hover {
		color: var(--color-text);
	}

	@media (max-width: 640px) {
		.form-grid {
			grid-template-columns: 1fr;
		}

		.form-group.full-width {
			grid-column: 1;
		}

		.quick-actions {
			grid-template-columns: 1fr;
		}

		.progress-line {
			width: 20px;
		}

		.step-label {
			display: none;
		}
	}
</style>
