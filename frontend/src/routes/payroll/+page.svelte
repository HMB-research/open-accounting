<script lang="ts">
	import { page } from '$app/stores';
	import { api, type PayrollRun, type Payslip, type PayrollStatus } from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';
	import StatusBadge, { type StatusConfig } from '$lib/components/StatusBadge.svelte';

	let payrollRuns = $state<PayrollRun[]>([]);
	let isLoading = $state(true);
	let error = $state('');
	let showCreateRun = $state(false);
	let showPayslips = $state(false);
	let selectedRun = $state<PayrollRun | null>(null);
	let payslips = $state<Payslip[]>([]);
	let filterYear = $state(new Date().getFullYear());

	// New payroll run form
	let newYear = $state(new Date().getFullYear());
	let newMonth = $state(new Date().getMonth() + 1);
	let newPaymentDate = $state('');
	let newNotes = $state('');

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

	const months = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12];

	$effect(() => {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadPayrollRuns(tenantId);
		}
	});

	async function loadPayrollRuns(tenantId: string) {
		isLoading = true;
		error = '';

		try {
			payrollRuns = await api.listPayrollRuns(tenantId, filterYear) ?? [];
		} catch (err) {
			error = err instanceof Error ? err.message : m.payroll_failedToLoad();
		} finally {
			isLoading = false;
		}
	}

	async function createPayrollRun(e: Event) {
		e.preventDefault();
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const run = await api.createPayrollRun(tenantId, {
				period_year: newYear,
				period_month: newMonth,
				payment_date: newPaymentDate || undefined,
				notes: newNotes || undefined
			});
			payrollRuns = [...payrollRuns, run].sort(
				(a, b) => b.period_year * 100 + b.period_month - (a.period_year * 100 + a.period_month)
			);
			showCreateRun = false;
			resetForm();
		} catch (err) {
			error = err instanceof Error ? err.message : m.payroll_failedToCreate();
		}
	}

	function resetForm() {
		newYear = new Date().getFullYear();
		newMonth = new Date().getMonth() + 1;
		newPaymentDate = '';
		newNotes = '';
	}

	async function calculatePayroll(run: PayrollRun) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const updated = await api.calculatePayroll(tenantId, run.id);
			payrollRuns = payrollRuns.map((r) => (r.id === updated.id ? updated : r));
		} catch (err) {
			error = err instanceof Error ? err.message : m.payroll_failedToCalculate();
		}
	}

	async function approvePayroll(run: PayrollRun) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const updated = await api.approvePayroll(tenantId, run.id);
			payrollRuns = payrollRuns.map((r) => (r.id === updated.id ? updated : r));
		} catch (err) {
			error = err instanceof Error ? err.message : m.payroll_failedToApprove();
		}
	}

	async function viewPayslips(run: PayrollRun) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			payslips = await api.getPayslips(tenantId, run.id);
			selectedRun = run;
			showPayslips = true;
		} catch (err) {
			error = err instanceof Error ? err.message : m.payroll_failedToLoadPayslips();
		}
	}

	async function generateTSD(run: PayrollRun) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			await api.generateTSD(tenantId, run.id);
			// Navigate to TSD page or show success
			window.location.href = `/tsd?tenant=${tenantId}`;
		} catch (err) {
			error = err instanceof Error ? err.message : m.payroll_failedToGenerateTsd();
		}
	}

	async function handleYearChange() {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadPayrollRuns(tenantId);
		}
	}

	function formatDecimal(value: Decimal | string | number): string {
		if (value instanceof Decimal) {
			return value.toFixed(2);
		}
		return new Decimal(value).toFixed(2);
	}

	const statusConfig: Record<PayrollStatus, StatusConfig> = {
		DRAFT: { class: 'badge-draft', label: m.payroll_statusDraft() },
		CALCULATED: { class: 'badge-calculated', label: m.payroll_statusCalculated() },
		APPROVED: { class: 'badge-approved', label: m.payroll_statusApproved() },
		PAID: { class: 'badge-paid', label: m.payroll_statusPaid() },
		DECLARED: { class: 'badge-declared', label: m.payroll_statusDeclared() }
	};

	function canCalculate(run: PayrollRun): boolean {
		return run.status === 'DRAFT';
	}

	function canApprove(run: PayrollRun): boolean {
		return run.status === 'CALCULATED';
	}

	function canGenerateTSD(run: PayrollRun): boolean {
		return run.status === 'APPROVED' || run.status === 'PAID';
	}
</script>

<svelte:head>
	<title>{m.payroll_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>{m.payroll_title()}</h1>
		<button class="btn btn-primary" onclick={() => (showCreateRun = true)}>
			+ {m.payroll_newRun()}
		</button>
	</div>

	<div class="filters card">
		<div class="filter-row">
			<label class="label" for="yearFilter">{m.payroll_year()}</label>
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
		<p>{m.payroll_loading()}</p>
	{:else if payrollRuns.length === 0}
		<div class="empty-state card">
			<p>
				{m.payroll_emptyState({ year: filterYear.toString() })}
			</p>
		</div>
	{:else}
		<div class="card table-container">
			<table class="table table-mobile-cards">
				<thead>
					<tr>
						<th>{m.payroll_period()}</th>
						<th>{m.payroll_status()}</th>
						<th class="text-right">{m.payroll_grossTotal()}</th>
						<th class="text-right">{m.payroll_netTotal()}</th>
						<th class="text-right">{m.payroll_employerCost()}</th>
						<th>{m.payroll_actions()}</th>
					</tr>
				</thead>
				<tbody>
					{#each payrollRuns as run}
						<tr>
							<td class="period" data-label={m.payroll_period()}>{getMonthName(run.period_month)} {run.period_year}</td>
							<td data-label={m.payroll_status()}>
								<StatusBadge status={run.status} config={statusConfig} />
							</td>
							<td class="text-right mono" data-label={m.payroll_grossTotal()}>{formatDecimal(run.total_gross)}</td>
							<td class="text-right mono" data-label={m.payroll_netTotal()}>{formatDecimal(run.total_net)}</td>
							<td class="text-right mono" data-label={m.payroll_employerCost()}>{formatDecimal(run.total_employer_cost)}</td>
							<td class="actions actions-cell">
								<button class="btn btn-small" onclick={() => viewPayslips(run)}>{m.payroll_payslips()}</button>
								{#if canCalculate(run)}
									<button class="btn btn-small btn-primary" onclick={() => calculatePayroll(run)}>
										{m.payroll_calculate()}
									</button>
								{/if}
								{#if canApprove(run)}
									<button class="btn btn-small btn-success" onclick={() => approvePayroll(run)}>
										{m.payroll_approve()}
									</button>
								{/if}
								{#if canGenerateTSD(run)}
									<button class="btn btn-small btn-secondary" onclick={() => generateTSD(run)}>
										{m.payroll_generateTsd()}
									</button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}

	<div class="info-box card">
		<h3>{m.payroll_estonianTaxRates()}</h3>
		<div class="tax-rates">
			<div class="tax-rate">
				<span class="rate-label">{m.payroll_incomeTaxRate()}</span>
				<span class="rate-value">22%</span>
			</div>
			<div class="tax-rate">
				<span class="rate-label">{m.payroll_socialTaxRate()}</span>
				<span class="rate-value">33%</span>
			</div>
			<div class="tax-rate">
				<span class="rate-label">{m.payroll_unemploymentEmployeeRate()}</span>
				<span class="rate-value">1.6%</span>
			</div>
			<div class="tax-rate">
				<span class="rate-label">{m.payroll_unemploymentEmployerRate()}</span>
				<span class="rate-value">0.8%</span>
			</div>
			<div class="tax-rate">
				<span class="rate-label">{m.payroll_basicExemptionMax()}</span>
				<span class="rate-value">700 EUR</span>
			</div>
		</div>
	</div>
</div>

{#if showCreateRun}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showCreateRun = false)} role="presentation">
		<div
			class="modal card"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
			aria-labelledby="create-run-title"
			tabindex="-1"
		>
			<h2 id="create-run-title">{m.payroll_createRun()}</h2>
			<form onsubmit={createPayrollRun}>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="year">{m.payroll_year()} *</label>
						<select class="input" id="year" bind:value={newYear} required>
							{#each Array.from({ length: 5 }, (_, i) => new Date().getFullYear() - i + 1) as year}
								<option value={year}>{year}</option>
							{/each}
						</select>
					</div>
					<div class="form-group">
						<label class="label" for="month">{m.payroll_month()} *</label>
						<select class="input" id="month" bind:value={newMonth} required>
							{#each months as month}
								<option value={month}>{getMonthName(month)}</option>
							{/each}
						</select>
					</div>
				</div>

				<div class="form-group">
					<label class="label" for="paymentDate">{m.payroll_paymentDate()}</label>
					<input class="input" type="date" id="paymentDate" bind:value={newPaymentDate} />
					<small class="help-text">{m.payroll_paymentDateHelp()}</small>
				</div>

				<div class="form-group">
					<label class="label" for="notes">{m.payroll_notes()}</label>
					<textarea
						class="input"
						id="notes"
						bind:value={newNotes}
						rows="3"
						placeholder={m.payroll_notesPlaceholder()}
					></textarea>
				</div>

				<div class="modal-actions">
					<button type="button" class="btn btn-secondary" onclick={() => (showCreateRun = false)}>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary">{m.payroll_create()}</button>
				</div>
			</form>
		</div>
	</div>
{/if}

{#if showPayslips && selectedRun}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showPayslips = false)} role="presentation">
		<div
			class="modal modal-wide card"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
			aria-labelledby="payslips-title"
			tabindex="-1"
		>
			<h2 id="payslips-title">
				{m.payroll_payslips()} - {getMonthName(selectedRun.period_month)}
				{selectedRun.period_year}
			</h2>

			{#if payslips.length === 0}
				<p class="text-muted">{m.payroll_noPayslips()}</p>
			{:else}
				<div class="payslips-table-container">
					<table class="table payslips-table">
						<thead>
							<tr>
								<th>{m.payroll_employee()}</th>
								<th class="text-right">{m.payroll_gross()}</th>
								<th class="text-right">{m.payroll_incomeTax()}</th>
								<th class="text-right">{m.payroll_unempEe()}</th>
								<th class="text-right">{m.payroll_pension()}</th>
								<th class="text-right">{m.payroll_net()}</th>
								<th class="text-right">{m.payroll_socialTax()}</th>
								<th class="text-right">{m.payroll_unempEr()}</th>
								<th class="text-right">{m.payroll_totalCost()}</th>
							</tr>
						</thead>
						<tbody>
							{#each payslips as payslip}
								<tr>
									<td class="employee-name">
										{#if payslip.employee}
											{payslip.employee.last_name}, {payslip.employee.first_name}
										{:else}
											{m.payroll_employeeId()} {payslip.employee_id}
										{/if}
									</td>
									<td class="text-right mono">{formatDecimal(payslip.gross_salary)}</td>
									<td class="text-right mono">{formatDecimal(payslip.income_tax)}</td>
									<td class="text-right mono">
										{formatDecimal(payslip.unemployment_insurance_employee)}
									</td>
									<td class="text-right mono">{formatDecimal(payslip.funded_pension)}</td>
									<td class="text-right mono net-salary">{formatDecimal(payslip.net_salary)}</td>
									<td class="text-right mono employer-cost">{formatDecimal(payslip.social_tax)}</td>
									<td class="text-right mono employer-cost">
										{formatDecimal(payslip.unemployment_insurance_employer)}
									</td>
									<td class="text-right mono total-cost">
										{formatDecimal(payslip.total_employer_cost)}
									</td>
								</tr>
							{/each}
						</tbody>
						<tfoot>
							<tr class="totals-row">
								<td><strong>{m.payroll_totals()}</strong></td>
								<td class="text-right mono">
									<strong>{formatDecimal(selectedRun.total_gross)}</strong>
								</td>
								<td colspan="4"></td>
								<td class="text-right mono">
									<strong>{formatDecimal(selectedRun.total_net)}</strong>
								</td>
								<td colspan="2"></td>
								<td class="text-right mono">
									<strong>{formatDecimal(selectedRun.total_employer_cost)}</strong>
								</td>
							</tr>
						</tfoot>
					</table>
				</div>
			{/if}

			<div class="modal-actions">
				<button type="button" class="btn btn-secondary" onclick={() => (showPayslips = false)}>
					{m.payroll_close()}
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1.5rem;
	}

	h1 {
		font-size: 1.75rem;
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

	.text-muted {
		color: var(--color-text-muted);
	}

	.actions {
		display: flex;
		gap: 0.5rem;
	}

	.btn-small {
		padding: 0.25rem 0.5rem;
		font-size: 0.75rem;
	}

	.btn-success {
		background: #16a34a;
		color: white;
	}

	.btn-success:hover {
		background: #15803d;
	}

	.empty-state {
		text-align: center;
		padding: 3rem;
		color: var(--color-text-muted);
	}

	.info-box {
		margin-top: 2rem;
		padding: 1.5rem;
	}

	.info-box h3 {
		margin-bottom: 1rem;
		font-size: 1rem;
	}

	.tax-rates {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
		gap: 1rem;
	}

	.tax-rate {
		display: flex;
		justify-content: space-between;
		padding: 0.5rem;
		background: var(--color-bg);
		border-radius: 4px;
	}

	.rate-label {
		color: var(--color-text-muted);
	}

	.rate-value {
		font-weight: 600;
		font-family: var(--font-mono);
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
		max-width: 500px;
		margin: 1rem;
		max-height: 90vh;
		overflow-y: auto;
	}

	.modal-wide {
		max-width: 1000px;
	}

	.modal h2 {
		margin-bottom: 1.5rem;
	}

	.form-row {
		display: flex;
		gap: 1rem;
	}

	.form-row .form-group {
		flex: 1;
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

	.payslips-table-container {
		overflow-x: auto;
		margin: 1rem 0;
	}

	.payslips-table th,
	.payslips-table td {
		padding: 0.5rem;
		font-size: 0.875rem;
	}

	.employee-name {
		font-weight: 500;
		white-space: nowrap;
	}

	.net-salary {
		color: #166534;
		font-weight: 500;
	}

	.employer-cost {
		color: var(--color-text-muted);
	}

	.total-cost {
		font-weight: 500;
	}

	.totals-row {
		border-top: 2px solid var(--color-border);
		background: var(--color-bg);
	}

	textarea.input {
		resize: vertical;
		min-height: 80px;
	}

	/* Mobile responsive */
	@media (max-width: 768px) {
		h1 {
			font-size: 1.25rem;
		}

		.header {
			flex-direction: column;
			align-items: flex-start;
			gap: 1rem;
		}

		.header .btn {
			width: 100%;
			min-height: 44px;
			justify-content: center;
		}

		.filter-row {
			flex-direction: column;
			gap: 0.5rem;
			align-items: flex-start;
		}

		.filter-row .input {
			width: 100%;
			min-height: 44px;
		}

		.table {
			font-size: 0.875rem;
		}

		.table th,
		.table td {
			padding: 0.5rem;
		}

		.actions {
			flex-direction: column;
			gap: 0.5rem;
		}

		.actions .btn {
			width: 100%;
			min-height: 44px;
		}

		.btn-small {
			padding: 0.5rem 0.75rem;
			font-size: 0.875rem;
		}

		.tax-rates {
			grid-template-columns: 1fr;
		}

		.empty-state {
			padding: 2rem 1rem;
		}

		.modal-backdrop {
			padding: 0;
			align-items: flex-end;
		}

		.modal {
			max-width: 100%;
			max-height: 95vh;
			border-radius: 1rem 1rem 0 0;
			margin: 0;
		}

		.modal h2 {
			font-size: 1.25rem;
		}

		.form-row {
			flex-direction: column;
			gap: 0;
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
