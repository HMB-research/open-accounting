<script lang="ts">
	/**
	 * Multi-step onboarding wizard for new tenant setup.
	 * Guides users through company profile, branding, and first contact creation.
	 *
	 * Steps:
	 * 1. Company Profile - Name, registration, VAT, contact details
	 * 2. Branding - Logo, colors, bank details, invoice terms
	 * 3. First Contact - Optional customer or supplier creation
	 * 4. Complete - Quick actions to get started
	 *
	 * @example
	 * ```svelte
	 * {#if tenant.needs_onboarding}
	 *   <OnboardingWizard
	 *     {tenant}
	 *     oncomplete={() => tenant.needs_onboarding = false}
	 *   />
	 * {/if}
	 * ```
	 */
	import { api, type Tenant, type TenantSettings } from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';

	/**
	 * Props for OnboardingWizard component
	 */
	interface Props {
		/** The tenant being onboarded */
		tenant: Tenant;
		/** Callback when onboarding is completed or skipped */
		oncomplete: () => void;
	}

	let { tenant, oncomplete }: Props = $props();

	let currentStep = $state(1);
	let isSubmitting = $state(false);
	let error = $state('');
	let initializedTenantId = $state('');

	// Step 1: Company Profile
	let companyName = $state('');
	let regCode = $state('');
	let vatNumber = $state('');
	let address = $state('');
	let email = $state('');
	let phone = $state('');

	// Step 2: Branding (optional)
	let logo = $state('');
	let pdfPrimaryColor = $state('#4f46e5');
	let bankDetails = $state('');
	let invoiceTerms = $state('');

	// Step 3: First Contact (optional)
	let contactName = $state('');
	let contactEmail = $state('');
	let contactType = $state<'CUSTOMER' | 'SUPPLIER'>('CUSTOMER');

	const totalSteps = 4;

	$effect(() => {
		if (!tenant?.id || initializedTenantId === tenant.id) return;

		const settings = tenant.settings ?? {};

		currentStep = 1;
		error = '';
		companyName = tenant.name || '';
		regCode = settings.reg_code || '';
		vatNumber = settings.vat_number || '';
		address = settings.address || '';
		email = settings.email || '';
		phone = settings.phone || '';
		logo = settings.logo || '';
		pdfPrimaryColor = settings.pdf_primary_color || '#4f46e5';
		bankDetails = settings.bank_details || '';
		invoiceTerms = settings.invoice_terms || '';
		contactName = '';
		contactEmail = '';
		contactType = 'CUSTOMER';
		initializedTenantId = tenant.id;
	});

	function getStepTitle(step: number): string {
		switch (step) {
			case 1:
				return m.onboarding_step1Title();
			case 2:
				return m.onboarding_step2Title();
			case 3:
				return m.onboarding_step3Title();
			default:
				return m.onboarding_step4Title();
		}
	}

	function getStepDescription(step: number): string {
		switch (step) {
			case 1:
				return m.onboarding_companyInfoDesc();
			case 2:
				return m.onboarding_brandingDesc();
			case 3:
				return m.onboarding_addFirstContactDesc();
			default:
				return m.onboarding_allSetDesc();
		}
	}

	function getStepHighlights(step: number): string[] {
		switch (step) {
			case 1:
				return [m.settings_companyProfile(), m.settings_regCode(), m.settings_vatNumber(), m.settings_email()];
			case 2:
				return [m.onboarding_companyLogo(), m.onboarding_bankDetails(), m.onboarding_invoiceTerms(), m.settings_primaryColor()];
			case 3:
				return [m.onboarding_customer(), m.onboarding_supplier(), m.contacts_importContacts()];
			default:
				return [m.invoices_newInvoice(), m.contacts_importContacts(), m.journal_importOpeningBalances(), m.settings_periodLockDate()];
		}
	}

	function isStepOptional(step: number): boolean {
		return step === 2 || step === 3;
	}

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
		if (currentStep === 1 && !companyName.trim()) {
			error = m.onboarding_companyNameRequired();
			return;
		}

		if (currentStep === 1 || currentStep === 2) {
			const saved = await saveProgress();
			if (!saved) return;
		}

		if (currentStep === 3 && contactName.trim()) {
			// Create the contact
			try {
				await api.createContact(tenant.id, {
					name: contactName.trim(),
					email: contactEmail.trim() || undefined,
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

		<div class="wizard-stage">
			<aside class="wizard-sidebar">
				<div class="sidebar-pill-row">
					<span class="sidebar-step-chip">0{currentStep}</span>
					<span class="sidebar-step-type">{isStepOptional(currentStep) ? m.common_optional() : m.onboarding_required()}</span>
				</div>
				<h2>{getStepTitle(currentStep)}</h2>
				<p class="sidebar-description">{getStepDescription(currentStep)}</p>

				<ul class="sidebar-highlights">
					{#each getStepHighlights(currentStep) as highlight}
						<li>{highlight}</li>
					{/each}
				</ul>

				{#if currentStep === 3}
					<div class="sidebar-note">
						<strong>{m.contacts_importContacts()}</strong>
						<p>{m.onboarding_contactImportHint()}</p>
					</div>
				{:else if currentStep === 4}
					<div class="sidebar-note">
						<strong>{m.onboarding_afterSetupTitle()}</strong>
						<p>{m.onboarding_afterSetupDesc()}</p>
					</div>
				{/if}
			</aside>

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
					<p class="inline-note">{m.onboarding_contactImportHint()}</p>
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
							<div>
								<strong>{m.onboarding_createInvoice()}</strong>
								<p>{m.onboarding_createInvoiceDesc()}</p>
							</div>
						</a>
						<a href="/contacts?tenant={tenant.id}" class="quick-action-card">
							<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
								<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
								<circle cx="9" cy="7" r="4" />
								<path d="M23 21v-2a4 4 0 0 0-3-3.87" />
								<path d="M16 3.13a4 4 0 0 1 0 7.75" />
							</svg>
							<div>
								<strong>{m.onboarding_addContacts()}</strong>
								<p>{m.onboarding_addContactsDesc()}</p>
							</div>
						</a>
						<a href="/journal?tenant={tenant.id}" class="quick-action-card">
							<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
								<line x1="12" y1="1" x2="12" y2="23" />
								<path d="M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6" />
							</svg>
							<div>
								<strong>{m.onboarding_importOpeningBalances()}</strong>
								<p>{m.onboarding_importOpeningBalancesDesc()}</p>
							</div>
						</a>
						<a href="/settings/company?tenant={tenant.id}" class="quick-action-card">
							<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
								<circle cx="12" cy="12" r="3" />
								<path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z" />
							</svg>
							<div>
								<strong>{m.onboarding_moreSettings()}</strong>
								<p>{m.onboarding_moreSettingsDesc()}</p>
							</div>
						</a>
					</div>
				</div>
			{/if}
			</div>
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
		max-width: 960px;
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

	.wizard-stage {
		display: grid;
		grid-template-columns: minmax(220px, 280px) minmax(0, 1fr);
		gap: 1.5rem;
		align-items: start;
		padding: 1.5rem 0;
	}

	.wizard-sidebar {
		position: sticky;
		top: 0;
		padding: 1.25rem;
		border-radius: 0.875rem;
		border: 1px solid var(--color-border);
		background:
			radial-gradient(circle at top right, rgba(37, 99, 235, 0.12), transparent 42%),
			linear-gradient(180deg, #ffffff 0%, #f8fbff 100%);
	}

	.sidebar-pill-row {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		margin-bottom: 1rem;
	}

	.sidebar-step-chip,
	.sidebar-step-type {
		display: inline-flex;
		align-items: center;
		padding: 0.2rem 0.6rem;
		border-radius: 999px;
		font-size: 0.75rem;
		font-weight: 700;
		letter-spacing: 0.04em;
		text-transform: uppercase;
	}

	.sidebar-step-chip {
		background: rgba(37, 99, 235, 0.12);
		color: var(--color-primary);
	}

	.sidebar-step-type {
		background: rgba(148, 163, 184, 0.16);
		color: #475569;
	}

	.wizard-sidebar h2 {
		font-size: 1.25rem;
		margin-bottom: 0.45rem;
	}

	.sidebar-description {
		color: var(--color-text-muted);
		margin-bottom: 1rem;
	}

	.sidebar-highlights {
		list-style: none;
		display: grid;
		gap: 0.6rem;
		margin-bottom: 1rem;
	}

	.sidebar-highlights li {
		display: flex;
		align-items: center;
		gap: 0.55rem;
		font-size: 0.9rem;
	}

	.sidebar-highlights li::before {
		content: '';
		width: 0.55rem;
		height: 0.55rem;
		border-radius: 999px;
		background: linear-gradient(135deg, var(--color-primary), #38bdf8);
		flex-shrink: 0;
	}

	.sidebar-note {
		padding: 0.9rem 1rem;
		border-radius: 0.75rem;
		background: rgba(255, 255, 255, 0.8);
		border: 1px solid rgba(37, 99, 235, 0.12);
	}

	.sidebar-note strong {
		display: block;
		margin-bottom: 0.35rem;
	}

	.sidebar-note p {
		font-size: 0.85rem;
		color: var(--color-text-muted);
	}

	.wizard-content {
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

	.inline-note {
		margin-top: 1rem;
		padding: 0.8rem 1rem;
		border-radius: 0.75rem;
		background: rgba(37, 99, 235, 0.08);
		color: var(--color-text-muted);
		font-size: 0.875rem;
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
		align-items: flex-start;
		gap: 0.85rem;
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
		flex-shrink: 0;
	}

	.quick-action-card div {
		text-align: left;
	}

	.quick-action-card strong {
		display: block;
		font-size: 0.95rem;
		margin-bottom: 0.2rem;
	}

	.quick-action-card p {
		font-size: 0.82rem;
		color: var(--color-text-muted);
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
		.wizard-stage {
			grid-template-columns: 1fr;
		}

		.wizard-sidebar {
			position: static;
		}

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
