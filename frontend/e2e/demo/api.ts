import { DEMO_API_URL } from './utils';

const DEMO_SECRET = process.env.DEMO_RESET_SECRET || '';

export interface EntityStatus {
	count: number;
	keys: string[];
}

export interface DemoStatus {
	user: number;
	accounts: EntityStatus;
	contacts: EntityStatus;
	invoices: EntityStatus;
	employees: EntityStatus;
	payments: EntityStatus;
	journalEntries: EntityStatus;
	bankAccounts: EntityStatus;
	recurringInvoices: EntityStatus;
	payrollRuns: EntityStatus;
	tsdDeclarations: EntityStatus;
}

/**
 * Expected demo data counts and key identifiers.
 * Update these when seed data changes.
 */
export const EXPECTED_DEMO_DATA = {
	accounts: {
		count: 33,
		keys: ['Cash', 'Bank Account - EUR', 'Accounts Receivable', 'Accounts Payable']
	},
	contacts: {
		count: 7,
		keys: ['TechStart OÜ', 'Nordic Solutions AS', 'Baltic Commerce']
	},
	invoices: {
		count: 9,
		// Keys are per-user: INV1-2024-001 for user 1, INV2-2024-001 for user 2, etc.
		keys: []
	},
	employees: {
		count: 5,
		keys: ['Maria Tamm', 'Jaan Kask', 'Anna Mets']
	},
	payments: {
		count: 4,
		// Keys are per-user: PAY1-2024-001 for user 1, etc.
		keys: []
	},
	journalEntries: {
		count: 4,
		// Keys are per-user: JE1-2024-001 for user 1, etc.
		keys: []
	},
	bankAccounts: {
		count: 2,
		keys: ['Main EUR Account', 'Savings Account']
	},
	recurringInvoices: {
		count: 3,
		keys: ['Monthly Support - TechStart', 'Quarterly Retainer - Nordic']
	},
	payrollRuns: {
		count: 3,
		keys: ['2024-10', '2024-11', '2024-12']
	},
	tsdDeclarations: {
		count: 3,
		keys: ['2024-10', '2024-11', '2024-12']
	}
};

/**
 * Get expected invoice key pattern for a user
 */
export function getExpectedInvoiceKey(userNum: number): string {
	return `INV${userNum}-2024-001`;
}

/**
 * Get expected payment key pattern for a user
 */
export function getExpectedPaymentKey(userNum: number): string {
	return `PAY${userNum}-2024-001`;
}

/**
 * Get expected journal entry key pattern for a user
 */
export function getExpectedJournalEntryKey(userNum: number): string {
	return `JE${userNum}-2024-001`;
}

/**
 * Trigger demo reset for a specific user
 */
export async function triggerDemoReset(userNum: number): Promise<void> {
	const response = await fetch(`${DEMO_API_URL}/api/demo/reset?user=${userNum}`, {
		method: 'POST',
		headers: {
			'X-Demo-Secret': DEMO_SECRET
		}
	});

	if (!response.ok) {
		throw new Error(`Demo reset failed: ${response.status} ${await response.text()}`);
	}
}

/**
 * Get demo status (counts and key identifiers) for a specific user
 */
export async function getDemoStatus(userNum: number): Promise<DemoStatus> {
	const response = await fetch(`${DEMO_API_URL}/api/demo/status?user=${userNum}`, {
		headers: {
			'X-Demo-Secret': DEMO_SECRET
		}
	});

	if (!response.ok) {
		throw new Error(`Demo status failed: ${response.status} ${await response.text()}`);
	}

	return response.json();
}
