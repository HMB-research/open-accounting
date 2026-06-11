import { test, expect } from '@playwright/test';
import { DEMO_API_URL } from './utils';

test.describe('Demo API Data Verification', () => {
	const DEMO_SECRET = process.env.DEMO_RESET_SECRET;

	test.skip(!DEMO_SECRET, 'DEMO_RESET_SECRET required for API verification');

	test('API reports correct data counts for all entities', async ({}, testInfo) => {
		const userNum = (testInfo.parallelIndex % 3) + 2;
		const response = await fetch(`${DEMO_API_URL}/api/demo/status?user=${userNum}`, {
			headers: { 'X-Demo-Secret': DEMO_SECRET! }
		});

		expect(response.ok, `API status check should succeed for user ${userNum}`).toBeTruthy();

		const status = await response.json();

		expect(status.accounts.count, 'Must have accounts').toBeGreaterThan(0);
		expect(status.contacts.count, 'Must have contacts').toBeGreaterThan(0);
		expect(status.invoices.count, 'Must have invoices').toBeGreaterThan(0);
		expect(status.employees.count, 'Must have employees').toBeGreaterThan(0);
		expect(status.payments.count, 'Must have payments').toBeGreaterThan(0);
		expect(status.journalEntries.count, 'Must have journal entries').toBeGreaterThan(0);
		expect(status.bankAccounts.count, 'Must have bank accounts').toBeGreaterThan(0);
		expect(status.recurringInvoices.count, 'Must have recurring invoices').toBeGreaterThan(0);
		expect(status.payrollRuns.count, 'Must have payroll runs').toBeGreaterThan(0);
		expect(status.tsdDeclarations.count, 'Must have TSD declarations').toBeGreaterThan(0);

		console.log(`User ${userNum} data counts:`, {
			accounts: status.accounts.count,
			contacts: status.contacts.count,
			invoices: status.invoices.count,
			employees: status.employees.count,
			payments: status.payments.count,
			journalEntries: status.journalEntries.count,
			bankAccounts: status.bankAccounts.count,
			recurringInvoices: status.recurringInvoices.count,
			payrollRuns: status.payrollRuns.count,
			tsdDeclarations: status.tsdDeclarations.count
		});
	});
});
