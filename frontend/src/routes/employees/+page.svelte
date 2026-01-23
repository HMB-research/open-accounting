<script lang="ts">
	import { page } from '$app/stores';
	import { api, type Employee, type EmploymentType } from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';
	import StatusBadge, { type StatusConfig } from '$lib/components/StatusBadge.svelte';

	let employees = $state<Employee[]>([]);
	let isLoading = $state(true);
	let error = $state('');
	let showCreateEmployee = $state(false);
	let showActiveOnly = $state(true);
	let searchQuery = $state('');

	// New employee form
	let newFirstName = $state('');
	let newLastName = $state('');
	let newPersonalCode = $state('');
	let newEmail = $state('');
	let newPhone = $state('');
	let newAddress = $state('');
	let newBankAccount = $state('');
	let newStartDate = $state(new Date().toISOString().split('T')[0]);
	let newPosition = $state('');
	let newDepartment = $state('');
	let newEmploymentType = $state<EmploymentType>('FULL_TIME');
	let newApplyBasicExemption = $state(true);
	let newBasicExemptionAmount = $state('700.00');
	let newFundedPensionRate = $state('0.02');

	$effect(() => {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadEmployees(tenantId);
		}
	});

	async function loadEmployees(tenantId: string) {
		isLoading = true;
		error = '';

		try {
			employees = await api.listEmployees(tenantId, showActiveOnly);
		} catch (err) {
			error = err instanceof Error ? err.message : m.employees_failedToLoad();
		} finally {
			isLoading = false;
		}
	}

	async function createEmployee(e: Event) {
		e.preventDefault();
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const employee = await api.createEmployee(tenantId, {
				first_name: newFirstName,
				last_name: newLastName,
				personal_code: newPersonalCode || undefined,
				email: newEmail || undefined,
				phone: newPhone || undefined,
				address: newAddress || undefined,
				bank_account: newBankAccount || undefined,
				start_date: newStartDate,
				position: newPosition || undefined,
				department: newDepartment || undefined,
				employment_type: newEmploymentType,
				apply_basic_exemption: newApplyBasicExemption,
				basic_exemption_amount: newBasicExemptionAmount || undefined,
				funded_pension_rate: newFundedPensionRate || undefined
			});
			employees = [...employees, employee].sort((a, b) =>
				`${a.last_name} ${a.first_name}`.localeCompare(`${b.last_name} ${b.first_name}`)
			);
			showCreateEmployee = false;
			resetForm();
		} catch (err) {
			error = err instanceof Error ? err.message : m.employees_failedToCreate();
		}
	}

	function resetForm() {
		newFirstName = '';
		newLastName = '';
		newPersonalCode = '';
		newEmail = '';
		newPhone = '';
		newAddress = '';
		newBankAccount = '';
		newStartDate = new Date().toISOString().split('T')[0];
		newPosition = '';
		newDepartment = '';
		newEmploymentType = 'FULL_TIME';
		newApplyBasicExemption = true;
		newBasicExemptionAmount = '700.00';
		newFundedPensionRate = '0.02';
	}

	async function handleFilter() {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadEmployees(tenantId);
		}
	}

	function formatDecimal(value: Decimal | string | number): string {
		if (value instanceof Decimal) {
			return value.toFixed(2);
		}
		return new Decimal(value).toFixed(2);
	}

	function formatPercent(value: Decimal | string | number): string {
		if (value instanceof Decimal) {
			return `${value.mul(100).toFixed(0)}%`;
		}
		return `${new Decimal(value).mul(100).toFixed(0)}%`;
	}

	const typeConfig: Record<EmploymentType, StatusConfig> = {
		FULL_TIME: { class: 'badge-fulltime', label: m.employees_fullTime() },
		PART_TIME: { class: 'badge-parttime', label: m.employees_partTime() },
		CONTRACT: { class: 'badge-contract', label: m.employees_contract() }
	};

	$effect(() => {
		// Filter employees by search query
		if (searchQuery) {
			const query = searchQuery.toLowerCase();
			employees = employees.filter(
				(e) =>
					e.first_name.toLowerCase().includes(query) ||
					e.last_name.toLowerCase().includes(query) ||
					(e.personal_code && e.personal_code.includes(query)) ||
					(e.position && e.position.toLowerCase().includes(query))
			);
		}
	});
</script>

<svelte:head>
	<title>{m.employees_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>{m.employees_title()}</h1>
		<button class="btn btn-primary" onclick={() => (showCreateEmployee = true)}>
			+ {m.employees_newEmployee()}
		</button>
	</div>

	<div class="filters card">
		<div class="filter-row">
			<label class="checkbox-label">
				<input type="checkbox" bind:checked={showActiveOnly} onchange={handleFilter} />
				{m.employees_activeOnly()}
			</label>
			<input
				class="input search-input"
				type="text"
				placeholder={m.employees_searchPlaceholder()}
				bind:value={searchQuery}
			/>
		</div>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<p>{m.employees_loading()}</p>
	{:else if employees.length === 0}
		<div class="empty-state card">
			<p>{m.employees_emptyState()}</p>
		</div>
	{:else}
		<div class="card table-container">
			<table class="table table-mobile-cards">
				<thead>
					<tr>
						<th>{m.employees_name()}</th>
						<th>{m.employees_personalCode()}</th>
						<th>{m.employees_position()}</th>
						<th>{m.employees_type()}</th>
						<th>{m.employees_basicExemption()}</th>
						<th>{m.employees_pensionRate()}</th>
						<th>{m.employees_status()}</th>
					</tr>
				</thead>
				<tbody>
					{#each employees as employee}
						<tr class:inactive={!employee.is_active}>
							<td class="name" data-label={m.employees_name()}>
								{employee.last_name}, {employee.first_name}
								{#if employee.email}
									<div class="email-sub">{employee.email}</div>
								{/if}
							</td>
							<td class="mono" data-label={m.employees_personalCode()}>{employee.personal_code || '-'}</td>
							<td data-label={m.employees_position()}>
								{employee.position || '-'}
								{#if employee.department}
									<div class="dept-sub">{employee.department}</div>
								{/if}
							</td>
							<td data-label={m.employees_type()}>
								<StatusBadge status={employee.employment_type} config={typeConfig} />
							</td>
							<td data-label={m.employees_basicExemption()}>
								{#if employee.apply_basic_exemption}
									{formatDecimal(employee.basic_exemption_amount)}
								{:else}
									<span class="text-muted">{m.employees_notApplied()}</span>
								{/if}
							</td>
							<td data-label={m.employees_pensionRate()}>{formatPercent(employee.funded_pension_rate)}</td>
							<td data-label={m.employees_status()}>
								<span class="badge {employee.is_active ? 'badge-active' : 'badge-inactive'}">
									{employee.is_active ? m.employees_active() : m.employees_inactive()}
								</span>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

{#if showCreateEmployee}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showCreateEmployee = false)} role="presentation">
		<div
			class="modal card"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
			aria-labelledby="create-employee-title"
			tabindex="-1"
		>
			<h2 id="create-employee-title">{m.employees_addNewEmployee()}</h2>
			<form onsubmit={createEmployee}>
				<h3 class="section-title">{m.employees_personalInfo()}</h3>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="firstName">{m.employees_firstName()} *</label>
						<input
							class="input"
							type="text"
							id="firstName"
							bind:value={newFirstName}
							required
							placeholder="Mari"
						/>
					</div>
					<div class="form-group">
						<label class="label" for="lastName">{m.employees_lastName()} *</label>
						<input
							class="input"
							type="text"
							id="lastName"
							bind:value={newLastName}
							required
							placeholder="Maasikas"
						/>
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="personalCode">{m.employees_personalCodeIsikukood()}</label>
						<input
							class="input"
							type="text"
							id="personalCode"
							bind:value={newPersonalCode}
							placeholder="38001010001"
							maxlength="11"
						/>
					</div>
					<div class="form-group">
						<label class="label" for="email">{m.common_email()}</label>
						<input
							class="input"
							type="email"
							id="email"
							bind:value={newEmail}
							placeholder="mari@example.com"
						/>
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="phone">{m.employees_phone()}</label>
						<input
							class="input"
							type="tel"
							id="phone"
							bind:value={newPhone}
							placeholder="+372 5551234"
						/>
					</div>
					<div class="form-group">
						<label class="label" for="bankAccount">{m.employees_bankAccount()}</label>
						<input
							class="input"
							type="text"
							id="bankAccount"
							bind:value={newBankAccount}
							placeholder="EE123456789012345678"
						/>
					</div>
				</div>

				<div class="form-group">
					<label class="label" for="address">{m.employees_address()}</label>
					<input
						class="input"
						type="text"
						id="address"
						bind:value={newAddress}
						placeholder="Pikk tn 1, 10111 Tallinn"
					/>
				</div>

				<h3 class="section-title">{m.employees_employmentDetails()}</h3>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="startDate">{m.employees_startDate()} *</label>
						<input class="input" type="date" id="startDate" bind:value={newStartDate} required />
					</div>
					<div class="form-group">
						<label class="label" for="employmentType">{m.employees_employmentType()}</label>
						<select class="input" id="employmentType" bind:value={newEmploymentType}>
							<option value="FULL_TIME">{m.employees_fullTime()}</option>
							<option value="PART_TIME">{m.employees_partTime()}</option>
							<option value="CONTRACT">{m.employees_contract()}</option>
						</select>
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="position">{m.employees_position()}</label>
						<input
							class="input"
							type="text"
							id="position"
							bind:value={newPosition}
							placeholder="Software Developer"
						/>
					</div>
					<div class="form-group">
						<label class="label" for="department">{m.employees_department()}</label>
						<input
							class="input"
							type="text"
							id="department"
							bind:value={newDepartment}
							placeholder="Engineering"
						/>
					</div>
				</div>

				<h3 class="section-title">{m.employees_taxSettings()}</h3>
				<div class="form-group">
					<label class="checkbox-label">
						<input type="checkbox" bind:checked={newApplyBasicExemption} />
						{m.employees_applyBasicExemption()}
					</label>
				</div>

				{#if newApplyBasicExemption}
					<div class="form-group">
						<label class="label" for="basicExemption">{m.employees_basicExemptionAmount()}</label>
						<input
							class="input"
							type="number"
							id="basicExemption"
							bind:value={newBasicExemptionAmount}
							step="0.01"
							min="0"
							max="700"
						/>
						<small class="help-text">{m.employees_basicExemptionHelp()}</small>
					</div>
				{/if}

				<div class="form-group">
					<label class="label" for="pensionRate">{m.employees_fundedPensionRate()}</label>
					<select class="input" id="pensionRate" bind:value={newFundedPensionRate}>
						<option value="0">{m.employees_notEnrolled()}</option>
						<option value="0.02">{m.employees_standard2()}</option>
						<option value="0.04">{m.employees_increased4()}</option>
					</select>
				</div>

				<div class="modal-actions">
					<button
						type="button"
						class="btn btn-secondary"
						onclick={() => (showCreateEmployee = false)}
					>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary">{m.employees_addEmployee()}</button>
				</div>
			</form>
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

	.search-input {
		flex: 1;
	}

	.checkbox-label {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		cursor: pointer;
		white-space: nowrap;
	}

	.name {
		font-weight: 500;
	}

	.mono {
		font-family: var(--font-mono);
		font-size: 0.875rem;
	}

	.email-sub,
	.dept-sub {
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}

	.text-muted {
		color: var(--color-text-muted);
		font-style: italic;
	}

	.inactive {
		opacity: 0.6;
	}

	.badge-active {
		background: #dcfce7;
		color: #166534;
	}

	.badge-inactive {
		background: #fef2f2;
		color: #991b1b;
	}

	.empty-state {
		text-align: center;
		padding: 3rem;
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
		max-width: 700px;
		margin: 1rem;
		max-height: 90vh;
		overflow-y: auto;
	}

	.modal h2 {
		margin-bottom: 1.5rem;
	}

	.section-title {
		font-size: 1rem;
		font-weight: 600;
		margin: 1.5rem 0 1rem;
		padding-bottom: 0.5rem;
		border-bottom: 1px solid var(--color-border);
	}

	.section-title:first-of-type {
		margin-top: 0;
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
		padding-top: 1rem;
		border-top: 1px solid var(--color-border);
	}

	/* Mobile styles */
	@media (max-width: 768px) {
		.header {
			flex-direction: column;
			align-items: stretch;
			gap: 1rem;
		}

		.header h1 {
			font-size: 1.5rem;
		}

		.header .btn {
			width: 100%;
			justify-content: center;
			min-height: 44px;
		}

		.filter-row {
			flex-direction: column;
			align-items: stretch;
		}

		.checkbox-label {
			padding: 0.5rem 0;
		}

		.form-row {
			flex-direction: column;
		}

		.modal {
			margin: 0;
			max-width: 100%;
			border-radius: 1rem 1rem 0 0;
		}

		.modal-backdrop {
			align-items: flex-end;
			padding: 0;
		}

		.modal-actions {
			flex-direction: column;
		}

		.modal-actions .btn {
			width: 100%;
			min-height: 44px;
			justify-content: center;
		}
	}
</style>
