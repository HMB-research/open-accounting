<script lang="ts">
	import { page } from '$app/stores';
	import { api, type TSDDeclaration, type TSDStatus } from '$lib/api';
	import Decimal from 'decimal.js';

	let declarations = $state<TSDDeclaration[]>([]);
	let isLoading = $state(true);
	let error = $state('');
	let showSubmitModal = $state(false);
	let selectedDeclaration = $state<TSDDeclaration | null>(null);
	let emtaReference = $state('');
	let filterYear = $state(new Date().getFullYear());

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
			error = err instanceof Error ? err.message : 'Failed to load TSD declarations';
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
			error = err instanceof Error ? err.message : 'Failed to download XML';
		}
	}

	async function downloadCSV(declaration: TSDDeclaration) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			await api.downloadTSDCsv(tenantId, declaration.period_year, declaration.period_month);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to download CSV';
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
			error = err instanceof Error ? err.message : 'Failed to mark as submitted';
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

	const statusLabels: Record<TSDStatus, string> = {
		DRAFT: 'Draft',
		SUBMITTED: 'Submitted',
		ACCEPTED: 'Accepted',
		REJECTED: 'Rejected'
	};

	const statusBadgeClass: Record<TSDStatus, string> = {
		DRAFT: 'badge-draft',
		SUBMITTED: 'badge-submitted',
		ACCEPTED: 'badge-accepted',
		REJECTED: 'badge-rejected'
	};

	function canSubmit(declaration: TSDDeclaration): boolean {
		return declaration.status === 'DRAFT';
	}
</script>

<svelte:head>
	<title>TSD Declarations - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<div>
			<h1>TSD Declarations</h1>
			<p class="subtitle">Tulu- ja sotsiaalmaksu deklaratsioon</p>
		</div>
	</div>

	<div class="info-banner card">
		<div class="banner-icon">i</div>
		<div class="banner-content">
			<strong>Manual Submission Required</strong>
			<p>
				Automatic e-MTA submission is not yet available. Export your TSD as XML and upload it
				manually to the <a href="https://www.emta.ee" target="_blank" rel="noopener">e-MTA portal</a
				>.
			</p>
		</div>
	</div>

	<div class="filters card">
		<div class="filter-row">
			<label class="label" for="yearFilter">Year</label>
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
		<p>Loading TSD declarations...</p>
	{:else if declarations.length === 0}
		<div class="empty-state card">
			<p>
				No TSD declarations found for {filterYear}. Generate TSD from an approved payroll run first.
			</p>
			<a href="/payroll?tenant={$page.url.searchParams.get('tenant')}" class="btn btn-primary">
				Go to Payroll
			</a>
		</div>
	{:else}
		<div class="card">
			<table class="table">
				<thead>
					<tr>
						<th>Period</th>
						<th>Status</th>
						<th class="text-right">Total Payments</th>
						<th class="text-right">Income Tax</th>
						<th class="text-right">Social Tax</th>
						<th class="text-right">Total Taxes</th>
						<th>e-MTA Reference</th>
						<th>Actions</th>
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
							<td class="period">
								{months[declaration.period_month - 1]}
								{declaration.period_year}
							</td>
							<td>
								<span class="badge {statusBadgeClass[declaration.status]}">
									{statusLabels[declaration.status]}
								</span>
								{#if declaration.submitted_at}
									<div class="submitted-date">
										{formatDate(declaration.submitted_at)}
									</div>
								{/if}
							</td>
							<td class="text-right mono">{formatDecimal(declaration.total_payments)}</td>
							<td class="text-right mono">{formatDecimal(declaration.total_income_tax)}</td>
							<td class="text-right mono">{formatDecimal(declaration.total_social_tax)}</td>
							<td class="text-right mono total-taxes">{formatDecimal(totalTaxes)}</td>
							<td class="mono reference">{declaration.emta_reference || '-'}</td>
							<td class="actions">
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
										Mark Submitted
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
		<h3>TSD Submission Workflow</h3>
		<ol class="workflow-steps">
			<li>
				<span class="step-number">1</span>
				<div class="step-content">
					<strong>Generate Payroll</strong>
					<p>Create and calculate payroll for the period</p>
				</div>
			</li>
			<li>
				<span class="step-number">2</span>
				<div class="step-content">
					<strong>Approve Payroll</strong>
					<p>Review and approve the calculated payroll</p>
				</div>
			</li>
			<li>
				<span class="step-number">3</span>
				<div class="step-content">
					<strong>Generate TSD</strong>
					<p>Generate TSD declaration from approved payroll</p>
				</div>
			</li>
			<li>
				<span class="step-number">4</span>
				<div class="step-content">
					<strong>Export XML</strong>
					<p>Download the TSD in XML format</p>
				</div>
			</li>
			<li>
				<span class="step-number">5</span>
				<div class="step-content">
					<strong>Upload to e-MTA</strong>
					<p>Log into e-MTA portal and upload the XML file</p>
				</div>
			</li>
			<li>
				<span class="step-number">6</span>
				<div class="step-content">
					<strong>Record Reference</strong>
					<p>Mark as submitted with the e-MTA reference number</p>
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
			<h2 id="submit-title">Mark TSD as Submitted</h2>
			<p class="modal-subtitle">
				{months[selectedDeclaration.period_month - 1]}
				{selectedDeclaration.period_year}
			</p>

			<form onsubmit={markAsSubmitted}>
				<div class="form-group">
					<label class="label" for="emtaRef">e-MTA Reference Number *</label>
					<input
						class="input"
						type="text"
						id="emtaRef"
						bind:value={emtaReference}
						required
						placeholder="e.g., TSD-2025-12345"
					/>
					<small class="help-text">
						Enter the reference number you received from e-MTA after submitting
					</small>
				</div>

				<div class="modal-actions">
					<button
						type="button"
						class="btn btn-secondary"
						onclick={() => (showSubmitModal = false)}
					>
						Cancel
					</button>
					<button type="submit" class="btn btn-primary">Mark as Submitted</button>
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

	.badge-draft {
		background: #f3f4f6;
		color: #374151;
	}

	.badge-submitted {
		background: #fef3c7;
		color: #92400e;
	}

	.badge-accepted {
		background: #dcfce7;
		color: #166534;
	}

	.badge-rejected {
		background: #fef2f2;
		color: #991b1b;
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
</style>
