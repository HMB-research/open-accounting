<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { api, type Tenant, type TenantSettings } from '$lib/api';
	import * as m from '$lib/paraglide/messages.js';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let tenant = $state<Tenant | null>(null);
	let isLoading = $state(true);
	let isSaving = $state(false);
	let error = $state('');
	let success = $state('');

	// Form state - will be populated from tenant
	let companyName = $state('');
	let regCode = $state('');
	let vatNumber = $state('');
	let address = $state('');
	let email = $state('');
	let phone = $state('');
	let logo = $state('');
	let pdfPrimaryColor = $state('#4f46e5');
	let pdfFooterText = $state('');
	let bankDetails = $state('');
	let invoiceTerms = $state('');
	let timezone = $state('');
	let dateFormat = $state('');
	let decimalSep = $state('');
	let thousandsSep = $state('');
	let fiscalYearStart = $state(1);

	onMount(async () => {
		if (!tenantId) {
			error = m.settings_noTenantSelected();
			isLoading = false;
			return;
		}

		try {
			tenant = await api.getTenant(tenantId);
			populateForm(tenant);
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_loadFailed();
		} finally {
			isLoading = false;
		}
	});

	function populateForm(t: Tenant) {
		companyName = t.name || '';
		regCode = t.settings?.reg_code || '';
		vatNumber = t.settings?.vat_number || '';
		address = t.settings?.address || '';
		email = t.settings?.email || '';
		phone = t.settings?.phone || '';
		logo = t.settings?.logo || '';
		pdfPrimaryColor = t.settings?.pdf_primary_color || '#4f46e5';
		pdfFooterText = t.settings?.pdf_footer_text || '';
		bankDetails = t.settings?.bank_details || '';
		invoiceTerms = t.settings?.invoice_terms || '';
		timezone = t.settings?.timezone || 'Europe/Tallinn';
		dateFormat = t.settings?.date_format || 'DD.MM.YYYY';
		decimalSep = t.settings?.decimal_sep || ',';
		thousandsSep = t.settings?.thousands_sep || ' ';
		fiscalYearStart = t.settings?.fiscal_year_start_month || 1;
	}

	async function saveSettings(e: Event) {
		e.preventDefault();
		isSaving = true;
		error = '';
		success = '';

		try {
			const settings: Partial<TenantSettings> = {
				reg_code: regCode,
				vat_number: vatNumber,
				address: address,
				email: email,
				phone: phone,
				logo: logo,
				pdf_primary_color: pdfPrimaryColor,
				pdf_footer_text: pdfFooterText,
				bank_details: bankDetails,
				invoice_terms: invoiceTerms,
				timezone: timezone,
				date_format: dateFormat,
				decimal_sep: decimalSep,
				thousands_sep: thousandsSep,
				fiscal_year_start_month: fiscalYearStart
			};

			tenant = await api.updateTenant(tenantId, {
				name: companyName,
				settings
			});
			success = m.settings_settingsSaved();
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_saveFailed();
		} finally {
			isSaving = false;
		}
	}

	function handleLogoUpload(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;

		if (file.size > 500 * 1024) {
			error = m.settings_logoTooLarge();
			return;
		}

		const reader = new FileReader();
		reader.onload = () => {
			logo = reader.result as string;
		};
		reader.readAsDataURL(file);
	}

	function removeLogo() {
		logo = '';
	}

	const timezones = [
		'Europe/Tallinn',
		'Europe/Helsinki',
		'Europe/Riga',
		'Europe/Vilnius',
		'Europe/London',
		'Europe/Paris',
		'Europe/Berlin',
		'America/New_York',
		'America/Los_Angeles',
		'Asia/Tokyo',
		'Asia/Singapore',
		'UTC'
	];

	const dateFormats = [
		{ value: 'DD.MM.YYYY', label: '31.12.2024' },
		{ value: 'MM/DD/YYYY', label: '12/31/2024' },
		{ value: 'YYYY-MM-DD', label: '2024-12-31' },
		{ value: 'DD/MM/YYYY', label: '31/12/2024' }
	];

	function getMonthLabel(index: number): string {
		switch (index) {
			case 0: return m.months_january();
			case 1: return m.months_february();
			case 2: return m.months_march();
			case 3: return m.months_april();
			case 4: return m.months_may();
			case 5: return m.months_june();
			case 6: return m.months_july();
			case 7: return m.months_august();
			case 8: return m.months_september();
			case 9: return m.months_october();
			case 10: return m.months_november();
			case 11: return m.months_december();
			default: return '';
		}
	}

	const monthIndices = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11];
</script>

<svelte:head>
	<title>{m.settings_companySettings()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="page-header">
		<div>
			<a href="/settings?tenant={tenantId}" class="back-link">&larr; {m.settings_backToSettings()}</a>
			<h1>{m.settings_companySettings()}</h1>
		</div>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if success}
		<div class="alert alert-success">{success}</div>
	{/if}

	{#if isLoading}
		<p>{m.common_loading()}</p>
	{:else if !tenantId}
		<div class="card empty-state">
			<p>{m.settings_selectTenantDashboard()} <a href="/dashboard">{m.dashboard_title()}</a>.</p>
		</div>
	{:else}
		<form onsubmit={saveSettings}>
			<!-- Company Information -->
			<section class="card settings-section">
				<h2>{m.settings_companyInfo()}</h2>
				<div class="form-grid">
					<div class="form-group">
						<label class="label" for="companyName">{m.settings_companyName()}</label>
						<input class="input" type="text" id="companyName" bind:value={companyName} required />
					</div>
					<div class="form-group">
						<label class="label" for="regCode">{m.settings_regCode()}</label>
						<input
							class="input"
							type="text"
							id="regCode"
							bind:value={regCode}
							placeholder="e.g. 12345678"
						/>
					</div>
					<div class="form-group">
						<label class="label" for="vatNumber">{m.settings_vatNumber()}</label>
						<input
							class="input"
							type="text"
							id="vatNumber"
							bind:value={vatNumber}
							placeholder="e.g. EE123456789"
						/>
					</div>
					<div class="form-group full-width">
						<label class="label" for="address">{m.settings_address()}</label>
						<textarea
							class="input"
							id="address"
							bind:value={address}
							rows="3"
							placeholder={m.settings_addressPlaceholder()}
						></textarea>
					</div>
				</div>
			</section>

			<!-- Contact Information -->
			<section class="card settings-section">
				<h2>{m.settings_contactInfo()}</h2>
				<div class="form-grid">
					<div class="form-group">
						<label class="label" for="email">{m.settings_email()}</label>
						<input
							class="input"
							type="email"
							id="email"
							bind:value={email}
							placeholder="info@company.com"
						/>
					</div>
					<div class="form-group">
						<label class="label" for="phone">{m.settings_phone()}</label>
						<input class="input" type="tel" id="phone" bind:value={phone} placeholder="+372 5555 5555" />
					</div>
				</div>
			</section>

			<!-- Branding -->
			<section class="card settings-section">
				<h2>{m.settings_brandingInvoice()}</h2>
				<div class="form-grid">
					<div class="form-group">
						<label class="label">{m.settings_logo()}</label>
						<div class="logo-upload">
							{#if logo}
								<div class="logo-preview">
									<img src={logo} alt="Company logo" />
									<button type="button" class="btn btn-sm btn-danger" onclick={removeLogo}>
										{m.settings_removeLogo()}
									</button>
								</div>
							{:else}
								<div class="logo-placeholder">
									<input
										type="file"
										accept="image/png,image/jpeg,image/svg+xml"
										onchange={handleLogoUpload}
										id="logoUpload"
									/>
									<label for="logoUpload" class="btn btn-secondary">{m.settings_uploadLogo()}</label>
									<span class="help-text">{m.settings_logoMaxSize()}</span>
								</div>
							{/if}
						</div>
					</div>
					<div class="form-group">
						<label class="label" for="pdfPrimaryColor">{m.settings_primaryColor()}</label>
						<div class="color-input">
							<input type="color" id="pdfPrimaryColor" bind:value={pdfPrimaryColor} />
							<input
								class="input"
								type="text"
								bind:value={pdfPrimaryColor}
								placeholder="#4f46e5"
								pattern="^#[0-9A-Fa-f]{6}$"
							/>
						</div>
						<span class="help-text">{m.settings_usedInPdf()}</span>
					</div>
					<div class="form-group full-width">
						<label class="label" for="bankDetails">{m.settings_bankDetails()}</label>
						<textarea
							class="input"
							id="bankDetails"
							bind:value={bankDetails}
							rows="2"
							placeholder={m.settings_bankDetailsPlaceholder()}
						></textarea>
						<span class="help-text">{m.settings_displayedOnInvoices()}</span>
					</div>
					<div class="form-group full-width">
						<label class="label" for="invoiceTerms">{m.settings_invoiceTerms()}</label>
						<textarea
							class="input"
							id="invoiceTerms"
							bind:value={invoiceTerms}
							rows="2"
							placeholder={m.settings_invoiceTermsPlaceholder()}
						></textarea>
					</div>
					<div class="form-group full-width">
						<label class="label" for="pdfFooterText">{m.settings_pdfFooterText()}</label>
						<input
							class="input"
							type="text"
							id="pdfFooterText"
							bind:value={pdfFooterText}
							placeholder={m.settings_pdfFooterPlaceholder()}
						/>
					</div>
				</div>
			</section>

			<!-- Regional Settings -->
			<section class="card settings-section">
				<h2>{m.settings_regionalSettings()}</h2>
				<div class="form-grid">
					<div class="form-group">
						<label class="label" for="currency">{m.settings_currency()}</label>
						<input
							class="input"
							type="text"
							id="currency"
							value={tenant?.settings?.default_currency || 'EUR'}
							disabled
						/>
						<span class="help-text">{m.settings_currencyNoChange()}</span>
					</div>
					<div class="form-group">
						<label class="label" for="timezone">{m.settings_timezone()}</label>
						<select class="input" id="timezone" bind:value={timezone}>
							{#each timezones as tz}
								<option value={tz}>{tz}</option>
							{/each}
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="dateFormat">{m.settings_dateFormat()}</label>
						<select class="input" id="dateFormat" bind:value={dateFormat}>
							{#each dateFormats as fmt}
								<option value={fmt.value}>{fmt.label}</option>
							{/each}
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="fiscalYearStart">{m.settings_fiscalYearStart()}</label>
						<select class="input" id="fiscalYearStart" bind:value={fiscalYearStart}>
							{#each monthIndices as i}
								<option value={i + 1}>{getMonthLabel(i)}</option>
							{/each}
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="decimalSep">{m.settings_decimalSeparator()}</label>
						<select class="input" id="decimalSep" bind:value={decimalSep}>
							<option value=",">{m.settings_commaDecimal()}</option>
							<option value=".">{m.settings_periodDecimal()}</option>
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="thousandsSep">{m.settings_thousandsSeparator()}</label>
						<select class="input" id="thousandsSep" bind:value={thousandsSep}>
							<option value=" ">{m.settings_spaceThousands()}</option>
							<option value=",">{m.settings_commaThousands()}</option>
							<option value=".">{m.settings_periodThousands()}</option>
							<option value="">{m.settings_noneThousands()}</option>
						</select>
					</div>
				</div>
			</section>

			<div class="form-actions">
				<button type="submit" class="btn btn-primary" disabled={isSaving}>
					{isSaving ? m.settings_saving() : m.settings_saveSettings()}
				</button>
			</div>
		</form>
	{/if}
</div>

<style>
	.back-link {
		display: inline-block;
		margin-bottom: 0.5rem;
		color: var(--color-text-muted);
		text-decoration: none;
		font-size: 0.875rem;
	}

	.back-link:hover {
		color: var(--color-primary);
	}

	h1 {
		font-size: 1.75rem;
		margin: 0;
	}

	.settings-section {
		margin-bottom: 1.5rem;
	}

	.settings-section h2 {
		font-size: 1.25rem;
		margin: 0 0 1rem;
		padding-bottom: 0.5rem;
		border-bottom: 1px solid var(--color-border);
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
		max-width: 150px;
		max-height: 60px;
		object-fit: contain;
		border: 1px solid var(--color-border);
		border-radius: 4px;
		padding: 0.5rem;
		background: white;
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

	.form-actions {
		display: flex;
		justify-content: flex-end;
		padding-top: 1rem;
	}

	.empty-state {
		text-align: center;
		padding: 3rem;
	}

	.alert-success {
		background: var(--color-success-bg, #d1fae5);
		color: var(--color-success, #10b981);
		padding: 1rem;
		border-radius: 0.5rem;
		margin-bottom: 1rem;
	}

	@media (max-width: 640px) {
		.form-grid {
			grid-template-columns: 1fr;
		}

		.form-group.full-width {
			grid-column: 1;
		}

		.color-input {
			flex-direction: column;
			align-items: stretch;
		}

		.color-input input[type='color'] {
			width: 100%;
		}
	}
</style>
