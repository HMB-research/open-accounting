<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { api, type Tenant, type TenantSettings } from '$lib/api';

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
			error = 'No tenant selected. Please select a tenant from the dashboard.';
			isLoading = false;
			return;
		}

		try {
			tenant = await api.getTenant(tenantId);
			populateForm(tenant);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load company settings';
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
			success = 'Settings saved successfully';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to save settings';
		} finally {
			isSaving = false;
		}
	}

	function handleLogoUpload(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;

		if (file.size > 500 * 1024) {
			error = 'Logo file must be less than 500KB';
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

	const months = [
		'January',
		'February',
		'March',
		'April',
		'May',
		'June',
		'July',
		'August',
		'September',
		'October',
		'November',
		'December'
	];
</script>

<svelte:head>
	<title>Company Settings - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="page-header">
		<div>
			<a href="/settings?tenant={tenantId}" class="back-link">&larr; Back to Settings</a>
			<h1>Company Settings</h1>
		</div>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if success}
		<div class="alert alert-success">{success}</div>
	{/if}

	{#if isLoading}
		<p>Loading...</p>
	{:else if !tenantId}
		<div class="card empty-state">
			<p>Please select a tenant from the <a href="/dashboard">dashboard</a>.</p>
		</div>
	{:else}
		<form onsubmit={saveSettings}>
			<!-- Company Information -->
			<section class="card settings-section">
				<h2>Company Information</h2>
				<div class="form-grid">
					<div class="form-group">
						<label class="label" for="companyName">Company Name</label>
						<input class="input" type="text" id="companyName" bind:value={companyName} required />
					</div>
					<div class="form-group">
						<label class="label" for="regCode">Registration Code</label>
						<input
							class="input"
							type="text"
							id="regCode"
							bind:value={regCode}
							placeholder="e.g. 12345678"
						/>
					</div>
					<div class="form-group">
						<label class="label" for="vatNumber">VAT Number</label>
						<input
							class="input"
							type="text"
							id="vatNumber"
							bind:value={vatNumber}
							placeholder="e.g. EE123456789"
						/>
					</div>
					<div class="form-group full-width">
						<label class="label" for="address">Address</label>
						<textarea
							class="input"
							id="address"
							bind:value={address}
							rows="3"
							placeholder="Street address, City, Postal code, Country"
						></textarea>
					</div>
				</div>
			</section>

			<!-- Contact Information -->
			<section class="card settings-section">
				<h2>Contact Information</h2>
				<div class="form-grid">
					<div class="form-group">
						<label class="label" for="email">Email</label>
						<input
							class="input"
							type="email"
							id="email"
							bind:value={email}
							placeholder="info@company.com"
						/>
					</div>
					<div class="form-group">
						<label class="label" for="phone">Phone</label>
						<input class="input" type="tel" id="phone" bind:value={phone} placeholder="+372 5555 5555" />
					</div>
				</div>
			</section>

			<!-- Branding -->
			<section class="card settings-section">
				<h2>Branding & Invoice Settings</h2>
				<div class="form-grid">
					<div class="form-group">
						<label class="label">Company Logo</label>
						<div class="logo-upload">
							{#if logo}
								<div class="logo-preview">
									<img src={logo} alt="Company logo" />
									<button type="button" class="btn btn-sm btn-danger" onclick={removeLogo}>
										Remove
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
									<label for="logoUpload" class="btn btn-secondary">Upload Logo</label>
									<span class="help-text">PNG, JPG or SVG, max 500KB</span>
								</div>
							{/if}
						</div>
					</div>
					<div class="form-group">
						<label class="label" for="pdfPrimaryColor">Primary Color</label>
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
						<span class="help-text">Used in PDF invoices and branding</span>
					</div>
					<div class="form-group full-width">
						<label class="label" for="bankDetails">Bank Details</label>
						<textarea
							class="input"
							id="bankDetails"
							bind:value={bankDetails}
							rows="2"
							placeholder="Bank name, IBAN, SWIFT/BIC"
						></textarea>
						<span class="help-text">Displayed on invoices</span>
					</div>
					<div class="form-group full-width">
						<label class="label" for="invoiceTerms">Invoice Terms</label>
						<textarea
							class="input"
							id="invoiceTerms"
							bind:value={invoiceTerms}
							rows="2"
							placeholder="Payment terms and conditions"
						></textarea>
					</div>
					<div class="form-group full-width">
						<label class="label" for="pdfFooterText">PDF Footer Text</label>
						<input
							class="input"
							type="text"
							id="pdfFooterText"
							bind:value={pdfFooterText}
							placeholder="Custom footer text for PDF invoices"
						/>
					</div>
				</div>
			</section>

			<!-- Regional Settings -->
			<section class="card settings-section">
				<h2>Regional Settings</h2>
				<div class="form-grid">
					<div class="form-group">
						<label class="label" for="currency">Currency</label>
						<input
							class="input"
							type="text"
							id="currency"
							value={tenant?.settings?.default_currency || 'EUR'}
							disabled
						/>
						<span class="help-text">Currency cannot be changed after creation</span>
					</div>
					<div class="form-group">
						<label class="label" for="timezone">Timezone</label>
						<select class="input" id="timezone" bind:value={timezone}>
							{#each timezones as tz}
								<option value={tz}>{tz}</option>
							{/each}
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="dateFormat">Date Format</label>
						<select class="input" id="dateFormat" bind:value={dateFormat}>
							{#each dateFormats as fmt}
								<option value={fmt.value}>{fmt.label}</option>
							{/each}
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="fiscalYearStart">Fiscal Year Start</label>
						<select class="input" id="fiscalYearStart" bind:value={fiscalYearStart}>
							{#each months as month, i}
								<option value={i + 1}>{month}</option>
							{/each}
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="decimalSep">Decimal Separator</label>
						<select class="input" id="decimalSep" bind:value={decimalSep}>
							<option value=",">Comma (1.234,56)</option>
							<option value=".">Period (1,234.56)</option>
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="thousandsSep">Thousands Separator</label>
						<select class="input" id="thousandsSep" bind:value={thousandsSep}>
							<option value=" ">Space (1 234)</option>
							<option value=",">Comma (1,234)</option>
							<option value=".">Period (1.234)</option>
							<option value="">None (1234)</option>
						</select>
					</div>
				</div>
			</section>

			<div class="form-actions">
				<button type="submit" class="btn btn-primary" disabled={isSaving}>
					{isSaving ? 'Saving...' : 'Save Settings'}
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
