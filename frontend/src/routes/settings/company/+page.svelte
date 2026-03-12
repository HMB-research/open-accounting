<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { api, type ClosePeriodRequest, type PeriodCloseEvent, type ReopenPeriodRequest, type Tenant, type TenantSettings } from '$lib/api';
	import WorkflowHero, { type WorkflowHeroAction, type WorkflowHeroAside, type WorkflowHeroStat } from '$lib/components/WorkflowHero.svelte';
	import * as m from '$lib/paraglide/messages.js';

	let tenantId = $derived($page.url.searchParams.get('tenant') || '');
	let tenant = $state<Tenant | null>(null);
	let periodCloseEvents = $state<PeriodCloseEvent[]>([]);
	let isLoading = $state(true);
	let isSaving = $state(false);
	let isHistoryLoading = $state(false);
	let isClosingPeriod = $state(false);
	let isReopeningPeriod = $state(false);
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
	let closePeriodEndDate = $state('');
	let closeNote = $state('');
	let reopenPeriodEndDate = $state('');
	let reopenNote = $state('');

	onMount(async () => {
		if (!tenantId) {
			error = m.settings_noTenantSelected();
			isLoading = false;
			return;
		}

		try {
			await loadTenantWorkspace();
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_loadFailed();
		} finally {
			isLoading = false;
		}
	});

	async function loadTenantWorkspace() {
		if (!tenantId) {
			return;
		}

		const [loadedTenant, loadedHistory] = await Promise.all([
			api.getTenant(tenantId),
			api.listPeriodCloseEvents(tenantId, 20)
		]);

		tenant = loadedTenant;
		periodCloseEvents = loadedHistory;
		populateForm(loadedTenant);
		populatePeriodActions(loadedTenant);
	}

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

	function populatePeriodActions(t: Tenant) {
		closePeriodEndDate = getSuggestedCloseDate(t.settings?.period_lock_date || null);
		reopenPeriodEndDate = t.settings?.period_lock_date || '';
		closeNote = '';
		reopenNote = '';
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
			if (tenant) {
				populateForm(tenant);
				populatePeriodActions(tenant);
			}
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

	async function refreshPeriodCloseHistory() {
		if (!tenantId) {
			return;
		}

		isHistoryLoading = true;
		try {
			periodCloseEvents = await api.listPeriodCloseEvents(tenantId, 20);
		} finally {
			isHistoryLoading = false;
		}
	}

	async function submitClosePeriod(e?: Event) {
		e?.preventDefault();
		if (!tenantId) {
			return;
		}

		const payload: ClosePeriodRequest = {
			period_end_date: closePeriodEndDate,
			note: closeNote.trim() || undefined
		};

		isClosingPeriod = true;
		error = '';
		success = '';

		try {
			const result = await api.closePeriod(tenantId, payload);
			tenant = result.tenant;
			populateForm(result.tenant);
			populatePeriodActions(result.tenant);
			await refreshPeriodCloseHistory();
			success = m.settings_periodCloseSuccess();
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_saveFailed();
		} finally {
			isClosingPeriod = false;
		}
	}

	async function submitReopenPeriod(e?: Event) {
		e?.preventDefault();
		if (!tenantId) {
			return;
		}

		const payload: ReopenPeriodRequest = {
			period_end_date: reopenPeriodEndDate,
			note: reopenNote.trim()
		};

		isReopeningPeriod = true;
		error = '';
		success = '';

		try {
			const result = await api.reopenPeriod(tenantId, payload);
			tenant = result.tenant;
			populateForm(result.tenant);
			populatePeriodActions(result.tenant);
			await refreshPeriodCloseHistory();
			success = m.settings_periodReopenSuccess();
		} catch (err) {
			error = err instanceof Error ? err.message : m.errors_saveFailed();
		} finally {
			isReopeningPeriod = false;
		}
	}

	function parseDateValue(value: string | null | undefined): Date | null {
		if (!value) {
			return null;
		}

		const [year, month, day] = value.split('-').map((part) => Number(part));
		if (!year || !month || !day) {
			return null;
		}

		return new Date(Date.UTC(year, month - 1, day));
	}

	function formatIsoDate(value: Date): string {
		const year = value.getUTCFullYear();
		const month = String(value.getUTCMonth() + 1).padStart(2, '0');
		const day = String(value.getUTCDate()).padStart(2, '0');
		return `${year}-${month}-${day}`;
	}

	function monthEndOffset(value: Date, monthOffset: number): Date {
		return new Date(Date.UTC(value.getUTCFullYear(), value.getUTCMonth() + monthOffset + 1, 0));
	}

	function getSuggestedCloseDate(periodLockDate: string | null): string {
		const currentLock = parseDateValue(periodLockDate);
		if (currentLock) {
			return formatIsoDate(monthEndOffset(currentLock, 1));
		}

		const today = new Date();
		return formatIsoDate(new Date(Date.UTC(today.getUTCFullYear(), today.getUTCMonth(), 0)));
	}

	function formatDateLabel(value: string | null | undefined): string {
		const parsed = parseDateValue(value);
		if (!parsed) {
			return 'N/A';
		}

		return new Intl.DateTimeFormat(undefined, {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			timeZone: 'UTC'
		}).format(parsed);
	}

	function formatDateTimeLabel(value: string): string {
		const parsed = new Date(value);
		if (Number.isNaN(parsed.getTime())) {
			return value;
		}

		return new Intl.DateTimeFormat(undefined, {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		}).format(parsed);
	}

	function getEventTitle(event: PeriodCloseEvent): string {
		return event.action === 'reopen' ? m.settings_periodHistoryReopened() : m.settings_periodHistoryClosed();
	}

	function getEventBadgeClass(event: PeriodCloseEvent): string {
		return event.action === 'reopen' ? 'badge badge-warning' : 'badge badge-success';
	}

	const currentPeriodLockDate = $derived(tenant?.settings?.period_lock_date || '');
	const suggestedCloseDateLabel = $derived(formatDateLabel(closePeriodEndDate));
	const closeStatusLabel = $derived(
		currentPeriodLockDate
			? m.settings_periodClosedThrough({ date: formatDateLabel(currentPeriodLockDate) })
			: m.settings_periodOpenStatus()
	);

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
	const selectedFiscalYearLabel = $derived(getMonthLabel(Math.max(0, fiscalYearStart - 1)));
	const heroStats = $derived.by<WorkflowHeroStat[]>(() => [
		{
			label: m.settings_regCode(),
			value: regCode || m.common_notSet()
		},
		{
			label: m.settings_vatNumber(),
			value: vatNumber || m.common_notSet()
		},
		{
			label: m.settings_fiscalYearStart(),
			value: selectedFiscalYearLabel
		},
		{
			label: m.settings_periodLockDate(),
			value: currentPeriodLockDate ? formatDateLabel(currentPeriodLockDate) : m.settings_periodOpenStatus(),
			tone: currentPeriodLockDate ? 'warning' : 'success'
		}
	]);
	const heroActions = $derived.by<WorkflowHeroAction[]>(() => [
		{
			label: isSaving ? m.settings_saving() : m.settings_saveSettings(),
			type: 'submit',
			form: 'company-settings-form',
			disabled: isSaving
		},
		{
			label: m.nav_invoices(),
			variant: 'secondary',
			href: tenantId ? `/invoices?tenant=${tenantId}` : '/invoices'
		}
	]);
	const heroAside = $derived.by<WorkflowHeroAside>(() => ({
		kicker: m.settings_accountingControls(),
		title: m.settings_heroAsideTitle(),
		body: m.settings_heroAsideDesc(),
		linkLabel: m.settings_periodHistoryTitle(),
		href: tenantId ? `/settings/company?tenant=${tenantId}#period-history` : '/settings/company#period-history',
		items: [m.settings_heroAsideItemOne(), m.settings_heroAsideItemTwo()]
	}));
</script>

<svelte:head>
	<title>{m.settings_companySettings()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<WorkflowHero
		backHref={tenantId ? `/settings?tenant=${tenantId}` : '/settings'}
		backLabel={m.settings_backToSettings()}
		eyebrow={m.settings_companyInfo()}
		title={tenant?.name || m.settings_companySettings()}
		description={m.settings_heroDesc()}
		badgeLabel={closeStatusLabel}
		badgeTone={currentPeriodLockDate ? 'warning' : 'success'}
		actions={heroActions}
		stats={heroStats}
		aside={heroAside}
	/>

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
		<form id="company-settings-form" onsubmit={saveSettings}>
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
						<span class="label">{m.settings_logo()}</span>
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

			<section class="card settings-section">
				<div class="section-header">
					<div>
						<h2>{m.settings_accountingControls()}</h2>
						<p class="section-description">{m.settings_periodCloseIntro()}</p>
					</div>
					<span class="status-pill" class:status-pill-open={!currentPeriodLockDate}>
						{closeStatusLabel}
					</span>
				</div>

				<div class="period-summary-grid">
					<div class="period-summary-card">
						<span class="summary-label">{m.settings_periodLockDate()}</span>
						<strong>{currentPeriodLockDate ? formatDateLabel(currentPeriodLockDate) : m.settings_periodOpenStatus()}</strong>
						<span class="help-text">{m.settings_periodLockHelp()}</span>
					</div>
					<div class="period-summary-card">
						<span class="summary-label">{m.settings_periodSuggestedClose()}</span>
						<strong>{suggestedCloseDateLabel}</strong>
						<span class="help-text">{m.settings_periodSuggestedCloseHelp()}</span>
					</div>
				</div>

				<div class="period-action-grid">
					<div class="period-form">
						<div class="period-form-header">
							<h3>{m.settings_closePeriodTitle()}</h3>
							<p>{m.settings_closePeriodHelp()}</p>
						</div>
						<div class="form-grid">
							<div class="form-group">
								<label class="label" for="closePeriodEndDate">{m.settings_periodEndDate()}</label>
								<input class="input" type="date" id="closePeriodEndDate" bind:value={closePeriodEndDate} required />
							</div>
							<div class="form-group full-width">
								<label class="label" for="closeNote">{m.settings_closeNote()}</label>
								<textarea
									class="input"
									id="closeNote"
									bind:value={closeNote}
									rows="3"
									placeholder={m.settings_closeNotePlaceholder()}
								></textarea>
							</div>
						</div>
						<div class="period-form-actions">
							<button type="button" class="btn btn-primary" disabled={isClosingPeriod} onclick={submitClosePeriod}>
								{isClosingPeriod ? m.settings_periodCloseSubmitting() : m.settings_closePeriodAction()}
							</button>
						</div>
					</div>

					<div class="period-form period-form-secondary">
						<div class="period-form-header">
							<h3>{m.settings_reopenPeriodTitle()}</h3>
							<p>{m.settings_reopenPeriodHelp()}</p>
						</div>
						<div class="form-grid">
							<div class="form-group">
								<label class="label" for="reopenPeriodEndDate">{m.settings_periodEndDate()}</label>
								<input class="input" type="date" id="reopenPeriodEndDate" bind:value={reopenPeriodEndDate} required />
							</div>
							<div class="form-group full-width">
								<label class="label" for="reopenNote">{m.settings_reopenNote()}</label>
								<textarea
									class="input"
									id="reopenNote"
									bind:value={reopenNote}
									rows="3"
									placeholder={m.settings_reopenNotePlaceholder()}
									required
								></textarea>
							</div>
						</div>
						<div class="period-form-actions">
							<button
								type="button"
								class="btn btn-secondary"
								disabled={isReopeningPeriod || !currentPeriodLockDate}
								onclick={submitReopenPeriod}
							>
								{isReopeningPeriod ? m.settings_periodReopenSubmitting() : m.settings_reopenPeriodAction()}
							</button>
						</div>
					</div>
				</div>

				<div class="period-history" id="period-history">
					<div class="section-header compact">
						<div>
							<h3>{m.settings_periodHistoryTitle()}</h3>
							<p class="section-description">{m.settings_periodHistoryHelp()}</p>
						</div>
					</div>

					{#if isHistoryLoading}
						<p>{m.common_loading()}</p>
					{:else if periodCloseEvents.length === 0}
						<div class="empty-history">
							<p>{m.settings_periodHistoryEmpty()}</p>
						</div>
					{:else}
						<ul class="history-list">
							{#each periodCloseEvents as event}
								<li class="history-item">
									<div class="history-main">
										<div class="history-heading">
											<span class={getEventBadgeClass(event)}>{getEventTitle(event)}</span>
											<strong>{formatDateLabel(event.period_end_date)}</strong>
										</div>
										<p class="history-meta">
											{event.close_kind === 'year_end' ? m.settings_periodYearEnd() : m.settings_periodMonthEnd()}
											•
											{formatDateTimeLabel(event.created_at)}
										</p>
									</div>
									<div class="history-lock-range">
										<span>{m.settings_periodHistoryBefore()} {event.lock_date_before ? formatDateLabel(event.lock_date_before) : m.settings_periodUnlockedLabel()}</span>
										<span>{m.settings_periodHistoryAfter()} {event.lock_date_after ? formatDateLabel(event.lock_date_after) : m.settings_periodUnlockedLabel()}</span>
									</div>
									{#if event.note}
										<p class="history-note">{event.note}</p>
									{/if}
								</li>
							{/each}
						</ul>
					{/if}
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
	.settings-section {
		margin-bottom: 1.5rem;
	}

	.settings-section h2 {
		font-size: 1.25rem;
		margin: 0 0 1rem;
		padding-bottom: 0.5rem;
		border-bottom: 1px solid var(--color-border);
	}

	.section-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 1rem;
		margin-bottom: 1rem;
	}

	.section-header.compact {
		margin-bottom: 0.75rem;
	}

	.section-header.compact h3 {
		margin: 0;
	}

	.section-description {
		margin: 0.25rem 0 0;
		color: var(--color-text-muted);
		font-size: 0.9rem;
		line-height: 1.5;
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

	.status-pill {
		display: inline-flex;
		align-items: center;
		padding: 0.5rem 0.875rem;
		border-radius: 999px;
		background: color-mix(in srgb, var(--color-success, #16a34a) 12%, white);
		color: var(--color-success, #166534);
		font-size: 0.85rem;
		font-weight: 600;
	}

	.status-pill-open {
		background: color-mix(in srgb, var(--color-info, #2563eb) 12%, white);
		color: var(--color-info, #1d4ed8);
	}

	.period-summary-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 1rem;
		margin-bottom: 1rem;
	}

	.period-summary-card {
		padding: 1rem;
		border: 1px solid var(--color-border);
		border-radius: 0.75rem;
		background: color-mix(in srgb, var(--color-surface, white) 92%, var(--color-primary, #1d4ed8) 8%);
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
	}

	.summary-label {
		font-size: 0.8rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--color-text-muted);
	}

	.period-action-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 1rem;
		margin-bottom: 1.5rem;
	}

	.period-form {
		padding: 1rem;
		border: 1px solid var(--color-border);
		border-radius: 0.75rem;
		background: var(--color-surface, white);
	}

	.period-form-secondary {
		background: color-mix(in srgb, var(--color-warning, #d97706) 5%, white);
	}

	.period-form-header {
		margin-bottom: 1rem;
	}

	.period-form-header h3 {
		margin: 0 0 0.25rem;
	}

	.period-form-header p {
		margin: 0;
		color: var(--color-text-muted);
		font-size: 0.9rem;
	}

	.period-form-actions {
		display: flex;
		justify-content: flex-end;
		margin-top: 1rem;
	}

	.period-history {
		border-top: 1px solid var(--color-border);
		padding-top: 1rem;
	}

	.history-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	.history-item {
		border: 1px solid var(--color-border);
		border-radius: 0.75rem;
		padding: 1rem;
		background: var(--color-surface, white);
	}

	.history-main {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 1rem;
	}

	.history-heading {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		flex-wrap: wrap;
	}

	.history-meta {
		margin: 0.35rem 0 0;
		color: var(--color-text-muted);
		font-size: 0.85rem;
	}

	.history-lock-range {
		display: flex;
		flex-wrap: wrap;
		gap: 1rem;
		margin-top: 0.75rem;
		font-size: 0.9rem;
		color: var(--color-text-muted);
	}

	.history-note {
		margin: 0.75rem 0 0;
		padding-top: 0.75rem;
		border-top: 1px solid var(--color-border);
	}

	.empty-history {
		padding: 1rem;
		border: 1px dashed var(--color-border);
		border-radius: 0.75rem;
		color: var(--color-text-muted);
	}

	.badge {
		display: inline-flex;
		align-items: center;
		padding: 0.2rem 0.6rem;
		border-radius: 999px;
		font-size: 0.75rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	.badge-success {
		background: color-mix(in srgb, var(--color-success, #16a34a) 14%, white);
		color: var(--color-success, #166534);
	}

	.badge-warning {
		background: color-mix(in srgb, var(--color-warning, #d97706) 14%, white);
		color: var(--color-warning, #92400e);
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

		.section-header,
		.history-main {
			flex-direction: column;
		}

		.period-summary-grid,
		.period-action-grid {
			grid-template-columns: 1fr;
		}

		.status-pill {
			align-self: flex-start;
		}
	}
</style>
