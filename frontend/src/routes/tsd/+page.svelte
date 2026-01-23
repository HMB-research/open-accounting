<script lang="ts">
	import { page } from '$app/stores';
	import { api, type TSDDeclaration, type TSDStatus } from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';
	import StatusBadge, { type StatusConfig } from '$lib/components/StatusBadge.svelte';

	let declarations = $state<TSDDeclaration[]>([]);
	let isLoading = $state(true);
	let error = $state('');
	let showSubmitModal = $state(false);
	let selectedDeclaration = $state<TSDDeclaration | null>(null);
	let emtaReference = $state('');
	let filterYear = $state(new Date().getFullYear());

	function getMonthName(month: number): string {
		switch (month) {
			case 1: return m.payroll_monthJan();
			case 2: return m.payroll_monthFeb();
			case 3: return m.payroll_monthMar();
			case 4: return m.payroll_monthApr();
			case 5: return m.payroll_monthMay();
			case 6: return m.payroll_monthJun();
			case 7: return m.payroll_monthJul();
			case 8: return m.payroll_monthAug();
			case 9: return m.payroll_monthSep();
			case 10: return m.payroll_monthOct();
			case 11: return m.payroll_monthNov();
			case 12: return m.payroll_monthDec();
			default: return '';
		}
	}

	$effect(() => {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadDeclarations(tenantId);
		}
	});

	async function loadDeclarations(tenantId: string) {
		isLoading = true;
		error = '';

		try {
			declarations = await api.listTSD(tenantId, filterYear);
		} catch (err) {
			error = err instanceof Error ? err.message : m.tsd_failedToLoad();
		} finally {
			isLoading = false;
		}
	}

	async function downloadXML(declaration: TSDDeclaration) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			await api.downloadTSDXml(tenantId, declaration.period_year, declaration.period_month);
		} catch (err) {
			error = err instanceof Error ? err.message : m.tsd_failedToDownloadXml();
		}
	}

	async function downloadCSV(declaration: TSDDeclaration) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			await api.downloadTSDCsv(tenantId, declaration.period_year, declaration.period_month);
		} catch (err) {
			error = err instanceof Error ? err.message : m.tsd_failedToDownloadCsv();
		}
	}

	function openSubmitModal(declaration: TSDDeclaration) {
		selectedDeclaration = declaration;
		emtaReference = '';
		showSubmitModal = true;
	}

	async function markAsSubmitted(e: Event) {
		e.preventDefault();
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId || !selectedDeclaration) return;

		try {
			await api.markTSDSubmitted(
				tenantId,
				selectedDeclaration.period_year,
				selectedDeclaration.period_month,
				emtaReference
			);
			// Reload declarations to get updated status
			await loadDeclarations(tenantId);
			showSubmitModal = false;
		} catch (err) {
			error = err instanceof Error ? err.message : m.tsd_failedToMarkSubmitted();
		}
	}

	async function handleYearChange() {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadDeclarations(tenantId);
		}
	}

	function formatDecimal(value: Decimal | string | number): string {
		if (value instanceof Decimal) {
			return value.toFixed(2);
		}
		return new Decimal(value).toFixed(2);
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString('et-EE', {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}

	const statusConfig: Record<TSDStatus, StatusConfig> = {
		DRAFT: { class: 'badge-draft', label: m.tsd_statusDraft() },
		SUBMITTED: { class: 'badge-submitted', label: m.tsd_statusSubmitted() },
		ACCEPTED: { class: 'badge-accepted', label: m.tsd_statusAccepted() },
		REJECTED: { class: 'badge-rejected', label: m.tsd_statusRejected() }
	};

	function canSubmit(declaration: TSDDeclaration): boolean {
		return declaration.status === 'DRAFT';
	}
</script>

<svelte:head>
	<title>{m.tsd_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<div>
			<h1>{m.tsd_title()}</h1>
			<p class="subtitle">{m.tsd_subtitle()}</p>
		</div>
	</div>

	<div class="info-banner card">
		<div class="banner-icon">i</div>
		<div class="banner-content">
			<strong>{m.tsd_manualSubmissionRequired()}</strong>
			<p>
				{m.tsd_manualSubmissionInfo()} <a href="https://www.emta.ee" target="_blank" rel="noopener">{m.tsd_emtaPortal()}</a>.
			</p>
		</div>
	</div>

	<div class="filters card">
		<div class="filter-row">
			<label class="label" for="yearFilter">{m.tsd_year()}</label>
			<select class="input" id="yearFilter" bind:value={filterYear} onchange={handleYearChange}>
				{#each Array.from({ length: 5 }, (_, i) => new Date().getFullYear() - i) as year}
					<option value={year}>{year}</option>
				{/each}
			</select>
		</div>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<p>{m.tsd_loading()}</p>
	{:else if declarations.length === 0}
		<div class="empty-state card">
			<p>
				{m.tsd_emptyState({ year: filterYear.toString() })}
			</p>
			<a href="/payroll?tenant={$page.url.searchParams.get('tenant')}" class="btn btn-primary">
				{m.tsd_goToPayroll()}
			</a>
		</div>
	{:else}
		<div class="card table-container">
			<table class="table table-mobile-cards">
				<thead>
					<tr>
						<th>{m.tsd_period()}</th>
						<th>{m.tsd_status()}</th>
						<th class="text-right">{m.tsd_totalPayments()}</th>
						<th class="text-right">{m.tsd_incomeTax()}</th>
						<th class="text-right">{m.tsd_socialTax()}</th>
						<th class="text-right">{m.tsd_totalTaxes()}</th>
						<th>{m.tsd_emtaReference()}</th>
						<th>{m.tsd_actions()}</th>
					</tr>
				</thead>
				<tbody>
					{#each declarations as declaration}
						{@const totalTaxes = new Decimal(declaration.total_income_tax)
							.add(new Decimal(declaration.total_social_tax))
							.add(new Decimal(declaration.total_unemployment_employer))
							.add(new Decimal(declaration.total_unemployment_employee))
							.add(new Decimal(declaration.total_funded_pension))}
						<tr>
							<td class="period" data-label={m.tsd_period()}>
								{getMonthName(declaration.period_month)}
								{declaration.period_year}
							</td>
							<td data-label={m.tsd_status()}>
								<StatusBadge status={declaration.status} config={statusConfig} />
								{#if declaration.submitted_at}
									<div class="submitted-date">
										{formatDate(declaration.submitted_at)}
									</div>
								{/if}
							</td>
							<td class="text-right mono" data-label={m.tsd_totalPayments()}>{formatDecimal(declaration.total_payments)}</td>
							<td class="text-right mono" data-label={m.tsd_incomeTax()}>{formatDecimal(declaration.total_income_tax)}</td>
							<td class="text-right mono" data-label={m.tsd_socialTax()}>{formatDecimal(declaration.total_social_tax)}</td>
							<td class="text-right mono total-taxes" data-label={m.tsd_totalTaxes()}>{formatDecimal(totalTaxes)}</td>
							<td class="mono reference" data-label={m.tsd_emtaReference()}>{declaration.emta_reference || '-'}</td>
							<td class="actions actions-cell">
								<button class="btn btn-small" onclick={() => downloadXML(declaration)}>
									XML
								</button>
								<button class="btn btn-small" onclick={() => downloadCSV(declaration)}>
									CSV
								</button>
								{#if canSubmit(declaration)}
									<button
										class="btn btn-small btn-primary"
										onclick={() => openSubmitModal(declaration)}
									>
										{m.tsd_markSubmitted()}
									</button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}

	<div class="workflow-info card">
		<h3>{m.tsd_workflowTitle()}</h3>
		<ol class="workflow-steps">
			<li>
				<span class="step-number">1</span>
				<div class="step-content">
					<strong>{m.tsd_step1Title()}</strong>
					<p>{m.tsd_step1Desc()}</p>
				</div>
			</li>
			<li>
				<span class="step-number">2</span>
				<div class="step-content">
					<strong>{m.tsd_step2Title()}</strong>
					<p>{m.tsd_step2Desc()}</p>
				</div>
			</li>
			<li>
				<span class="step-number">3</span>
				<div class="step-content">
					<strong>{m.tsd_step3Title()}</strong>
					<p>{m.tsd_step3Desc()}</p>
				</div>
			</li>
			<li>
				<span class="step-number">4</span>
				<div class="step-content">
					<strong>{m.tsd_step4Title()}</strong>
					<p>{m.tsd_step4Desc()}</p>
				</div>
			</li>
			<li>
				<span class="step-number">5</span>
				<div class="step-content">
					<strong>{m.tsd_step5Title()}</strong>
					<p>{m.tsd_step5Desc()}</p>
				</div>
			</li>
			<li>
				<span class="step-number">6</span>
				<div class="step-content">
					<strong>{m.tsd_step6Title()}</strong>
					<p>{m.tsd_step6Desc()}</p>
				</div>
			</li>
		</ol>
	</div>
</div>

{#if showSubmitModal && selectedDeclaration}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showSubmitModal = false)} role="presentation">
		<div
			class="modal card"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
			aria-labelledby="submit-title"
			tabindex="-1"
		>
			<h2 id="submit-title">{m.tsd_markTsdSubmitted()}</h2>
			<p class="modal-subtitle">
				{getMonthName(selectedDeclaration.period_month)}
				{selectedDeclaration.period_year}
			</p>

			<form onsubmit={markAsSubmitted}>
				<div class="form-group">
					<label class="label" for="emtaRef">{m.tsd_emtaRefNumber()} *</label>
					<input
						class="input"
						type="text"
						id="emtaRef"
						bind:value={emtaReference}
						required
						placeholder={m.tsd_emtaRefPlaceholder()}
					/>
					<small class="help-text">
						{m.tsd_emtaRefHelp()}
					</small>
				</div>

				<div class="modal-actions">
					<button
						type="button"
						class="btn btn-secondary"
						onclick={() => (showSubmitModal = false)}
					>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary">{m.tsd_markSubmitted()}</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<style>
	.header {
		margin-bottom: 1.5rem;
	}

	h1 {
		font-size: 1.75rem;
		margin-bottom: 0.25rem;
	}

	.subtitle {
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}

	.info-banner {
		display: flex;
		gap: 1rem;
		padding: 1rem;
		margin-bottom: 1.5rem;
		background: #fef3c7;
		border: 1px solid #f59e0b;
	}

	.banner-icon {
		width: 24px;
		height: 24px;
		background: #f59e0b;
		color: white;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		font-weight: bold;
		flex-shrink: 0;
	}

	.banner-content strong {
		display: block;
		margin-bottom: 0.25rem;
	}

	.banner-content p {
		margin: 0;
		font-size: 0.875rem;
		color: #92400e;
	}

	.banner-content a {
		color: #92400e;
		text-decoration: underline;
	}

	.filters {
		margin-bottom: 1.5rem;
		padding: 1rem;
	}

	.filter-row {
		display: flex;
		gap: 1rem;
		align-items: center;
	}

	.filter-row .label {
		margin-bottom: 0;
	}

	.filter-row .input {
		width: auto;
	}

	.period {
		font-weight: 500;
	}

	.mono {
		font-family: var(--font-mono);
		font-size: 0.875rem;
	}

	.text-right {
		text-align: right;
	}

	.total-taxes {
		font-weight: 500;
		color: #dc2626;
	}

	.reference {
		font-size: 0.75rem;
	}

	.submitted-date {
		font-size: 0.7rem;
		color: var(--color-text-muted);
		margin-top: 0.25rem;
	}

	.actions {
		display: flex;
		gap: 0.5rem;
	}

	.btn-small {
		padding: 0.25rem 0.5rem;
		font-size: 0.75rem;
	}

	.empty-state {
		text-align: center;
		padding: 3rem;
		color: var(--color-text-muted);
	}

	.empty-state .btn {
		margin-top: 1rem;
	}

	.workflow-info {
		margin-top: 2rem;
		padding: 1.5rem;
	}

	.workflow-info h3 {
		margin-bottom: 1.5rem;
		font-size: 1rem;
	}

	.workflow-steps {
		list-style: none;
		padding: 0;
		margin: 0;
		display: grid;
		gap: 1rem;
	}

	.workflow-steps li {
		display: flex;
		gap: 1rem;
		align-items: flex-start;
	}

	.step-number {
		width: 28px;
		height: 28px;
		background: var(--color-primary);
		color: white;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 0.875rem;
		font-weight: 600;
		flex-shrink: 0;
	}

	.step-content strong {
		display: block;
		margin-bottom: 0.25rem;
	}

	.step-content p {
		margin: 0;
		font-size: 0.875rem;
		color: var(--color-text-muted);
	}

	.modal-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.5);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 100;
	}

	.modal {
		width: 100%;
		max-width: 450px;
		margin: 1rem;
	}

	.modal h2 {
		margin-bottom: 0.25rem;
	}

	.modal-subtitle {
		color: var(--color-text-muted);
		margin-bottom: 1.5rem;
	}

	.help-text {
		display: block;
		font-size: 0.75rem;
		color: var(--color-text-muted);
		margin-top: 0.25rem;
	}

	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1.5rem;
	}

	.table-container {
		overflow-x: auto;
	}

	/* Mobile styles */
	@media (max-width: 768px) {
		h1 {
			font-size: 1.5rem;
		}

		.filter-row {
			flex-direction: column;
			align-items: stretch;
		}

		.filter-row .input {
			width: 100%;
		}

		.info-banner {
			flex-direction: column;
		}

		.actions {
			flex-direction: column;
			gap: 0.5rem;
		}

		.actions .btn {
			min-height: 44px;
			width: 100%;
			justify-content: center;
		}

		.btn-small {
			padding: 0.5rem 0.75rem;
			font-size: 0.875rem;
		}

		.modal-backdrop {
			align-items: flex-end;
			padding: 0;
		}

		.modal {
			max-width: 100%;
			border-radius: 1rem 1rem 0 0;
			margin: 0;
		}

		.modal-actions {
			flex-direction: column;
		}

		.modal-actions .btn {
			width: 100%;
			min-height: 44px;
			justify-content: center;
		}

		.workflow-steps {
			gap: 1.5rem;
		}
	}
</style>
