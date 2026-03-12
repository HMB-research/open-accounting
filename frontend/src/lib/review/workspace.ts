import Decimal from 'decimal.js';
import {
	api,
	type BankAccount,
	type BankTransaction,
	type JournalEntry,
	type OverdueInvoicesSummary,
	type PeriodCloseEvent,
	type Tenant
} from '$lib/api';

export type BankExceptionGroup = {
	account: BankAccount;
	transactions: BankTransaction[];
};

export type TenantReviewSnapshot = {
	tenant: Tenant;
	overdueSummary: OverdueInvoicesSummary | null;
	bankExceptions: BankExceptionGroup[];
	periodCloseEvents: PeriodCloseEvent[];
	journalEntries: JournalEntry[];
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

	const errorCount = [overdueResult, accountsResult, closeResult, journalResult].filter(
		(result) => result.status === 'rejected'
	).length;

	return {
		tenant,
		overdueSummary: overdueResult.status === 'fulfilled' ? overdueResult.value : null,
		bankExceptions,
		periodCloseEvents: closeResult.status === 'fulfilled' ? closeResult.value : [],
		journalEntries: journalResult.status === 'fulfilled' ? journalResult.value : [],
		errorCount
	};
}

async function loadUnmatchedTransactions(tenantId: string, accounts: BankAccount[]): Promise<BankExceptionGroup[]> {
	const groups = await Promise.all(
		accounts.map(async (account) => {
			try {
				const transactions = await api.listBankTransactions(tenantId, account.id, { status: 'UNMATCHED' });
				return { account, transactions };
			} catch {
				return { account, transactions: [] };
			}
		})
	);

	return groups
		.filter((group) => group.transactions.length > 0)
		.sort((left, right) => right.transactions.length - left.transactions.length);
}

export function flattenUnmatchedTransactions(bankExceptions: BankExceptionGroup[]) {
	return bankExceptions
		.flatMap((group) => group.transactions.map((transaction) => ({ account: group.account, transaction })))
		.sort((left, right) => new Date(right.transaction.transaction_date).getTime() - new Date(left.transaction.transaction_date).getTime());
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
