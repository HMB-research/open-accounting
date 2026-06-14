import Decimal from 'decimal.js';
import {
	api,
	type BankAccount,
	type BankRemediationAction,
	type BankTransaction,
	type DocumentAttachment,
	type DocumentRemediationAction,
	type DocumentRetentionReview,
	type DocumentReviewSummary,
	type ExpenseClaim,
	type ExpenseRemediationAction,
	type JournalEntry,
	type KMDDeclaration,
	type KMDRemediationAction,
	type MigrationExecutionRun,
	type MigrationRemediationAction,
	type OverdueInvoicesSummary,
	type PayrollRun,
	type PayrollRunRemediationAction,
	type PeriodCloseEvent,
	type Tenant,
	type TSDDeclaration,
	type TSDRemediationAction,
	type YearEndCloseRemediationAction,
	type YearEndCloseStatus
} from '$lib/api';

export type BankExceptionGroup = {
	account: BankAccount;
	transactions: BankTransaction[];
	documentSummaries: Record<string, DocumentReviewSummary>;
};

export type BankExceptionItem = {
	account: BankAccount;
	transaction: BankTransaction;
	documentSummary: DocumentReviewSummary;
};

export type JournalEvidenceItem = {
	entry: JournalEntry;
	documentSummary: DocumentReviewSummary;
};

export type TenantReviewSnapshot = {
	tenant: Tenant;
	overdueSummary: OverdueInvoicesSummary | null;
	bankExceptions: BankExceptionGroup[];
	periodCloseEvents: PeriodCloseEvent[];
	journalEntries: JournalEntry[];
	journalEvidence: JournalEvidenceItem[];
	assignmentActions: WorkspaceAssignmentAction[];
	errorCount: number;
	assignmentErrorCount: number;
};

export type WorkspaceAssignmentSource =
	| 'close'
	| 'banking'
	| 'documents'
	| 'expenses'
	| 'payroll'
	| 'tsd'
	| 'kmd'
	| 'migration';

export type WorkspaceAssignmentAction = {
	id: string;
	code: string;
	source: WorkspaceAssignmentSource;
	queue: string;
	assignmentKey: string;
	priority: string;
	dueInDays: number;
	severity: string;
	message: string;
	action: string;
	uiPath?: string;
	cliCommand?: string;
	entityType?: string;
	entityId?: string;
	documentId?: string;
	documentType?: string;
	dueDate?: string;
	daysUntilRetention?: number;
	period?: string;
	periodEndDate?: string;
};

type RemediationActionLike = {
	code: string;
	severity: string;
	workspace_queue?: string;
	assignment_key?: string;
	priority?: string;
	due_in_days?: number;
	message: string;
	action: string;
	ui_path?: string;
	cli_command?: string;
	entity_type?: string;
	entity_id?: string;
	document_id?: string;
	document_type?: string;
	due_date?: string;
	days_until_retention?: number;
	period?: string;
	period_end_date?: string;
	fiscal_year_end_date?: string;
};

type AssignmentActionContext = {
	periodEndDate?: string;
};

export async function loadTenantReviewSnapshot(tenant: Tenant): Promise<TenantReviewSnapshot> {
	const lastFiscalYearEnd = getLastCompletedFiscalYearEnd(tenant.settings?.fiscal_year_start_month);
	const [
		overdueResult,
		accountsResult,
		periodCloseResult,
		journalResult,
		retentionResult,
		expensesResult,
		payrollResult,
		tsdResult,
		kmdResult,
		migrationRunsResult,
		yearEndCloseResult
	] = await Promise.allSettled([
		api.getOverdueInvoices(tenant.id),
		api.listBankAccounts(tenant.id, true),
		api.listPeriodCloseEvents(tenant.id, 6),
		api.listJournalEntries(tenant.id, 6),
		api.getDocumentRetentionReview(tenant.id, {
			horizon_days: 30,
			include_missing: true
		}),
		api.listExpenses(tenant.id, { limit: 100 }),
		api.listPayrollRuns(tenant.id),
		api.listTSD(tenant.id),
		api.listKMD(tenant.id),
		api.listMigrationExecutionRuns(tenant.id, { limit: 25 }),
		api.getYearEndCloseStatus(tenant.id, lastFiscalYearEnd)
	] as const);

	let bankExceptions: BankExceptionGroup[] = [];
	if (accountsResult.status === 'fulfilled') {
		bankExceptions = await loadUnmatchedTransactions(tenant.id, accountsResult.value);
	}

	const journalEntries = journalResult.status === 'fulfilled' ? journalResult.value : [];
	const journalEvidence = await loadJournalEvidence(tenant.id, journalEntries);
	const assignmentActions = buildWorkspaceAssignmentActions({
		closeStatus: yearEndCloseResult.status === 'fulfilled' ? yearEndCloseResult.value : null,
		bankExceptions,
		retentionReview: retentionResult.status === 'fulfilled' ? retentionResult.value : null,
		expenses: expensesResult.status === 'fulfilled' ? expensesResult.value : [],
		payrollRuns: payrollResult.status === 'fulfilled' ? payrollResult.value : [],
		tsdDeclarations: tsdResult.status === 'fulfilled' ? tsdResult.value : [],
		kmdDeclarations: kmdResult.status === 'fulfilled' ? kmdResult.value : [],
		migrationRuns: migrationRunsResult.status === 'fulfilled' ? migrationRunsResult.value : []
	});

	const errorCount = [overdueResult, accountsResult, periodCloseResult, journalResult].filter(
		(result) => result.status === 'rejected'
	).length;
	const assignmentErrorCount = [
		yearEndCloseResult,
		retentionResult,
		expensesResult,
		payrollResult,
		tsdResult,
		kmdResult,
		migrationRunsResult
	].filter((result) => result.status === 'rejected').length;

	return {
		tenant,
		overdueSummary: overdueResult.status === 'fulfilled' ? overdueResult.value : null,
		bankExceptions,
		periodCloseEvents: periodCloseResult.status === 'fulfilled' ? periodCloseResult.value : [],
		journalEntries,
		journalEvidence,
		assignmentActions,
		errorCount,
		assignmentErrorCount
	};
}

function buildWorkspaceAssignmentActions(input: {
	closeStatus: YearEndCloseStatus | null;
	bankExceptions: BankExceptionGroup[];
	retentionReview: DocumentRetentionReview | null;
	expenses: ExpenseClaim[];
	payrollRuns: PayrollRun[];
	tsdDeclarations: TSDDeclaration[];
	kmdDeclarations: KMDDeclaration[];
	migrationRuns: MigrationExecutionRun[];
}): WorkspaceAssignmentAction[] {
	const actions: WorkspaceAssignmentAction[] = [];

	actions.push(
		...normalizeRemediationActions('close', input.closeStatus?.remediation_actions ?? [], {
			periodEndDate: input.closeStatus?.period_end_date
		}),
		...normalizeRemediationActions(
			'banking',
			input.bankExceptions.flatMap((group) =>
				group.transactions.flatMap((transaction) => transaction.remediation_actions ?? [])
			)
		),
		...normalizeRemediationActions('documents', input.retentionReview?.remediation_actions ?? []),
		...normalizeRemediationActions(
			'expenses',
			input.expenses.flatMap((expense) => expense.remediation_actions ?? [])
		),
		...normalizeRemediationActions(
			'payroll',
			input.payrollRuns.flatMap((run) => run.remediation_actions ?? [])
		),
		...normalizeRemediationActions(
			'tsd',
			input.tsdDeclarations.flatMap((declaration) => declaration.remediation_actions ?? [])
		),
		...normalizeRemediationActions(
			'kmd',
			input.kmdDeclarations.flatMap((declaration) => declaration.remediation_actions ?? [])
		),
		...normalizeRemediationActions(
			'migration',
			buildMigrationRunRemediationActions(input.migrationRuns)
		)
	);

	return dedupeAssignments(actions).sort(compareAssignments);
}

function normalizeRemediationActions(
	source: WorkspaceAssignmentSource,
	actions:
		| BankRemediationAction[]
		| DocumentRemediationAction[]
		| ExpenseRemediationAction[]
		| KMDRemediationAction[]
		| MigrationRemediationAction[]
		| PayrollRunRemediationAction[]
		| TSDRemediationAction[]
		| YearEndCloseRemediationAction[],
	context: AssignmentActionContext = {}
): WorkspaceAssignmentAction[] {
	return actions
		.map((action) => normalizeRemediationAction(source, action, context))
		.filter((action): action is WorkspaceAssignmentAction => action !== null);
}

function normalizeRemediationAction(
	source: WorkspaceAssignmentSource,
	action: RemediationActionLike,
	context: AssignmentActionContext
): WorkspaceAssignmentAction | null {
	const queue = action.workspace_queue?.trim();
	const assignmentKey = action.assignment_key?.trim();
	if (!queue || !assignmentKey) {
		return null;
	}

	return {
		id: `${source}:${assignmentKey}`,
		code: action.code,
		source,
		queue,
		assignmentKey,
		priority: action.priority?.trim() || priorityForSeverity(action.severity),
		dueInDays: action.due_in_days ?? dueWindowForSeverity(action.severity),
		severity: action.severity,
		message: action.message,
		action: action.action,
		uiPath: action.ui_path || defaultAssignmentUIPath(source, action),
		cliCommand: action.cli_command,
		entityType: action.entity_type,
		entityId: action.entity_id,
		documentId: action.document_id,
		documentType: action.document_type,
		dueDate: action.due_date,
		daysUntilRetention: action.days_until_retention,
		period: action.period,
		periodEndDate:
			action.period_end_date?.trim() || action.fiscal_year_end_date?.trim() || context.periodEndDate
	};
}

function defaultAssignmentUIPath(
	source: WorkspaceAssignmentSource,
	action: RemediationActionLike
): string | undefined {
	if (source === 'migration' || action.workspace_queue === 'migration_cutover') {
		return '/migration';
	}
	return undefined;
}

function buildMigrationRunRemediationActions(
	runs: MigrationExecutionRun[]
): MigrationRemediationAction[] {
	return runs
		.filter((run) => run.summary.status !== 'succeeded')
		.flatMap((run) => {
			const actions = run.remediation_actions ?? [];
			const runStatusAction = buildMigrationRunStatusAction(run);
			return runStatusAction ? [runStatusAction, ...actions] : actions;
		});
}

function buildMigrationRunStatusAction(
	run: MigrationExecutionRun
): MigrationRemediationAction | null {
	const runId = run.id?.trim();
	if (!runId) {
		return null;
	}

	const status = run.summary.status;
	const activeStep = formatMigrationActiveStep(run);
	const uiPath = `/migration?run_id=${encodeURIComponent(runId)}`;

	if (status === 'failed') {
		return {
			code: 'migration_execution_failed',
			severity: 'ACTION',
			scope: 'migration',
			owner_role: 'accountant',
			workspace_queue: 'migration_cutover',
			assignment_key: `migration:execution-failed:${runId}`,
			priority: 'high',
			due_in_days: 0,
			message: `Migration run ${runId} failed${activeStep ? ` at ${activeStep}` : ''}.`,
			action:
				'Open the migration workbench, inspect the failed step, correct the bundle or execution context, and resume the run.',
			issue_count: run.summary.failed_step_count || 1,
			entity_type: 'migration_execution_run',
			entity_id: runId,
			ui_path: uiPath,
			cli_command: `oa migration runs get --run-id ${runId} --json`
		};
	}

	if (status === 'running') {
		return {
			code: 'migration_execution_running',
			severity: 'ACTION',
			scope: 'migration',
			owner_role: 'accountant',
			workspace_queue: 'migration_cutover',
			assignment_key: `migration:execution-running:${runId}`,
			priority: 'normal',
			due_in_days: 0,
			message: `Migration run ${runId} is running${activeStep ? ` at ${activeStep}` : ''}.`,
			action: 'Open the migration workbench to monitor progress and active-step telemetry.',
			issue_count: run.summary.running_step_count || 1,
			entity_type: 'migration_execution_run',
			entity_id: runId,
			ui_path: uiPath,
			cli_command: `oa migration runs get --run-id ${runId} --json`
		};
	}

	if (status === 'needs_confirmation') {
		return {
			code: 'migration_execution_needs_confirmation',
			severity: 'ACTION',
			scope: 'migration',
			owner_role: 'accountant',
			workspace_queue: 'migration_cutover',
			assignment_key: `migration:execution-needs-confirmation:${runId}`,
			priority: 'normal',
			due_in_days: 1,
			message: `Migration run ${runId} is ready for accountant confirmation.`,
			action:
				'Open the migration workbench, review the saved plan, and execute the confirmed cutover when ready.',
			issue_count: run.summary.planned_step_count || 1,
			entity_type: 'migration_execution_run',
			entity_id: runId,
			ui_path: uiPath,
			cli_command: `oa migration execute --resume-run-id ${runId} --confirm --json`
		};
	}

	if (status === 'blocked') {
		return {
			code: 'migration_execution_blocked',
			severity: 'BLOCKER',
			scope: 'migration',
			owner_role: 'accountant',
			workspace_queue: 'migration_cutover',
			assignment_key: `migration:execution-blocked:${runId}`,
			priority: 'high',
			due_in_days: 1,
			message: `Migration run ${runId} is blocked before execution.`,
			action:
				'Open the migration workbench, resolve preflight or missing-context blockers, and rebuild the execution plan.',
			issue_count: run.summary.blocked_step_count || run.summary.needs_context_count || 1,
			entity_type: 'migration_execution_run',
			entity_id: runId,
			ui_path: uiPath,
			cli_command: `oa migration runs get --run-id ${runId} --json`
		};
	}

	return null;
}

function formatMigrationActiveStep(run: MigrationExecutionRun): string {
	if (!run.summary.active_step_number) {
		return '';
	}
	const parts = [`step ${run.summary.active_step_number}`];
	if (run.summary.active_step_status) parts.push(run.summary.active_step_status);
	if (run.summary.active_step_kind) parts.push(run.summary.active_step_kind);
	if (run.summary.active_step_file_name) parts.push(run.summary.active_step_file_name);
	return parts.join(' ');
}

function dedupeAssignments(actions: WorkspaceAssignmentAction[]): WorkspaceAssignmentAction[] {
	const seen = new Set<string>();
	const deduped: WorkspaceAssignmentAction[] = [];
	for (const action of actions) {
		if (seen.has(action.id)) {
			continue;
		}
		seen.add(action.id);
		deduped.push(action);
	}
	return deduped;
}

function compareAssignments(
	left: WorkspaceAssignmentAction,
	right: WorkspaceAssignmentAction
): number {
	const priorityDiff = priorityRank(left.priority) - priorityRank(right.priority);
	if (priorityDiff !== 0) {
		return priorityDiff;
	}

	const dueDiff = left.dueInDays - right.dueInDays;
	if (dueDiff !== 0) {
		return dueDiff;
	}

	const sourceDiff = left.source.localeCompare(right.source);
	if (sourceDiff !== 0) {
		return sourceDiff;
	}

	return left.assignmentKey.localeCompare(right.assignmentKey);
}

function priorityRank(priority: string): number {
	switch (priority.toLowerCase()) {
		case 'high':
			return 0;
		case 'normal':
			return 1;
		case 'low':
			return 2;
		default:
			return 3;
	}
}

function priorityForSeverity(severity: string): string {
	switch (severity.toUpperCase()) {
		case 'BLOCKER':
		case 'ACTION':
		case 'ERROR':
			return 'high';
		case 'INFO':
			return 'low';
		default:
			return 'normal';
	}
}

function dueWindowForSeverity(severity: string): number {
	switch (severity.toUpperCase()) {
		case 'BLOCKER':
		case 'ACTION':
		case 'ERROR':
			return 1;
		case 'INFO':
			return 0;
		default:
			return 3;
	}
}

async function loadUnmatchedTransactions(
	tenantId: string,
	accounts: BankAccount[]
): Promise<BankExceptionGroup[]> {
	const groups = await Promise.all(
		accounts.map(async (account) => {
			try {
				const transactions = await api.listBankTransactions(tenantId, account.id, {
					status: 'UNMATCHED'
				});
				let documentSummaries: Record<string, DocumentReviewSummary> = {};
				if (transactions.length > 0) {
					documentSummaries = await loadDocumentSummaries(
						tenantId,
						'bank_transaction',
						transactions.map((transaction) => transaction.id)
					);
				}
				return { account, transactions, documentSummaries };
			} catch {
				return { account, transactions: [], documentSummaries: {} };
			}
		})
	);

	return groups
		.filter((group) => group.transactions.length > 0)
		.sort((left, right) => right.transactions.length - left.transactions.length);
}

async function loadJournalEvidence(
	tenantId: string,
	entries: JournalEntry[]
): Promise<JournalEvidenceItem[]> {
	const draftEvidenceEntries = entries.filter(
		(entry) => entry.requires_evidence && entry.status === 'DRAFT'
	);
	const documentSummaries = await loadDocumentSummaries(
		tenantId,
		'journal_entry',
		draftEvidenceEntries.map((entry) => entry.id)
	);

	return draftEvidenceEntries
		.map((entry) => ({
			entry,
			documentSummary:
				documentSummaries[entry.id] ?? missingDocumentSummary('journal_entry', entry.id)
		}))
		.filter((item) => needsEvidenceFollowUp(item.documentSummary))
		.sort(
			(left, right) =>
				new Date(right.entry.entry_date).getTime() - new Date(left.entry.entry_date).getTime()
		);
}

async function loadDocumentSummaries(
	tenantId: string,
	entityType: DocumentAttachment['entity_type'],
	entityIDs: string[]
): Promise<Record<string, DocumentReviewSummary>> {
	if (entityIDs.length === 0) {
		return {};
	}

	try {
		const summaries = await api.listDocumentReviewSummaries(tenantId, entityType, entityIDs);
		return Object.fromEntries(summaries.map((summary) => [summary.entity_id, summary]));
	} catch {
		return {};
	}
}

function needsEvidenceFollowUp(summary: DocumentReviewSummary): boolean {
	return (
		summary.missing_evidence ||
		summary.has_pending_review ||
		summary.has_rejected ||
		summary.approved_count === 0
	);
}

export function flattenUnmatchedTransactions(bankExceptions: BankExceptionGroup[]) {
	return bankExceptions
		.flatMap((group) =>
			group.transactions.map((transaction) => ({
				account: group.account,
				transaction,
				documentSummary:
					group.documentSummaries[transaction.id] ??
					missingDocumentSummary('bank_transaction', transaction.id)
			}))
		)
		.sort(
			(left, right) =>
				new Date(right.transaction.transaction_date).getTime() -
				new Date(left.transaction.transaction_date).getTime()
		);
}

function missingDocumentSummary(
	entityType: DocumentAttachment['entity_type'],
	entityID: string
): DocumentReviewSummary {
	return {
		entity_type: entityType,
		entity_id: entityID,
		total_count: 0,
		pending_review_count: 0,
		reviewed_count: 0,
		approved_count: 0,
		rejected_count: 0,
		missing_evidence: true,
		has_pending_review: false,
		has_rejected: false
	};
}

export function toDecimal(value: Decimal | number | string | null | undefined): Decimal {
	if (Decimal.isDecimal(value)) {
		return value;
	}
	if (value == null || value === '') {
		return new Decimal(0);
	}
	return new Decimal(value);
}

export function parseDateValue(value: string | null | undefined): Date | null {
	if (!value) {
		return null;
	}

	const [year, month, day] = value.split('-').map((part) => Number(part));
	if (!year || !month || !day) {
		return null;
	}

	return new Date(Date.UTC(year, month - 1, day));
}

export function formatIsoDate(value: Date): string {
	const year = value.getUTCFullYear();
	const month = String(value.getUTCMonth() + 1).padStart(2, '0');
	const day = String(value.getUTCDate()).padStart(2, '0');
	return `${year}-${month}-${day}`;
}

export function monthEndOffset(value: Date, monthOffset: number): Date {
	return new Date(Date.UTC(value.getUTCFullYear(), value.getUTCMonth() + monthOffset + 1, 0));
}

export function getSuggestedCloseDate(
	periodLockDate: string | null | undefined,
	today: Date = new Date()
): string {
	const currentLock = parseDateValue(periodLockDate);
	if (currentLock) {
		return formatIsoDate(monthEndOffset(currentLock, 1));
	}

	return formatIsoDate(monthEndOffset(today, -1));
}

export function needsPeriodClose(
	periodLockDate: string | null | undefined,
	today: Date = new Date()
): boolean {
	const currentLock = parseDateValue(periodLockDate);
	if (!currentLock) {
		return true;
	}

	const previousMonthEnd = new Date(Date.UTC(today.getUTCFullYear(), today.getUTCMonth(), 0));
	return currentLock.getTime() < previousMonthEnd.getTime();
}

export function getLastCompletedFiscalYearEnd(
	fiscalYearStartMonth: number | null | undefined,
	today: Date = new Date()
): string {
	const startMonth = Number.isInteger(fiscalYearStartMonth) ? Number(fiscalYearStartMonth) : 1;
	const normalizedStartMonth = Math.min(12, Math.max(1, startMonth));
	const endMonth = normalizedStartMonth === 1 ? 12 : normalizedStartMonth - 1;
	let candidate = new Date(Date.UTC(today.getUTCFullYear(), endMonth, 0));

	if (candidate.getTime() >= startOfUtcDay(today).getTime()) {
		candidate = new Date(Date.UTC(today.getUTCFullYear() - 1, endMonth, 0));
	}

	return formatIsoDate(candidate);
}

function startOfUtcDay(value: Date): Date {
	return new Date(Date.UTC(value.getUTCFullYear(), value.getUTCMonth(), value.getUTCDate()));
}
