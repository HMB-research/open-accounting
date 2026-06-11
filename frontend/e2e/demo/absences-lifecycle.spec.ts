import { test, expect, type Page, type Response, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, ensureDemoTenant, navigateTo, waitForRouteReady } from './utils';

interface AbsenceTypeResponse {
	id: string;
	code: string;
	name: string;
	requires_document: boolean;
}

interface EmployeeResponse {
	id: string;
	employee_number?: string;
	first_name: string;
	last_name: string;
}

interface LeaveRecordResponse {
	id: string;
	employee_id: string;
	absence_type_id: string;
	start_date: string;
	end_date: string;
	total_days: string | number;
	working_days: string | number;
	status: 'PENDING' | 'APPROVED' | 'REJECTED' | 'CANCELLED';
	document_number?: string;
	notes?: string;
	rejection_reason?: string;
}

interface LeaveBalanceResponse {
	id: string;
	absence_type_id: string;
	year: number;
	entitled_days: string | number;
	carryover_days: string | number;
	used_days: string | number;
	pending_days: string | number;
	remaining_days: string | number;
	absence_type?: AbsenceTypeResponse;
}

interface ImportLeaveBalancesResult {
	rows_processed: number;
	leave_balances_created: number;
	leave_balances_updated: number;
	rows_skipped: number;
}

function responsePath(response: Response): string {
	return new URL(response.url()).pathname;
}

function listAbsenceTypesResponse(response: Response): boolean {
	return (
		response.request().method() === 'GET' &&
		response.status() === 200 &&
		responsePath(response).endsWith('/absence-types')
	);
}

function listEmployeesResponse(response: Response): boolean {
	const url = new URL(response.url());
	return (
		response.request().method() === 'GET' &&
		response.status() === 200 &&
		responsePath(response).endsWith('/employees') &&
		url.searchParams.get('active_only') === 'true'
	);
}

function listLeaveRecordsResponse(response: Response): boolean {
	return (
		response.request().method() === 'GET' &&
		response.status() === 200 &&
		responsePath(response).endsWith('/leave-records')
	);
}

function createLeaveRecordResponse(response: Response): boolean {
	return (
		response.request().method() === 'POST' &&
		/\/api\/v1\/tenants\/[^/]+\/leave-records$/.test(responsePath(response))
	);
}

function transitionLeaveRecordResponse(recordId: string, transition: 'approve' | 'reject' | 'cancel') {
	return (response: Response): boolean =>
		response.request().method() === 'POST' &&
		responsePath(response).endsWith(`/leave-records/${recordId}/${transition}`);
}

function leaveBalancesResponse(employeeId: string, year: number) {
	return (response: Response): boolean => {
		const url = new URL(response.url());
		return (
			response.request().method() === 'GET' &&
			response.status() === 200 &&
			responsePath(response).endsWith(`/employees/${employeeId}/leave-balances`) &&
			url.searchParams.get('year') === String(year)
		);
	};
}

function initializeLeaveBalancesResponse(employeeId: string, year: number) {
	return (response: Response): boolean =>
		response.request().method() === 'POST' &&
		responsePath(response).endsWith(`/employees/${employeeId}/leave-balances/${year}/initialize`);
}

function importLeaveBalancesResponse(response: Response): boolean {
	return (
		response.request().method() === 'POST' &&
		responsePath(response).endsWith('/leave-balances/import')
	);
}

async function openAbsencesPage(
	page: Page,
	testInfo: TestInfo
): Promise<{ absenceTypes: AbsenceTypeResponse[]; employees: EmployeeResponse[] }> {
	const typesLoaded = page.waitForResponse(listAbsenceTypesResponse);
	const employeesLoaded = page.waitForResponse(listEmployeesResponse);
	const recordsLoaded = page.waitForResponse(listLeaveRecordsResponse);

	await navigateTo(page, '/employees/absences', testInfo, { waitForNetworkIdle: false });
	const [typesResponse, employeesResponse] = await Promise.all([
		typesLoaded,
		employeesLoaded,
		recordsLoaded
	]).then(([types, employees]) => [types, employees]);
	await waitForRouteReady(page, 'main h1, #yearFilter, #employeeFilter, .tabs', 15000);
	await page.getByText(/^Loading\.\.\.$|^Laadimine\.\.\.$/i).waitFor({
		state: 'hidden',
		timeout: 15000
	}).catch(() => {});

	return {
		absenceTypes: (await typesResponse.json()) as AbsenceTypeResponse[],
		employees: (await employeesResponse.json()) as EmployeeResponse[]
	};
}

function displayName(employee: EmployeeResponse): string {
	return `${employee.last_name}, ${employee.first_name}`;
}

function displayDate(apiDate: string): string {
	return new Date(apiDate).toLocaleDateString('et-EE');
}

function leaveRecordRow(page: Page, employee: EmployeeResponse, record: LeaveRecordResponse) {
	return page
		.locator('table tbody tr')
		.filter({ hasText: displayName(employee) })
		.filter({ hasText: displayDate(record.start_date) })
		.filter({ hasText: record.document_number || '' });
}

async function createLeaveRequest(
	page: Page,
	options: {
		employee: EmployeeResponse;
		absenceType: AbsenceTypeResponse;
		startDate: string;
		endDate: string;
		totalDays: string;
		workingDays: string;
		documentNumber: string;
		notes: string;
	}
): Promise<LeaveRecordResponse> {
	await page.getByRole('button', { name: /request leave|\+/i }).click();
	const dialog = page.getByRole('dialog', { name: /request leave/i });
	await expect(dialog).toBeVisible();

	await dialog.locator('#employee').selectOption(options.employee.id);
	await dialog.locator('#absenceType').selectOption(options.absenceType.id);
	await dialog.locator('#startDate').fill(options.startDate);
	await dialog.locator('#endDate').fill(options.endDate);
	await dialog.locator('#totalDays').fill(options.totalDays);
	await dialog.locator('#workingDays').fill(options.workingDays);
	await dialog.locator('#documentNumber').fill(options.documentNumber);
	await dialog.locator('#notes').fill(options.notes);

	const createResponsePromise = page.waitForResponse(createLeaveRecordResponse);
	await dialog.getByRole('button', { name: /^request leave$/i }).click();
	const createResponse = await createResponsePromise;
	expect(createResponse.status()).toBe(201);

	const payload = createResponse.request().postDataJSON() as Record<string, unknown>;
	expect(payload.employee_id).toBe(options.employee.id);
	expect(payload.absence_type_id).toBe(options.absenceType.id);
	expect(payload.start_date).toBe(`${options.startDate}T00:00:00Z`);
	expect(payload.end_date).toBe(`${options.endDate}T00:00:00Z`);
	expect(payload.document_number).toBe(options.documentNumber);
	expect(payload.notes).toBe(options.notes);

	const record = (await createResponse.json()) as LeaveRecordResponse;
	expect(record.status).toBe('PENDING');
	expect(record.employee_id).toBe(options.employee.id);
	expect(record.absence_type_id).toBe(options.absenceType.id);
	expect(Number(record.working_days)).toBe(Number(options.workingDays));
	expect(record.document_number).toBe(options.documentNumber);
	expect(record.notes).toBe(options.notes);

	await expect(dialog).toBeHidden();

	const row = leaveRecordRow(page, options.employee, record);
	await expect(row).toBeVisible({ timeout: 10000 });
	await expect(row).toContainText(/pending/i);
	await expect(row).toContainText(options.absenceType.name);
	await expect(row).toContainText(options.documentNumber);
	await expect(row).toContainText(Number(options.workingDays).toFixed(1));

	return record;
}

async function transitionRecord(
	page: Page,
	employee: EmployeeResponse,
	record: LeaveRecordResponse,
	transition: 'approve' | 'cancel'
): Promise<LeaveRecordResponse> {
	const row = leaveRecordRow(page, employee, record);
	await expect(row).toBeVisible({ timeout: 10000 });

	const responsePromise = page.waitForResponse(transitionLeaveRecordResponse(record.id, transition));
	await row.getByRole('button', { name: new RegExp(`^${transition}$`, 'i') }).click();
	const response = await responsePromise;
	expect(response.status()).toBe(200);

	const updated = (await response.json()) as LeaveRecordResponse;
	expect(updated.id).toBe(record.id);
	expect(updated.status).toBe(transition === 'approve' ? 'APPROVED' : 'CANCELLED');

	await expect(row).toContainText(transition === 'approve' ? /approved/i : /cancelled/i, {
		timeout: 10000
	});
	return updated;
}

async function rejectRecord(
	page: Page,
	employee: EmployeeResponse,
	record: LeaveRecordResponse,
	reason: string
): Promise<LeaveRecordResponse> {
	const row = leaveRecordRow(page, employee, record);
	await expect(row).toBeVisible({ timeout: 10000 });

	await row.getByRole('button', { name: /^reject$/i }).click();
	const dialog = page.getByRole('dialog', { name: /^reject$/i });
	await expect(dialog).toBeVisible();
	await dialog.locator('#reason').fill(reason);

	const responsePromise = page.waitForResponse(transitionLeaveRecordResponse(record.id, 'reject'));
	await dialog.getByRole('button', { name: /^reject$/i }).click();
	const response = await responsePromise;
	expect(response.status()).toBe(200);

	const updated = (await response.json()) as LeaveRecordResponse;
	expect(updated.id).toBe(record.id);
	expect(updated.status).toBe('REJECTED');
	expect(updated.rejection_reason).toBe(reason);

	await expect(dialog).toBeHidden();
	await expect(row).toContainText(/rejected/i, { timeout: 10000 });
	return updated;
}

async function openBalancesForEmployee(
	page: Page,
	employee: EmployeeResponse,
	year: number
): Promise<LeaveBalanceResponse[]> {
	const responsePromise = page.waitForResponse(leaveBalancesResponse(employee.id, year));
	await page.locator('#employeeFilter').selectOption(employee.id);
	const response = await responsePromise;
	const balances = (await response.json()) as LeaveBalanceResponse[];

	const balancesTab = page.getByRole('button', { name: /leave balances/i });
	await balancesTab.click();
	await expect(balancesTab).toHaveClass(/active/);
	await expect(page.locator('table, .empty-state').first()).toBeVisible({ timeout: 10000 });

	return balances;
}

function balanceRow(page: Page, absenceTypeName: string) {
	return page.locator('table tbody tr').filter({ hasText: absenceTypeName });
}

test.describe('Demo Leave Management - Lifecycle Workflows', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('creates, approves, rejects, cancels, initializes, and imports leave balances', async ({
		page
	}, testInfo) => {
		const currentYear = new Date().getFullYear();
		const { absenceTypes, employees } = await openAbsencesPage(page, testInfo);
		const annualLeave = absenceTypes.find((type) => type.code === 'ANNUAL_LEAVE');
		expect(annualLeave).toBeTruthy();
		expect(annualLeave?.requires_document).toBe(false);
		expect(employees.length).toBeGreaterThanOrEqual(4);

		const suffix = `${testInfo.workerIndex}-${testInfo.retry}-${Date.now()}`;
		const approveEmployee = employees[0];
		const rejectEmployee = employees[1];
		const cancelEmployee = employees[2];
		const initializeEmployee = employees[3];
		const importEmployee = employees.find((employee) => employee.employee_number === 'EMP002') || rejectEmployee;

		const approvedRecord = await createLeaveRequest(page, {
			employee: approveEmployee,
			absenceType: annualLeave!,
			startDate: `${currentYear}-03-02`,
			endDate: `${currentYear}-03-04`,
			totalDays: '3',
			workingDays: '3',
			documentNumber: `E2E-APP-${suffix}`,
			notes: `Approve lifecycle ${suffix}`
		});
		await transitionRecord(page, approveEmployee, approvedRecord, 'approve');
		await expect(
			leaveRecordRow(page, approveEmployee, approvedRecord).getByRole('button', { name: /^approve$/i })
		).toHaveCount(0);

		const rejectedRecord = await createLeaveRequest(page, {
			employee: rejectEmployee,
			absenceType: annualLeave!,
			startDate: `${currentYear}-04-06`,
			endDate: `${currentYear}-04-07`,
			totalDays: '2',
			workingDays: '2',
			documentNumber: `E2E-REJ-${suffix}`,
			notes: `Reject lifecycle ${suffix}`
		});
		await rejectRecord(page, rejectEmployee, rejectedRecord, `Staffing coverage ${suffix}`);

		const cancelledRecord = await createLeaveRequest(page, {
			employee: cancelEmployee,
			absenceType: annualLeave!,
			startDate: `${currentYear}-05-04`,
			endDate: `${currentYear}-05-04`,
			totalDays: '1',
			workingDays: '1',
			documentNumber: `E2E-CAN-${suffix}`,
			notes: `Cancel lifecycle ${suffix}`
		});
		await transitionRecord(page, cancelEmployee, cancelledRecord, 'cancel');

		await openBalancesForEmployee(page, initializeEmployee, currentYear);
		const initializeButton = page.getByRole('button', { name: /initialize balances/i });
		if (await initializeButton.isVisible().catch(() => false)) {
			const initializeResponsePromise = page.waitForResponse(
				initializeLeaveBalancesResponse(initializeEmployee.id, currentYear)
			);
			await initializeButton.click();
			const initializeResponse = await initializeResponsePromise;
			expect(initializeResponse.status()).toBe(200);
			const initialized = (await initializeResponse.json()) as LeaveBalanceResponse[];
			expect(initialized.length).toBeGreaterThan(0);
		}
		await expect(balanceRow(page, annualLeave!.name)).toBeVisible({ timeout: 10000 });

		await page.getByRole('button', { name: /import balances/i }).first().click();
		const importDialog = page.getByRole('dialog', { name: /import balances/i });
		await expect(importDialog).toBeVisible();

		const importCSV = [
			'year,employee_number,absence_type_code,entitled_days,carryover_days,used_days,pending_days,notes',
			`${currentYear},${importEmployee.employee_number},ANNUAL_LEAVE,26,1,2,0,E2E imported balance ${suffix}`
		].join('\n');
		const importFileName = `leave-balances-${suffix}.csv`;
		await importDialog.locator('#leave-balances-file').setInputFiles({
			name: importFileName,
			mimeType: 'text/csv',
			buffer: Buffer.from(importCSV)
		});
		await expect(importDialog).toContainText(importFileName);

		const importResponsePromise = page.waitForResponse(importLeaveBalancesResponse);
		await importDialog.getByRole('button', { name: /import balances/i }).last().click();
		const importResponse = await importResponsePromise;
		expect(importResponse.status()).toBe(200);
		const importResult = (await importResponse.json()) as ImportLeaveBalancesResult;
		expect(importResult.rows_processed).toBe(1);
		expect(importResult.rows_skipped).toBe(0);
		expect(importResult.leave_balances_created + importResult.leave_balances_updated).toBe(1);
		await expect(importDialog).toContainText(/import summary/i);
		await importDialog.getByRole('button', { name: /^cancel$/i }).click();
		await expect(importDialog).toBeHidden();

		const importedBalances = await openBalancesForEmployee(page, importEmployee, currentYear);
		const importedAnnual = importedBalances.find(
			(balance) => balance.absence_type_id === annualLeave!.id
		);
		expect(importedAnnual).toBeTruthy();
		expect(Number(importedAnnual?.entitled_days)).toBe(26);
		expect(Number(importedAnnual?.carryover_days)).toBe(1);
		expect(Number(importedAnnual?.used_days)).toBe(2);
		expect(Number(importedAnnual?.remaining_days)).toBe(25);

		const importedAnnualRow = balanceRow(page, annualLeave!.name);
		await expect(importedAnnualRow).toBeVisible({ timeout: 10000 });
		await expect(importedAnnualRow).toContainText('26.0');
		await expect(importedAnnualRow).toContainText('1.0');
		await expect(importedAnnualRow).toContainText('2.0');
		await expect(importedAnnualRow).toContainText('25.0');
	});
});
