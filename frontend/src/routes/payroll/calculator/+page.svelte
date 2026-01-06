<script lang="ts">
	import { page } from '$app/stores';
	import { api, type TaxCalculation } from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';

	let grossSalary = $state('2000');
	let applyBasicExemption = $state(true);
	let basicExemptionAmount = $state('700');
	let fundedPensionRate = $state('0.02');
	let isCalculating = $state(false);
	let error = $state('');
	let result = $state<TaxCalculation | null>(null);

	// Tax rate constants for display
	const TAX_RATES = {
		incomeTax: 22,
		socialTax: 33,
		unemploymentEmployee: 1.6,
		unemploymentEmployer: 0.8,
		maxBasicExemption: 700,
		fundedPensionOptions: [
			{ value: '0', label: '0%' },
			{ value: '0.02', label: '2%' },
			{ value: '0.04', label: '4%' }
		]
	};

	async function calculate() {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) {
			error = 'No tenant selected';
			return;
		}

		if (!grossSalary || parseFloat(grossSalary) <= 0) {
			error = m.calc_error_positive_salary();
			return;
		}

		isCalculating = true;
		error = '';

		try {
			result = await api.calculateTaxPreview(
				tenantId,
				grossSalary,
				applyBasicExemption ? basicExemptionAmount : '0',
				fundedPensionRate
			);
		} catch (err) {
			error = err instanceof Error ? err.message : m.calc_error_calculation();
		} finally {
			isCalculating = false;
		}
	}

	function formatDecimal(value: Decimal | string | number): string {
		if (value instanceof Decimal) {
			return value.toFixed(2);
		}
		return new Decimal(value).toFixed(2);
	}

	function formatPercent(rate: number): string {
		return `${rate}%`;
	}

	// Auto-calculate when inputs change
	$effect(() => {
		// Trigger on any input change
		void grossSalary;
		void applyBasicExemption;
		void basicExemptionAmount;
		void fundedPensionRate;

		// Debounce calculation
		const timeout = setTimeout(() => {
			if (grossSalary && parseFloat(grossSalary) > 0) {
				calculate();
			} else {
				// Clear results for zero or invalid salary
				result = null;
				error = '';
			}
		}, 300);

		return () => clearTimeout(timeout);
	});
</script>

<svelte:head>
	<title>{m.calc_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>{m.calc_title()}</h1>
		<p class="subtitle">{m.calc_subtitle()}</p>
	</div>

	<div class="calculator-layout">
		<div class="input-section card">
			<h2>{m.calc_input_section()}</h2>

			<div class="form-group">
				<label class="label" for="grossSalary">{m.calc_gross_salary()}</label>
				<div class="input-with-suffix">
					<input
						class="input"
						type="number"
						id="grossSalary"
						bind:value={grossSalary}
						min="0"
						step="100"
						placeholder="2000"
					/>
					<span class="suffix">EUR</span>
				</div>
			</div>

			<div class="form-group">
				<label class="checkbox-label">
					<input type="checkbox" bind:checked={applyBasicExemption} />
					{m.calc_apply_basic_exemption()}
				</label>
			</div>

			{#if applyBasicExemption}
				<div class="form-group">
					<label class="label" for="basicExemption">{m.calc_basic_exemption_amount()}</label>
					<div class="input-with-suffix">
						<input
							class="input"
							type="number"
							id="basicExemption"
							bind:value={basicExemptionAmount}
							min="0"
							max="700"
							step="50"
						/>
						<span class="suffix">EUR</span>
					</div>
					<small class="help-text">{m.calc_basic_exemption_help()}</small>
				</div>
			{/if}

			<div class="form-group">
				<label class="label" for="pensionRate">{m.calc_funded_pension_rate()}</label>
				<select class="input" id="pensionRate" bind:value={fundedPensionRate}>
					{#each TAX_RATES.fundedPensionOptions as opt}
						<option value={opt.value}>{opt.label}</option>
					{/each}
				</select>
				<small class="help-text">{m.calc_pension_help()}</small>
			</div>

			{#if error}
				<div class="alert alert-error">{error}</div>
			{/if}
		</div>

		<div class="results-section card">
			<h2>{m.calc_results_section()}</h2>

			{#if isCalculating}
				<p class="calculating">{m.calc_calculating()}</p>
			{:else if result}
				<div class="result-breakdown">
					<div class="result-group">
						<h3>{m.calc_employee_section()}</h3>
						<div class="result-row highlight">
							<span class="result-label">{m.calc_gross_salary()}</span>
							<span class="result-value">{formatDecimal(result.gross_salary)} EUR</span>
						</div>
						<div class="result-row deduction">
							<span class="result-label">- {m.calc_income_tax()} ({formatPercent(TAX_RATES.incomeTax)})</span>
							<span class="result-value">{formatDecimal(result.income_tax)} EUR</span>
						</div>
						<div class="result-row deduction">
							<span class="result-label">- {m.calc_unemployment_ee()} ({formatPercent(TAX_RATES.unemploymentEmployee)})</span>
							<span class="result-value">{formatDecimal(result.unemployment_employee)} EUR</span>
						</div>
						<div class="result-row deduction">
							<span class="result-label">- {m.calc_funded_pension()}</span>
							<span class="result-value">{formatDecimal(result.funded_pension)} EUR</span>
						</div>
						<div class="result-row total">
							<span class="result-label">{m.calc_total_deductions()}</span>
							<span class="result-value">{formatDecimal(result.total_deductions)} EUR</span>
						</div>
						<div class="result-row net-salary">
							<span class="result-label">{m.calc_net_salary()}</span>
							<span class="result-value">{formatDecimal(result.net_salary)} EUR</span>
						</div>
					</div>

					<div class="result-group">
						<h3>{m.calc_employer_section()}</h3>
						<div class="result-row">
							<span class="result-label">{m.calc_gross_salary()}</span>
							<span class="result-value">{formatDecimal(result.gross_salary)} EUR</span>
						</div>
						<div class="result-row employer-cost">
							<span class="result-label">+ {m.calc_social_tax()} ({formatPercent(TAX_RATES.socialTax)})</span>
							<span class="result-value">{formatDecimal(result.social_tax)} EUR</span>
						</div>
						<div class="result-row employer-cost">
							<span class="result-label">+ {m.calc_unemployment_er()} ({formatPercent(TAX_RATES.unemploymentEmployer)})</span>
							<span class="result-value">{formatDecimal(result.unemployment_employer)} EUR</span>
						</div>
						<div class="result-row total-cost">
							<span class="result-label">{m.calc_total_employer_cost()}</span>
							<span class="result-value">{formatDecimal(result.total_employer_cost)} EUR</span>
						</div>
					</div>

					<div class="result-group info-box">
						<h3>{m.calc_tax_details()}</h3>
						<div class="result-row">
							<span class="result-label">{m.calc_basic_exemption_applied()}</span>
							<span class="result-value">{formatDecimal(result.basic_exemption)} EUR</span>
						</div>
						<div class="result-row">
							<span class="result-label">{m.calc_taxable_income()}</span>
							<span class="result-value">{formatDecimal(result.taxable_income)} EUR</span>
						</div>
					</div>
				</div>
			{:else}
				<p class="no-result">{m.calc_enter_salary()}</p>
			{/if}
		</div>
	</div>

	<div class="info-box card">
		<h3>{m.calc_tax_rates_title()}</h3>
		<div class="tax-rates">
			<div class="tax-rate">
				<span class="rate-label">{m.payroll_incomeTaxRate()}</span>
				<span class="rate-value">{formatPercent(TAX_RATES.incomeTax)}</span>
			</div>
			<div class="tax-rate">
				<span class="rate-label">{m.payroll_socialTaxRate()}</span>
				<span class="rate-value">{formatPercent(TAX_RATES.socialTax)}</span>
			</div>
			<div class="tax-rate">
				<span class="rate-label">{m.payroll_unemploymentEmployeeRate()}</span>
				<span class="rate-value">{formatPercent(TAX_RATES.unemploymentEmployee)}</span>
			</div>
			<div class="tax-rate">
				<span class="rate-label">{m.payroll_unemploymentEmployerRate()}</span>
				<span class="rate-value">{formatPercent(TAX_RATES.unemploymentEmployer)}</span>
			</div>
			<div class="tax-rate">
				<span class="rate-label">{m.payroll_basicExemptionMax()}</span>
				<span class="rate-value">{TAX_RATES.maxBasicExemption} EUR</span>
			</div>
		</div>
	</div>
</div>

<style>
	.header {
		margin-bottom: 1.5rem;
	}

	h1 {
		font-size: 1.75rem;
		margin-bottom: 0.5rem;
	}

	.subtitle {
		color: var(--color-text-muted);
		margin: 0;
	}

	.calculator-layout {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1.5rem;
		margin-bottom: 1.5rem;
	}

	.input-section,
	.results-section {
		padding: 1.5rem;
	}

	.input-section h2,
	.results-section h2 {
		font-size: 1.125rem;
		margin-bottom: 1rem;
		padding-bottom: 0.5rem;
		border-bottom: 1px solid var(--color-border);
	}

	.input-with-suffix {
		display: flex;
		gap: 0;
	}

	.input-with-suffix .input {
		border-radius: 4px 0 0 4px;
		flex: 1;
	}

	.suffix {
		background: var(--color-bg);
		border: 1px solid var(--color-border);
		border-left: none;
		border-radius: 0 4px 4px 0;
		padding: 0.5rem 0.75rem;
		font-size: 0.875rem;
		color: var(--color-text-muted);
	}

	.checkbox-label {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		cursor: pointer;
	}

	.help-text {
		display: block;
		font-size: 0.75rem;
		color: var(--color-text-muted);
		margin-top: 0.25rem;
	}

	.calculating {
		text-align: center;
		color: var(--color-text-muted);
		padding: 2rem;
	}

	.no-result {
		text-align: center;
		color: var(--color-text-muted);
		padding: 2rem;
	}

	.result-breakdown {
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
	}

	.result-group {
		padding: 1rem;
		background: var(--color-bg);
		border-radius: 8px;
	}

	.result-group h3 {
		font-size: 0.875rem;
		font-weight: 600;
		margin-bottom: 0.75rem;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.result-row {
		display: flex;
		justify-content: space-between;
		padding: 0.5rem 0;
		border-bottom: 1px solid var(--color-border);
	}

	.result-row:last-child {
		border-bottom: none;
	}

	.result-label {
		color: var(--color-text);
	}

	.result-value {
		font-family: var(--font-mono);
		font-weight: 500;
	}

	.result-row.highlight {
		font-weight: 600;
	}

	.result-row.deduction .result-label {
		color: var(--color-text-muted);
	}

	.result-row.deduction .result-value {
		color: #dc2626;
	}

	.result-row.total {
		font-weight: 500;
		border-top: 1px solid var(--color-border);
		margin-top: 0.5rem;
		padding-top: 0.75rem;
	}

	.result-row.net-salary {
		font-size: 1.125rem;
		font-weight: 600;
		background: #dcfce7;
		margin: 0.5rem -1rem -1rem;
		padding: 1rem;
		border-radius: 0 0 8px 8px;
	}

	.result-row.net-salary .result-value {
		color: #166534;
	}

	.result-row.employer-cost .result-value {
		color: #92400e;
	}

	.result-row.total-cost {
		font-size: 1.125rem;
		font-weight: 600;
		background: #fef3c7;
		margin: 0.5rem -1rem -1rem;
		padding: 1rem;
		border-radius: 0 0 8px 8px;
	}

	.result-group.info-box {
		background: #f0f9ff;
	}

	.info-box {
		margin-top: 1.5rem;
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

	/* Mobile responsive */
	@media (max-width: 768px) {
		h1 {
			font-size: 1.25rem;
		}

		.calculator-layout {
			grid-template-columns: 1fr;
		}

		.input-section,
		.results-section {
			padding: 1rem;
		}

		.result-row.net-salary,
		.result-row.total-cost {
			margin: 0.5rem -1rem -1rem;
		}

		.tax-rates {
			grid-template-columns: 1fr;
		}
	}
</style>
