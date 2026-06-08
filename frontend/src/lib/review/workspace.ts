import Decimal from 'decimal.js';
import {
	api,
	type BankAccount,
	type BankTransaction,
	type DocumentAttachment,
	type DocumentReviewSummary,
	type JournalEntry,
	type OverdueInvoicesSummary,
	type PeriodCloseEvent,
	type Tenant
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
	errorCount: number;
};

export async function loadTenantReviewSnapshot(tenant: Tenant): Promise<TenantReviewSnapshot> {
	const [overdueResult, accountsResult, closeResult, journalResult] = await Promise.allSettled([
		api.getOverdueInvoices(tenant.id),
		api.listBankAccounts(tenant.id, true),
		api.listPeriodCloseEvents(tenant.id, 6),
		api.listJournalEntries(tenant.id, 6)
	]);

	let bankExceptions: BankExceptionGroup[] = [];
	if (accountsResult.status === 'fulfilled') {
		bankExceptions = await loadUnmatchedTransactions(tenant.id, accountsResult.value);
	}

	const journalEntries = journalResult.status === 'fulfilled' ? journalResult.value : [];
	const journalEvidence = await loadJournalEvidence(tenant.id, journalEntries);

	const errorCount = [overdueResult, accountsResult, closeResult, journalResult].filter(
		(result) => result.status === 'rejected'
	).length;

	return {
		tenant,
		overdueSummary: overdueResult.status === 'fulfilled' ? overdueResult.value : null,
		bankExceptions,
		periodCloseEvents: closeResult.status === 'fulfilled' ? closeResult.value : [],
		journalEntries,
		journalEvidence,
		errorCount
	};
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
