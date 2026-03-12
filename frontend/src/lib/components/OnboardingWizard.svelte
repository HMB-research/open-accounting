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
	let progressPercent = $derived((currentStep / totalSteps) * 100);

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

	function getStepStatusLabel(step: number): string {
		if (step < currentStep) return m.dashboard_setupDone();
		if (step === currentStep) return isStepOptional(step) ? m.common_optional() : m.onboarding_required();
		return m.dashboard_setupRecommended();
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
	<div class="onboarding-wizard">
		<div class="wizard-header">
			<div class="header-copy">
				<div class="header-eyebrow-row">
					<span class="header-eyebrow">{tenant.name || tenant.slug}</span>
					<span class="header-eyebrow subdued">{m.dashboard_setupCenter()}</span>
				</div>
				<h1>{m.onboarding_welcome()}</h1>
				<p class="subtitle">{m.onboarding_subtitle()}</p>
				<div class="header-chip-row" aria-hidden="true">
					{#each Array(totalSteps) as _, i}
						<span class="header-chip" class:current={i + 1 === currentStep}>{getStepTitle(i + 1)}</span>
					{/each}
				</div>
			</div>

			<div class="header-progress-card">
				<div class="progress-topline">
					<div class="progress-copy">
						<span>{m.dashboard_setupProgress()}</span>
						<strong>0{currentStep}/0{totalSteps}</strong>
					</div>
					<span class="progress-state">{getStepStatusLabel(currentStep)}</span>
				</div>
				<div
					class="progress-meter"
					role="progressbar"
					aria-valuemin="1"
					aria-valuemax={totalSteps}
					aria-valuenow={currentStep}
				>
					<span style={`width: ${progressPercent}%`}></span>
				</div>
				<p class="progress-caption">{getStepTitle(currentStep)} · {getStepDescription(currentStep)}</p>
			</div>
		</div>

		{#if error}
			<div class="alert alert-error onboarding-alert">{error}</div>
		{/if}

		<div class="wizard-stage">
			<aside class="wizard-sidebar">
				<div class="sidebar-current-step">
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
				</div>

				<nav class="sidebar-step-plan" aria-label={m.dashboard_setupProgress()}>
					{#each Array(totalSteps) as _, i}
						<div
							class="sidebar-step-card"
							class:current={i + 1 === currentStep}
							class:complete={i + 1 < currentStep}
						>
							<div class="sidebar-step-index">0{i + 1}</div>
							<div class="sidebar-step-body">
								<strong>{getStepTitle(i + 1)}</strong>
								<p>{getStepDescription(i + 1)}</p>
							</div>
							<span class="sidebar-step-state">{getStepStatusLabel(i + 1)}</span>
						</div>
					{/each}
				</nav>

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
				<div class="step-shell">
					<div class="step-shell-intro">
						<div>
							<span class="step-kicker">{m.dashboard_setupCenter()}</span>
							<h2>{currentStep === 1 ? m.onboarding_companyInfo() : currentStep === 2 ? m.onboarding_brandingTitle() : currentStep === 3 ? m.onboarding_addFirstContact() : m.onboarding_allSet()}</h2>
							<p class="step-description">
								{currentStep === 1 ? m.onboarding_companyInfoDesc() : currentStep === 2 ? m.onboarding_brandingDesc() : currentStep === 3 ? m.onboarding_addFirstContactDesc() : m.onboarding_allSetDesc()}
							</p>
						</div>
						<div class="step-type-badge">
							{isStepOptional(currentStep) ? m.common_optional() : m.onboarding_required()}
						</div>
					</div>

					{#if currentStep === 1}
						<div class="step-content">
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
						<div class="step-content">
							<div class="brand-preview-card">
								<div class="brand-preview-header">
									<div class="brand-preview-mark">
										<span class="brand-preview-swatch" style={`background: ${pdfPrimaryColor}`}></span>
										<div>
											<strong>{companyName.trim() || tenant.name || m.onboarding_companyInfo()}</strong>
											<p>{m.onboarding_brandingTitle()}</p>
										</div>
									</div>
									<span class="brand-preview-code">{pdfPrimaryColor}</span>
								</div>
								<div class="brand-preview-sheet">
									<div class="brand-preview-accent" style={`background: linear-gradient(135deg, ${pdfPrimaryColor}, #0f172a)`}></div>
									<div class="brand-preview-copy">
										<strong>{m.onboarding_createInvoice()}</strong>
										<p>{bankDetails || m.onboarding_bankDetailsHelp()}</p>
										<span>{invoiceTerms || m.onboarding_invoiceTermsPlaceholder()}</span>
									</div>
								</div>
							</div>

							<div class="form-grid">
								<div class="form-group">
									<span class="label">{m.onboarding_companyLogo()}</span>
									<div class="logo-upload">
										{#if logo}
											<div class="logo-preview">
												<img src={logo} alt="Logo" />
												<button type="button" class="btn btn-secondary" onclick={() => (logo = '')}>{m.common_remove()}</button>
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
						<div class="step-content">
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
						<div class="step-content complete-step">
							<div class="complete-icon">
								<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
									<path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
									<polyline points="22 4 12 14.01 9 11.01" />
								</svg>
							</div>

							<div class="quick-actions">
								<a href="/invoices?tenant={tenant.id}" class="quick-action-card featured">
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
		background:
			radial-gradient(circle at top left, rgba(37, 99, 235, 0.24), transparent 26rem),
			linear-gradient(135deg, rgba(15, 23, 42, 0.82), rgba(15, 23, 42, 0.72));
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 1000;
		padding: 1.5rem;
		backdrop-filter: blur(18px);
	}

	.onboarding-wizard {
		width: 100%;
		max-width: 1120px;
		max-height: 92vh;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
		padding: 1.75rem;
		border-radius: calc(var(--radius-lg) + 0.25rem);
		border: 1px solid rgba(255, 255, 255, 0.12);
		background:
			radial-gradient(circle at top right, rgba(37, 99, 235, 0.1), transparent 24rem),
			linear-gradient(180deg, rgba(255, 253, 248, 0.96), rgba(250, 246, 239, 0.98));
		box-shadow: 0 40px 120px rgba(15, 23, 42, 0.4);
	}

	.wizard-header {
		display: grid;
		grid-template-columns: minmax(0, 1.35fr) minmax(260px, 0.85fr);
		gap: 1.5rem;
		align-items: end;
	}

	.wizard-header h1 {
		font-family: var(--font-display);
		font-size: clamp(2.6rem, 5vw, 4.4rem);
		line-height: 0.94;
		letter-spacing: -0.04em;
		margin: 0;
	}

	.subtitle {
		max-width: 42rem;
		color: var(--color-text-muted);
		font-size: 1rem;
		margin-top: 0.85rem;
	}

	.header-copy {
		display: grid;
		gap: 1rem;
	}

	.header-eyebrow-row {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 0.65rem;
	}

	.header-eyebrow {
		display: inline-flex;
		align-items: center;
		padding: 0.35rem 0.7rem;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.08);
		color: #0f172a;
		font-size: 0.75rem;
		font-weight: 700;
		letter-spacing: 0.12em;
		text-transform: uppercase;
	}

	.header-eyebrow.subdued {
		background: rgba(255, 255, 255, 0.74);
		color: #475569;
	}

	.header-chip-row {
		display: flex;
		flex-wrap: wrap;
		gap: 0.6rem;
	}

	.header-chip {
		display: inline-flex;
		align-items: center;
		padding: 0.45rem 0.8rem;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.08);
		color: #475569;
		font-size: 0.8rem;
		font-weight: 600;
	}

	.header-chip.current {
		background: rgba(37, 99, 235, 0.14);
		color: var(--color-primary);
	}

	.header-progress-card {
		display: grid;
		gap: 1rem;
		padding: 1.25rem;
		border-radius: var(--radius-lg);
		background:
			radial-gradient(circle at top right, rgba(56, 189, 248, 0.16), transparent 12rem),
			linear-gradient(180deg, #0f172a 0%, #111827 100%);
		color: #f8fafc;
		box-shadow: 0 20px 50px rgba(15, 23, 42, 0.24);
	}

	.progress-topline {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1rem;
	}

	.progress-copy {
		display: grid;
		gap: 0.25rem;
	}

	.progress-copy span {
		font-size: 0.75rem;
		letter-spacing: 0.12em;
		text-transform: uppercase;
		color: rgba(226, 232, 240, 0.72);
	}

	.progress-copy strong {
		font-size: 1.15rem;
		font-weight: 700;
	}

	.progress-state {
		display: flex;
		align-items: center;
		padding: 0.35rem 0.7rem;
		border-radius: 999px;
		background: rgba(255, 255, 255, 0.1);
		font-size: 0.75rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}

	.progress-meter {
		position: relative;
		height: 0.55rem;
		border-radius: 999px;
		overflow: hidden;
		background: rgba(148, 163, 184, 0.24);
	}

	.progress-meter span {
		position: absolute;
		inset: 0 auto 0 0;
		border-radius: inherit;
		background: linear-gradient(90deg, #60a5fa, #38bdf8);
	}

	.progress-caption {
		color: rgba(226, 232, 240, 0.82);
		font-size: 0.92rem;
	}

	.wizard-stage {
		display: grid;
		grid-template-columns: minmax(260px, 320px) minmax(0, 1fr);
		gap: 1.5rem;
		align-items: start;
	}

	.wizard-sidebar {
		position: sticky;
		top: 0;
		display: grid;
		gap: 1rem;
		padding: 1.35rem;
		border-radius: var(--radius-lg);
		background:
			radial-gradient(circle at top right, rgba(59, 130, 246, 0.24), transparent 14rem),
			linear-gradient(180deg, #111827 0%, #0f172a 100%);
		color: #e2e8f0;
		box-shadow: 0 24px 55px rgba(15, 23, 42, 0.22);
	}

	.sidebar-current-step {
		display: grid;
		gap: 0.85rem;
	}

	.sidebar-pill-row {
		display: flex;
		align-items: center;
		gap: 0.5rem;
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
		background: rgba(96, 165, 250, 0.18);
		color: #bfdbfe;
	}

	.sidebar-step-type {
		background: rgba(148, 163, 184, 0.16);
		color: #cbd5e1;
	}

	.wizard-sidebar h2 {
		font-family: var(--font-display);
		font-size: 2rem;
		line-height: 0.95;
		margin: 0;
	}

	.sidebar-description {
		color: rgba(226, 232, 240, 0.72);
	}

	.sidebar-highlights {
		list-style: none;
		display: grid;
		gap: 0.6rem;
	}

	.sidebar-highlights li {
		display: flex;
		align-items: flex-start;
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

	.sidebar-step-plan {
		display: grid;
		gap: 0.75rem;
	}

	.sidebar-step-card {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr);
		gap: 0.75rem;
		padding: 0.9rem;
		border-radius: var(--radius-md);
		border: 1px solid rgba(148, 163, 184, 0.16);
		background: rgba(255, 255, 255, 0.04);
	}

	.sidebar-step-card.current {
		border-color: rgba(96, 165, 250, 0.42);
		background: rgba(37, 99, 235, 0.12);
	}

	.sidebar-step-card.complete {
		background: rgba(34, 197, 94, 0.08);
		border-color: rgba(74, 222, 128, 0.18);
	}

	.sidebar-step-index {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 2.1rem;
		height: 2.1rem;
		border-radius: 999px;
		background: rgba(255, 255, 255, 0.08);
		font-size: 0.78rem;
		font-weight: 700;
	}

	.sidebar-step-body {
		display: grid;
		gap: 0.15rem;
	}

	.sidebar-step-body strong {
		font-size: 0.92rem;
	}

	.sidebar-step-body p {
		font-size: 0.78rem;
		color: rgba(226, 232, 240, 0.68);
	}

	.sidebar-step-state {
		grid-column: 2;
		justify-self: start;
		display: inline-flex;
		align-items: center;
		padding: 0.25rem 0.55rem;
		border-radius: 999px;
		background: rgba(255, 255, 255, 0.08);
		font-size: 0.7rem;
		font-weight: 700;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}

	.sidebar-note {
		padding: 0.9rem 1rem;
		border-radius: var(--radius-md);
		background: rgba(255, 255, 255, 0.08);
		border: 1px solid rgba(148, 163, 184, 0.16);
	}

	.sidebar-note strong {
		display: block;
		margin-bottom: 0.35rem;
	}

	.sidebar-note p {
		font-size: 0.85rem;
		color: rgba(226, 232, 240, 0.68);
	}

	.wizard-content {
		display: grid;
		gap: 1rem;
	}

	.step-shell {
		display: grid;
		gap: 1.5rem;
		padding: 1.5rem;
		border-radius: var(--radius-lg);
		border: 1px solid rgba(30, 41, 59, 0.1);
		background:
			radial-gradient(circle at top right, rgba(37, 99, 235, 0.08), transparent 16rem),
			linear-gradient(180deg, rgba(255, 255, 255, 0.9), rgba(255, 252, 246, 0.96));
		box-shadow: var(--shadow-soft);
	}

	.step-shell-intro {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1rem;
		padding-bottom: 1.25rem;
		border-bottom: 1px solid rgba(30, 41, 59, 0.08);
	}

	.step-kicker {
		display: inline-flex;
		align-items: center;
		margin-bottom: 0.55rem;
		font-size: 0.74rem;
		font-weight: 700;
		letter-spacing: 0.12em;
		text-transform: uppercase;
		color: var(--color-primary);
	}

	.step-shell-intro h2 {
		font-family: var(--font-display);
		font-size: clamp(2rem, 4vw, 3rem);
		line-height: 0.96;
		letter-spacing: -0.04em;
		margin: 0;
	}

	.step-description {
		color: var(--color-text-muted);
		margin-top: 0.65rem;
		max-width: 42rem;
	}

	.step-type-badge {
		display: inline-flex;
		align-items: center;
		padding: 0.45rem 0.75rem;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.06);
		font-size: 0.75rem;
		font-weight: 700;
		letter-spacing: 0.1em;
		text-transform: uppercase;
		color: #475569;
	}

	.step-content {
		display: grid;
		gap: 1.25rem;
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

	.brand-preview-card {
		display: grid;
		gap: 1rem;
		padding: 1rem;
		border-radius: var(--radius-md);
		border: 1px solid rgba(30, 41, 59, 0.08);
		background: linear-gradient(180deg, rgba(255, 255, 255, 0.82), rgba(246, 249, 255, 0.96));
	}

	.brand-preview-header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1rem;
	}

	.brand-preview-mark {
		display: flex;
		align-items: center;
		gap: 0.8rem;
	}

	.brand-preview-swatch {
		display: inline-flex;
		width: 2.5rem;
		height: 2.5rem;
		border-radius: 0.9rem;
		box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.42);
	}

	.brand-preview-mark strong {
		display: block;
		font-size: 0.95rem;
	}

	.brand-preview-mark p {
		font-size: 0.8rem;
		color: var(--color-text-muted);
	}

	.brand-preview-code {
		font-family: var(--font-mono);
		font-size: 0.8rem;
		color: var(--color-text-muted);
	}

	.brand-preview-sheet {
		display: grid;
		grid-template-columns: 10px minmax(0, 1fr);
		overflow: hidden;
		border-radius: 1rem;
		background: rgba(248, 250, 252, 0.92);
		border: 1px solid rgba(30, 41, 59, 0.08);
	}

	.brand-preview-accent {
		min-height: 100%;
	}

	.brand-preview-copy {
		display: grid;
		gap: 0.4rem;
		padding: 1rem 1.1rem;
	}

	.brand-preview-copy strong {
		font-size: 1rem;
	}

	.brand-preview-copy p {
		color: var(--color-text-muted);
	}

	.brand-preview-copy span {
		font-size: 0.82rem;
		color: #475569;
	}

	.logo-upload {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.logo-preview {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 1rem;
	}

	.logo-preview img {
		max-width: 120px;
		max-height: 48px;
		object-fit: contain;
		border: 1px solid rgba(30, 41, 59, 0.08);
		border-radius: 0.85rem;
		padding: 0.55rem;
		background: rgba(255, 255, 255, 0.86);
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
		border: 1px solid rgba(30, 41, 59, 0.08);
		border-radius: 0.75rem;
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
		padding: 1.15rem;
		border: 1px solid rgba(30, 41, 59, 0.08);
		border-radius: var(--radius-md);
		background: rgba(255, 255, 255, 0.82);
		cursor: pointer;
		transition: all 0.2s;
		box-shadow: var(--shadow-card);
	}

	.type-btn:hover {
		border-color: rgba(37, 99, 235, 0.3);
		transform: translateY(-1px);
	}

	.type-btn.active {
		border-color: rgba(37, 99, 235, 0.45);
		background: linear-gradient(180deg, rgba(37, 99, 235, 0.12), rgba(224, 231, 255, 0.52));
	}

	.type-btn svg {
		color: var(--color-text-muted);
	}

	.type-btn.active svg {
		color: var(--color-primary);
	}

	.inline-note {
		margin-top: 1rem;
		padding: 0.9rem 1rem;
		border-radius: var(--radius-md);
		background: rgba(37, 99, 235, 0.08);
		border: 1px solid rgba(37, 99, 235, 0.1);
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}

	.complete-step {
		gap: 1.5rem;
	}

	.complete-icon {
		color: var(--color-success, #10b981);
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 5rem;
		height: 5rem;
		border-radius: 50%;
		background: rgba(34, 197, 94, 0.12);
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
		padding: 1rem 1.05rem;
		border: 1px solid rgba(30, 41, 59, 0.08);
		border-radius: var(--radius-md);
		text-decoration: none;
		color: inherit;
		transition: all 0.2s;
		background: rgba(255, 255, 255, 0.76);
	}

	.quick-action-card:hover {
		border-color: rgba(37, 99, 235, 0.32);
		background: rgba(239, 246, 255, 0.92);
		transform: translateY(-1px);
		text-decoration: none;
	}

	.quick-action-card.featured {
		grid-column: 1 / -1;
		padding: 1.15rem 1.2rem;
		background:
			radial-gradient(circle at top right, rgba(37, 99, 235, 0.12), transparent 12rem),
			linear-gradient(180deg, rgba(239, 246, 255, 0.92), rgba(255, 255, 255, 0.92));
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
		padding: 1.2rem 1.4rem 0;
		border-top: 1px solid rgba(30, 41, 59, 0.08);
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

	.onboarding-alert {
		margin-bottom: 0;
	}

	@media (max-width: 640px) {
		.onboarding-overlay {
			padding: 0.75rem;
		}

		.onboarding-wizard {
			padding: 1rem;
		}

		.wizard-stage {
			grid-template-columns: 1fr;
		}

		.wizard-sidebar {
			position: static;
		}

		.step-shell-intro,
		.wizard-footer,
		.progress-topline {
			flex-direction: column;
			align-items: flex-start;
		}

		.form-grid {
			grid-template-columns: 1fr;
		}

		.form-group.full-width {
			grid-column: 1;
		}

		.contact-type-selector,
		.quick-actions {
			grid-template-columns: 1fr;
		}

		.quick-action-card.featured {
			grid-column: auto;
		}
	}

	@media (max-width: 900px) {
		.wizard-header {
			grid-template-columns: 1fr;
		}
	}
</style>
