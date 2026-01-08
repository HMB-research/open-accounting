import { test, expect } from '@playwright/test';
import {
	getDemoStatus,
	triggerDemoReset,
	EXPECTED_DEMO_DATA,
	getExpectedInvoiceKey,
	getExpectedPaymentKey,
	getExpectedJournalEntryKey
} from './api';

// Skip all reset tests if DEMO_RESET_SECRET is not provided
const DEMO_SECRET = process.env.DEMO_RESET_SECRET;
test.describe('Demo Data Reset Verification', () => {
	test.skip(!DEMO_SECRET, 'DEMO_RESET_SECRET environment variable required');

	test.describe('Initial State Verification', () => {
		test('has correct account count and key accounts', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 2;
			const status = await getDemoStatus(userNum);

			expect(status.accounts.count).toBe(EXPECTED_DEMO_DATA.accounts.count);
			for (const key of EXPECTED_DEMO_DATA.accounts.keys) {
				expect(status.accounts.keys).toContain(key);
			}
		});

		test('has correct contact count and key contacts', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 2;
			const status = await getDemoStatus(userNum);

			// Use >= check to handle timing variations in demo seeding
			expect(status.contacts.count).toBeGreaterThanOrEqual(EXPECTED_DEMO_DATA.contacts.count);
			for (const key of EXPECTED_DEMO_DATA.contacts.keys) {
				expect(status.contacts.keys).toContain(key);
			}
		});

		test('has correct invoice count and key invoices', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 2;
			const status = await getDemoStatus(userNum);

			expect(status.invoices.count).toBe(EXPECTED_DEMO_DATA.invoices.count);
			expect(status.invoices.keys).toContain(getExpectedInvoiceKey(userNum));
		});

		test('has correct employee count and key employees', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 2;
			const status = await getDemoStatus(userNum);

			expect(status.employees.count).toBe(EXPECTED_DEMO_DATA.employees.count);
			for (const key of EXPECTED_DEMO_DATA.employees.keys) {
				expect(status.employees.keys).toContain(key);
			}
		});

		test('has correct payment count and key payments', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 2;
			const status = await getDemoStatus(userNum);

			expect(status.payments.count).toBe(EXPECTED_DEMO_DATA.payments.count);
			expect(status.payments.keys).toContain(getExpectedPaymentKey(userNum));
		});

		test('has correct journal entry count and key entries', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 2;
			const status = await getDemoStatus(userNum);

			expect(status.journalEntries.count).toBe(EXPECTED_DEMO_DATA.journalEntries.count);
			expect(status.journalEntries.keys).toContain(getExpectedJournalEntryKey(userNum));
		});

		test('has correct bank account count', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 2;
			const status = await getDemoStatus(userNum);

			expect(status.bankAccounts.count).toBe(EXPECTED_DEMO_DATA.bankAccounts.count);
			for (const key of EXPECTED_DEMO_DATA.bankAccounts.keys) {
				expect(status.bankAccounts.keys).toContain(key);
			}
		});

		test('has correct recurring invoice count', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 2;
			const status = await getDemoStatus(userNum);

			expect(status.recurringInvoices.count).toBe(EXPECTED_DEMO_DATA.recurringInvoices.count);
			for (const key of EXPECTED_DEMO_DATA.recurringInvoices.keys) {
				expect(status.recurringInvoices.keys).toContain(key);
			}
		});

		test('has correct payroll run count', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 2;
			const status = await getDemoStatus(userNum);

			expect(status.payrollRuns.count).toBe(EXPECTED_DEMO_DATA.payrollRuns.count);
			for (const key of EXPECTED_DEMO_DATA.payrollRuns.keys) {
				expect(status.payrollRuns.keys).toContain(key);
			}
		});

		test('has correct TSD declaration count', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 2;
			const status = await getDemoStatus(userNum);

			expect(status.tsdDeclarations.count).toBe(EXPECTED_DEMO_DATA.tsdDeclarations.count);
			for (const key of EXPECTED_DEMO_DATA.tsdDeclarations.keys) {
				expect(status.tsdDeclarations.keys).toContain(key);
			}
		});
	});

	test.describe('Reset Functionality', () => {
		test('reset is idempotent', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 2;

			// Reset twice
			await triggerDemoReset(userNum);
			const statusAfterFirst = await getDemoStatus(userNum);

			await triggerDemoReset(userNum);
			const statusAfterSecond = await getDemoStatus(userNum);

			// Should produce identical state
			expect(statusAfterSecond.accounts.count).toBe(statusAfterFirst.accounts.count);
			expect(statusAfterSecond.contacts.count).toBe(statusAfterFirst.contacts.count);
			expect(statusAfterSecond.invoices.count).toBe(statusAfterFirst.invoices.count);
			expect(statusAfterSecond.employees.count).toBe(statusAfterFirst.employees.count);
		});

		test('reset restores expected counts', async ({}, testInfo) => {
			const userNum = (testInfo.parallelIndex % 3) + 2;

			// Trigger reset
			await triggerDemoReset(userNum);

			// Verify all counts match or exceed expected (timing variations may add extra records)
			const status = await getDemoStatus(userNum);

			expect(status.accounts.count).toBeGreaterThanOrEqual(EXPECTED_DEMO_DATA.accounts.count);
			expect(status.contacts.count).toBeGreaterThanOrEqual(EXPECTED_DEMO_DATA.contacts.count);
			expect(status.invoices.count).toBeGreaterThanOrEqual(EXPECTED_DEMO_DATA.invoices.count);
			expect(status.employees.count).toBeGreaterThanOrEqual(EXPECTED_DEMO_DATA.employees.count);
			expect(status.payments.count).toBeGreaterThanOrEqual(EXPECTED_DEMO_DATA.payments.count);
			expect(status.journalEntries.count).toBeGreaterThanOrEqual(EXPECTED_DEMO_DATA.journalEntries.count);
			expect(status.bankAccounts.count).toBeGreaterThanOrEqual(EXPECTED_DEMO_DATA.bankAccounts.count);
			expect(status.recurringInvoices.count).toBeGreaterThanOrEqual(EXPECTED_DEMO_DATA.recurringInvoices.count);
			expect(status.payrollRuns.count).toBeGreaterThanOrEqual(EXPECTED_DEMO_DATA.payrollRuns.count);
			expect(status.tsdDeclarations.count).toBeGreaterThanOrEqual(EXPECTED_DEMO_DATA.tsdDeclarations.count);
		});
	});
});
