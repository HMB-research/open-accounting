<script lang="ts">
	import { page } from '$app/stores';
	import {
		api,
		type AbsenceType,
		type LeaveBalance,
		type LeaveRecord,
		type LeaveStatus,
		type Employee
	} from '$lib/api';
	import Decimal from 'decimal.js';
	import * as m from '$lib/paraglide/messages.js';
	import StatusBadge, { type StatusConfig } from '$lib/components/StatusBadge.svelte';

	let absenceTypes = $state<AbsenceType[]>([]);
	let leaveBalances = $state<LeaveBalance[]>([]);
	let leaveRecords = $state<LeaveRecord[]>([]);
	let employees = $state<Employee[]>([]);
	let isLoading = $state(true);
	let error = $state('');

	// Filters
	let filterYear = $state(new Date().getFullYear());
	let selectedEmployeeId = $state('');
	let selectedTab = $state<'records' | 'balances'>('records');

	// Create leave request modal
	let showCreateRequest = $state(false);
	let newEmployeeId = $state('');
	let newAbsenceTypeId = $state('');
	let newStartDate = $state('');
	let newEndDate = $state('');
	let newTotalDays = $state('1');
	let newWorkingDays = $state('1');
	let newDocumentNumber = $state('');
	let newNotes = $state('');

	// Reject modal
	let showRejectModal = $state(false);
	let selectedRecordId = $state('');
	let rejectionReason = $state('');

	$effect(() => {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadData(tenantId);
		}
	});

	async function loadData(tenantId: string) {
		isLoading = true;
		error = '';

		try {
			const [typesData, employeesData, recordsData] = await Promise.all([
				api.listAbsenceTypes(tenantId, true),
				api.listEmployees(tenantId, true),
				api.listLeaveRecords(tenantId, selectedEmployeeId || undefined, filterYear)
			]);
			absenceTypes = typesData;
			employees = employeesData;
			leaveRecords = recordsData;

			// Load balances if an employee is selected
			if (selectedEmployeeId) {
				leaveBalances = await api.listLeaveBalances(tenantId, selectedEmployeeId, filterYear);
			} else {
				leaveBalances = [];
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load data';
		} finally {
			isLoading = false;
		}
	}

	async function handleFilterChange() {
		const tenantId = $page.url.searchParams.get('tenant');
		if (tenantId) {
			loadData(tenantId);
		}
	}

	async function createLeaveRequest(e: Event) {
		e.preventDefault();
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const record = await api.createLeaveRecord(tenantId, {
				employee_id: newEmployeeId,
				absence_type_id: newAbsenceTypeId,
				start_date: newStartDate,
				end_date: newEndDate,
				total_days: new Decimal(newTotalDays),
				working_days: new Decimal(newWorkingDays),
				document_number: newDocumentNumber || undefined,
				notes: newNotes || undefined
			});
			leaveRecords = [record, ...leaveRecords];
			showCreateRequest = false;
			resetForm();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create leave request';
		}
	}

	function resetForm() {
		newEmployeeId = '';
		newAbsenceTypeId = '';
		newStartDate = '';
		newEndDate = '';
		newTotalDays = '1';
		newWorkingDays = '1';
		newDocumentNumber = '';
		newNotes = '';
	}

	async function approveRecord(recordId: string) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const updated = await api.approveLeaveRecord(tenantId, recordId);
			leaveRecords = leaveRecords.map((r) => (r.id === updated.id ? updated : r));
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to approve leave request';
		}
	}

	function openRejectModal(recordId: string) {
		selectedRecordId = recordId;
		rejectionReason = '';
		showRejectModal = true;
	}

	async function rejectRecord() {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId || !selectedRecordId || !rejectionReason) return;

		try {
			const updated = await api.rejectLeaveRecord(tenantId, selectedRecordId, rejectionReason);
			leaveRecords = leaveRecords.map((r) => (r.id === updated.id ? updated : r));
			showRejectModal = false;
			selectedRecordId = '';
			rejectionReason = '';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to reject leave request';
		}
	}

	async function cancelRecord(recordId: string) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const updated = await api.cancelLeaveRecord(tenantId, recordId);
			leaveRecords = leaveRecords.map((r) => (r.id === updated.id ? updated : r));
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to cancel leave request';
		}
	}

	async function initializeBalances(employeeId: string) {
		const tenantId = $page.url.searchParams.get('tenant');
		if (!tenantId) return;

		try {
			const balances = await api.initializeLeaveBalances(tenantId, employeeId, filterYear);
			leaveBalances = balances;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to initialize leave balances';
		}
	}

	function formatDecimal(value: Decimal | string | number): string {
		if (value instanceof Decimal) {
			return value.toFixed(1);
		}
		return new Decimal(value).toFixed(1);
	}

	function formatDate(dateStr: string): string {
		return new Date(dateStr).toLocaleDateString();
	}

	const statusConfig: Record<LeaveStatus, StatusConfig> = {
		PENDING: { class: 'badge-pending', label: m.leave_status_pending() },
		APPROVED: { class: 'badge-approved', label: m.leave_status_approved() },
		REJECTED: { class: 'badge-rejected', label: m.leave_status_rejected() },
		CANCELLED: { class: 'badge-cancelled', label: m.leave_status_cancelled() }
	};

	function getAbsenceTypeName(typeId: string): string {
		const type = absenceTypes.find((t) => t.id === typeId);
		return type?.name || typeId;
	}

	function getEmployeeName(employeeId: string): string {
		const emp = employees.find((e) => e.id === employeeId);
		return emp ? `${emp.last_name}, ${emp.first_name}` : employeeId;
	}

	function canApprove(record: LeaveRecord): boolean {
		return record.status === 'PENDING';
	}

	function canReject(record: LeaveRecord): boolean {
		return record.status === 'PENDING';
	}

	function canCancel(record: LeaveRecord): boolean {
		return record.status === 'PENDING' || record.status === 'APPROVED';
	}

	// Calculate days when dates change
	$effect(() => {
		if (newStartDate && newEndDate) {
			const start = new Date(newStartDate);
			const end = new Date(newEndDate);
			if (end >= start) {
				const diffTime = Math.abs(end.getTime() - start.getTime());
				const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24)) + 1;
				newTotalDays = String(diffDays);
				// Estimate working days (rough: exclude weekends)
				let workDays = 0;
				const current = new Date(start);
				while (current <= end) {
					const day = current.getDay();
					if (day !== 0 && day !== 6) workDays++;
					current.setDate(current.getDate() + 1);
				}
				newWorkingDays = String(workDays);
			}
		}
	});
</script>

<svelte:head>
	<title>{m.leave_title()} - Open Accounting</title>
</svelte:head>

<div class="container">
	<div class="header">
		<h1>{m.leave_title()}</h1>
		<button class="btn btn-primary" onclick={() => (showCreateRequest = true)}>
			+ {m.leave_request()}
		</button>
	</div>

	<div class="filters card">
		<div class="filter-row">
			<div class="filter-group">
				<label class="label" for="yearFilter">{m.payroll_year()}</label>
				<select class="input" id="yearFilter" bind:value={filterYear} onchange={handleFilterChange}>
					{#each Array.from({ length: 5 }, (_, i) => new Date().getFullYear() - i) as year}
						<option value={year}>{year}</option>
					{/each}
				</select>
			</div>
			<div class="filter-group">
				<label class="label" for="employeeFilter">{m.leave_employee()}</label>
				<select
					class="input"
					id="employeeFilter"
					bind:value={selectedEmployeeId}
					onchange={handleFilterChange}
				>
					<option value="">All Employees</option>
					{#each employees as emp}
						<option value={emp.id}>{emp.last_name}, {emp.first_name}</option>
					{/each}
				</select>
			</div>
		</div>
	</div>

	<div class="tabs">
		<button
			class="tab"
			class:active={selectedTab === 'records'}
			onclick={() => (selectedTab = 'records')}
		>
			{m.leave_records()}
		</button>
		<button
			class="tab"
			class:active={selectedTab === 'balances'}
			onclick={() => (selectedTab = 'balances')}
		>
			{m.leave_balances()}
		</button>
	</div>

	{#if error}
		<div class="alert alert-error">{error}</div>
	{/if}

	{#if isLoading}
		<p>Loading...</p>
	{:else if selectedTab === 'records'}
		{#if leaveRecords.length === 0}
			<div class="empty-state card">
				<p>{m.leave_no_records()}</p>
			</div>
		{:else}
			<div class="card table-container">
				<table class="table table-mobile-cards">
					<thead>
						<tr>
							<th>{m.leave_employee()}</th>
							<th>{m.leave_type()}</th>
							<th>{m.leave_start_date()}</th>
							<th>{m.leave_end_date()}</th>
							<th class="text-right">{m.leave_working_days()}</th>
							<th>{m.leave_status()}</th>
							<th>Actions</th>
						</tr>
					</thead>
					<tbody>
						{#each leaveRecords as record}
							<tr>
								<td class="employee-name" data-label={m.leave_employee()}>
									{record.employee
										? `${record.employee.last_name}, ${record.employee.first_name}`
										: getEmployeeName(record.employee_id)}
								</td>
								<td data-label={m.leave_type()}>
									{record.absence_type?.name || getAbsenceTypeName(record.absence_type_id)}
								</td>
								<td data-label={m.leave_start_date()}>{formatDate(record.start_date)}</td>
								<td data-label={m.leave_end_date()}>{formatDate(record.end_date)}</td>
								<td class="text-right mono" data-label={m.leave_working_days()}>
									{formatDecimal(record.working_days)}
								</td>
								<td data-label={m.leave_status()}>
									<StatusBadge status={record.status} config={statusConfig} />
								</td>
								<td class="actions actions-cell">
									{#if canApprove(record)}
										<button
											class="btn btn-small btn-success"
											onclick={() => approveRecord(record.id)}
										>
											{m.leave_approve()}
										</button>
									{/if}
									{#if canReject(record)}
										<button
											class="btn btn-small btn-danger"
											onclick={() => openRejectModal(record.id)}
										>
											{m.leave_reject()}
										</button>
									{/if}
									{#if canCancel(record)}
										<button
											class="btn btn-small btn-secondary"
											onclick={() => cancelRecord(record.id)}
										>
											{m.leave_cancel()}
										</button>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	{:else if selectedTab === 'balances'}
		{#if !selectedEmployeeId}
			<div class="empty-state card">
				<p>Please select an employee to view leave balances</p>
			</div>
		{:else if leaveBalances.length === 0}
			<div class="empty-state card">
				<p>{m.leave_no_balances()}</p>
				<button class="btn btn-primary" onclick={() => initializeBalances(selectedEmployeeId)}>
					{m.leave_initialize_balances()}
				</button>
			</div>
		{:else}
			<div class="card table-container">
				<table class="table table-mobile-cards">
					<thead>
						<tr>
							<th>{m.leave_type()}</th>
							<th class="text-right">{m.leave_entitled()}</th>
							<th class="text-right">{m.leave_carryover()}</th>
							<th class="text-right">{m.leave_used()}</th>
							<th class="text-right">{m.leave_pending()}</th>
							<th class="text-right">{m.leave_remaining()}</th>
						</tr>
					</thead>
					<tbody>
						{#each leaveBalances as balance}
							<tr>
								<td data-label={m.leave_type()}>
									{balance.absence_type?.name || getAbsenceTypeName(balance.absence_type_id)}
								</td>
								<td class="text-right mono" data-label={m.leave_entitled()}>
									{formatDecimal(balance.entitled_days)}
								</td>
								<td class="text-right mono" data-label={m.leave_carryover()}>
									{formatDecimal(balance.carryover_days)}
								</td>
								<td class="text-right mono" data-label={m.leave_used()}>
									{formatDecimal(balance.used_days)}
								</td>
								<td class="text-right mono" data-label={m.leave_pending()}>
									{formatDecimal(balance.pending_days)}
								</td>
								<td class="text-right mono remaining" data-label={m.leave_remaining()}>
									{formatDecimal(balance.remaining_days)}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	{/if}
</div>

{#if showCreateRequest}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showCreateRequest = false)} role="presentation">
		<div
			class="modal card"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
			aria-labelledby="create-request-title"
			tabindex="-1"
		>
			<h2 id="create-request-title">{m.leave_request()}</h2>
			<form onsubmit={createLeaveRequest}>
				<div class="form-group">
					<label class="label" for="employee">{m.leave_employee()} *</label>
					<select class="input" id="employee" bind:value={newEmployeeId} required>
						<option value="">Select employee...</option>
						{#each employees as emp}
							<option value={emp.id}>{emp.last_name}, {emp.first_name}</option>
						{/each}
					</select>
				</div>

				<div class="form-group">
					<label class="label" for="absenceType">{m.leave_type()} *</label>
					<select class="input" id="absenceType" bind:value={newAbsenceTypeId} required>
						<option value="">Select type...</option>
						{#each absenceTypes as type}
							<option value={type.id}>{type.name}</option>
						{/each}
					</select>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="startDate">{m.leave_start_date()} *</label>
						<input class="input" type="date" id="startDate" bind:value={newStartDate} required />
					</div>
					<div class="form-group">
						<label class="label" for="endDate">{m.leave_end_date()} *</label>
						<input class="input" type="date" id="endDate" bind:value={newEndDate} required />
					</div>
				</div>

				<div class="form-row">
					<div class="form-group">
						<label class="label" for="totalDays">{m.leave_total_days()}</label>
						<input
							class="input"
							type="number"
							id="totalDays"
							bind:value={newTotalDays}
							min="0.5"
							step="0.5"
						/>
					</div>
					<div class="form-group">
						<label class="label" for="workingDays">{m.leave_working_days()}</label>
						<input
							class="input"
							type="number"
							id="workingDays"
							bind:value={newWorkingDays}
							min="0.5"
							step="0.5"
						/>
					</div>
				</div>

				<div class="form-group">
					<label class="label" for="documentNumber">{m.leave_document_number()}</label>
					<input class="input" type="text" id="documentNumber" bind:value={newDocumentNumber} />
				</div>

				<div class="form-group">
					<label class="label" for="notes">{m.leave_notes()}</label>
					<textarea class="input" id="notes" bind:value={newNotes} rows="2"></textarea>
				</div>

				<div class="modal-actions">
					<button
						type="button"
						class="btn btn-secondary"
						onclick={() => (showCreateRequest = false)}
					>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-primary">{m.leave_request()}</button>
				</div>
			</form>
		</div>
	</div>
{/if}

{#if showRejectModal}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="modal-backdrop" onclick={() => (showRejectModal = false)} role="presentation">
		<div
			class="modal card"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			aria-modal="true"
			aria-labelledby="reject-title"
			tabindex="-1"
		>
			<h2 id="reject-title">{m.leave_reject()}</h2>
			<form
				onsubmit={(e) => {
					e.preventDefault();
					rejectRecord();
				}}
			>
				<div class="form-group">
					<label class="label" for="reason">{m.leave_rejection_reason()} *</label>
					<textarea class="input" id="reason" bind:value={rejectionReason} rows="3" required
					></textarea>
				</div>

				<div class="modal-actions">
					<button
						type="button"
						class="btn btn-secondary"
						onclick={() => (showRejectModal = false)}
					>
						{m.common_cancel()}
					</button>
					<button type="submit" class="btn btn-danger">{m.leave_reject()}</button>
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
		gap: 1.5rem;
		align-items: flex-end;
	}

	.filter-group {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.filter-group .input {
		min-width: 200px;
	}

	.tabs {
		display: flex;
		gap: 0;
		margin-bottom: 1.5rem;
		border-bottom: 1px solid var(--color-border);
	}

	.tab {
		padding: 0.75rem 1.5rem;
		background: none;
		border: none;
		border-bottom: 2px solid transparent;
		cursor: pointer;
		font-size: 0.875rem;
		color: var(--color-text-muted);
		transition: all 0.2s;
	}

	.tab:hover {
		color: var(--color-text);
	}

	.tab.active {
		color: var(--color-primary);
		border-bottom-color: var(--color-primary);
	}

	.employee-name {
		font-weight: 500;
	}

	.mono {
		font-family: var(--font-mono);
		font-size: 0.875rem;
	}

	.text-right {
		text-align: right;
	}

	.remaining {
		font-weight: 600;
		color: var(--color-primary);
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

	.btn-danger {
		background: #dc2626;
		color: white;
	}

	.btn-danger:hover {
		background: #b91c1c;
	}

	.badge-pending {
		background: #fef3c7;
		color: #92400e;
	}

	.badge-approved {
		background: #dcfce7;
		color: #166534;
	}

	.badge-rejected {
		background: #fef2f2;
		color: #991b1b;
	}

	.badge-cancelled {
		background: #f3f4f6;
		color: #374151;
	}

	.empty-state {
		text-align: center;
		padding: 3rem;
		color: var(--color-text-muted);
	}

	.empty-state .btn {
		margin-top: 1rem;
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

	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 1.5rem;
	}

	textarea.input {
		resize: vertical;
		min-height: 60px;
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
			gap: 0.75rem;
			align-items: stretch;
		}

		.filter-group .input {
			min-width: unset;
			width: 100%;
			min-height: 44px;
		}

		.tabs {
			overflow-x: auto;
		}

		.tab {
			flex: 1;
			text-align: center;
			min-height: 44px;
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
