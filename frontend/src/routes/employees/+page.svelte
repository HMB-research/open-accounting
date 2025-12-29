<script lang="ts">
	import { page } from '$app/stores';
	import { api, type Employee, type EmploymentType } from '$lib/api';
	import Decimal from 'decimal.js';

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
			error = err instanceof Error ? err.message : 'Failed to load employees';
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
			error = err instanceof Error ? err.message : 'Failed to create employee';
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

	const typeLabels: Record<EmploymentType, string> = {
		FULL_TIME: 'Full-time',
		PART_TIME: 'Part-time',
		CONTRACT: 'Contract'
	};

	const typeBadgeClass: Record<EmploymentType, string> = {
		FULL_TIME: 'badge-fulltime',
		PART_TIME: 'badge-parttime',
		CONTRACT: 'badge-contract'
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
	<title>Employees - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>Employees</h1>
		<button class="btn btn-primary" onclick={() => (showCreateEmployee = true)}>
			+ New Employee
		</button>
	</div>

	<div class="filters card">
		<div class="filter-row">
			<label class="checkbox-label">
				<input type="checkbox" bind:checked={showActiveOnly} onchange={handleFilter} />
				Active only
			</label>
			<input
				class="input search-input"
				type="text"
				placeholder="Search employees..."
				bind:value={searchQuery}
			/>
		</div>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<p>Loading employees...</p>
	{:else if employees.length === 0}
		<div class="empty-state card">
			<p>No employees found. Add your first employee to get started with payroll.</p>
		</div>
	{:else}
		<div class="card">
			<table class="table">
				<thead>
					<tr>
						<th>Name</th>
						<th>Personal Code</th>
						<th>Position</th>
						<th>Type</th>
						<th>Basic Exemption</th>
						<th>Pension Rate</th>
						<th>Status</th>
					</tr>
				</thead>
				<tbody>
					{#each employees as employee}
						<tr class:inactive={!employee.is_active}>
							<td class="name">
								{employee.last_name}, {employee.first_name}
								{#if employee.email}
									<div class="email-sub">{employee.email}</div>
								{/if}
							</td>
							<td class="mono">{employee.personal_code || '-'}</td>
							<td>
								{employee.position || '-'}
								{#if employee.department}
									<div class="dept-sub">{employee.department}</div>
								{/if}
							</td>
							<td>
								<span class="badge {typeBadgeClass[employee.employment_type]}">
									{typeLabels[employee.employment_type]}
								</span>
							</td>
							<td>
								{#if employee.apply_basic_exemption}
									{formatDecimal(employee.basic_exemption_amount)}
								{:else}
									<span class="text-muted">Not applied</span>
								{/if}
							</td>
							<td>{formatPercent(employee.funded_pension_rate)}</td>
							<td>
								<span class="badge {employee.is_active ? 'badge-active' : 'badge-inactive'}">
									{employee.is_active ? 'Active' : 'Inactive'}
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
			<h2 id="create-employee-title">Add New Employee</h2>
			<form onsubmit={createEmployee}>
				<h3 class="section-title">Personal Information</h3>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="firstName">First Name *</label>
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
						<label class="label" for="lastName">Last Name *</label>
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
						<label class="label" for="personalCode">Personal Code (Isikukood)</label>
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
						<label class="label" for="email">Email</label>
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
						<label class="label" for="phone">Phone</label>
						<input
							class="input"
							type="tel"
							id="phone"
							bind:value={newPhone}
							placeholder="+372 5551234"
						/>
					</div>
					<div class="form-group">
						<label class="label" for="bankAccount">Bank Account (IBAN)</label>
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
					<label class="label" for="address">Address</label>
					<input
						class="input"
						type="text"
						id="address"
						bind:value={newAddress}
						placeholder="Pikk tn 1, 10111 Tallinn"
					/>
				</div>

				<h3 class="section-title">Employment Details</h3>
				<div class="form-row">
					<div class="form-group">
						<label class="label" for="startDate">Start Date *</label>
						<input class="input" type="date" id="startDate" bind:value={newStartDate} required />
					</div>
					<div class="form-group">
						<label class="label" for="employmentType">Employment Type</label>
						<select class="input" id="employmentType" bind:value={newEmploymentType}>
							<option value="FULL_TIME">Full-time</option>
							<option value="PART_TIME">Part-time</option>
							<option value="CONTRACT">Contract</option>
						</select>
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="position">Position</label>
						<input
							class="input"
							type="text"
							id="position"
							bind:value={newPosition}
							placeholder="Software Developer"
						/>
					</div>
					<div class="form-group">
						<label class="label" for="department">Department</label>
						<input
							class="input"
							type="text"
							id="department"
							bind:value={newDepartment}
							placeholder="Engineering"
						/>
					</div>
				</div>

				<h3 class="section-title">Tax Settings</h3>
				<div class="form-group">
					<label class="checkbox-label">
						<input type="checkbox" bind:checked={newApplyBasicExemption} />
						Apply Basic Exemption (Maksuvaba tulu)
					</label>
				</div>

				{#if newApplyBasicExemption}
					<div class="form-group">
						<label class="label" for="basicExemption">Basic Exemption Amount (EUR/month)</label>
						<input
							class="input"
							type="number"
							id="basicExemption"
							bind:value={newBasicExemptionAmount}
							step="0.01"
							min="0"
							max="700"
						/>
						<small class="help-text">Maximum 700 EUR in 2025</small>
					</div>
				{/if}

				<div class="form-group">
					<label class="label" for="pensionRate">Funded Pension Rate (II pillar)</label>
					<select class="input" id="pensionRate" bind:value={newFundedPensionRate}>
						<option value="0">Not enrolled (0%)</option>
						<option value="0.02">Standard (2%)</option>
						<option value="0.04">Increased (4%)</option>
					</select>
				</div>

				<div class="modal-actions">
					<button
						type="button"
						class="btn btn-secondary"
						onclick={() => (showCreateEmployee = false)}
					>
						Cancel
					</button>
					<button type="submit" class="btn btn-primary">Add Employee</button>
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

	.badge-fulltime {
		background: #dcfce7;
		color: #166534;
	}

	.badge-parttime {
		background: #fef3c7;
		color: #92400e;
	}

	.badge-contract {
		background: #e0e7ff;
		color: #3730a3;
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
</style>
