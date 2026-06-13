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
	| 'kmd';

export type WorkspaceAssignmentAction = {
	id: string;
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
	period?: string;
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
	period?: string;
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
		yearEndCloseResult
	] = await Promise.allSettled([
		api.getOverdueInvoices(tenant.id),
		api.listBankAccounts(tenant.id, true),
		api.listPeriodCloseEvents(tenant.id, 6),
		api.listJournalEntries(tenant.id, 6),
		api.getDocumentRetentionReview(tenant.id, { horizon_days: 30, include_missing: true }),
		api.listExpenses(tenant.id, { limit: 100 }),
		api.listPayrollRuns(tenant.id),
		api.listTSD(tenant.id),
		api.listKMD(tenant.id),
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
		kmdDeclarations: kmdResult.status === 'fulfilled' ? kmdResult.value : []
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
		kmdResult
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
}): WorkspaceAssignmentAction[] {
	const actions: WorkspaceAssignmentAction[] = [];

	actions.push(
		...normalizeRemediationActions('close', input.closeStatus?.remediation_actions ?? []),
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
		| PayrollRunRemediationAction[]
		| TSDRemediationAction[]
		| YearEndCloseRemediationAction[]
): WorkspaceAssignmentAction[] {
	return actions
		.map((action) => normalizeRemediationAction(source, action))
		.filter((action): action is WorkspaceAssignmentAction => action !== null);
}

function normalizeRemediationAction(
	source: WorkspaceAssignmentSource,
	action: RemediationActionLike
): WorkspaceAssignmentAction | null {
	const queue = action.workspace_queue?.trim();
	const assignmentKey = action.assignment_key?.trim();
	if (!queue || !assignmentKey) {
		return null;
	}

	return {
		id: `${source}:${assignmentKey}`,
		source,
		queue,
		assignmentKey,
		priority: action.priority?.trim() || priorityForSeverity(action.severity),
		dueInDays: action.due_in_days ?? dueWindowForSeverity(action.severity),
		severity: action.severity,
		message: action.message,
		action: action.action,
		uiPath: action.ui_path,
		cliCommand: action.cli_command,
		entityType: action.entity_type,
		entityId: action.entity_id,
		period: action.period
	};
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

function compareAssignments(left: WorkspaceAssignmentAction, right: WorkspaceAssignmentAction): number {
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

async function loadUnmatchedTransactions(tenantId: string, accounts: BankAccount[]): Promise<BankExceptionGroup[]> {
	const groups = await Promise.all(
		accounts.map(async (account) => {
			try {
				const transactions = await api.listBankTransactions(tenantId, account.id, { status: 'UNMATCHED' });
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

async function loadJournalEvidence(tenantId: string, entries: JournalEntry[]): Promise<JournalEvidenceItem[]> {
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
			documentSummary: documentSummaries[entry.id] ?? missingDocumentSummary('journal_entry', entry.id)
		}))
		.filter((item) => needsEvidenceFollowUp(item.documentSummary))
		.sort((left, right) => new Date(right.entry.entry_date).getTime() - new Date(left.entry.entry_date).getTime());
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
					group.documentSummaries[transaction.id] ?? missingDocumentSummary('bank_transaction', transaction.id)
			}))
		)
		.sort((left, right) => new Date(right.transaction.transaction_date).getTime() - new Date(left.transaction.transaction_date).getTime());
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

export function getSuggestedCloseDate(periodLockDate: string | null | undefined, today: Date = new Date()): string {
	const currentLock = parseDateValue(periodLockDate);
	if (currentLock) {
		return formatIsoDate(monthEndOffset(currentLock, 1));
	}

	return formatIsoDate(monthEndOffset(today, -1));
}

export function needsPeriodClose(periodLockDate: string | null | undefined, today: Date = new Date()): boolean {
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
