import { test, expect, type Page, type Response, type TestInfo } from '@playwright/test';
import {
	DEMO_API_URL,
	ensureAuthenticated,
	getDemoCredentials,
	navigateTo,
	ensureDemoTenant,
	waitForRouteReady
} from './utils';

const MAIN_DEMO_BANK_ACCOUNT = 'EE123456789012345678';

function uniqueImportId(testInfo: TestInfo): string {
	return `e2e-${testInfo.parallelIndex}-${testInfo.retry}-${Date.now()}`;
}

function statementDateFor(testInfo: TestInfo): string {
	return `2026-06-${String(8 + testInfo.retry).padStart(2, '0')}`;
}

function statementAmountFor(testInfo: TestInfo) {
	const cents = 75 + testInfo.retry;
	return {
		lhvAmount: `42,${String(cents).padStart(2, '0')}`,
		displayAmount: `42.${String(cents).padStart(2, '0')}`
	};
}

function buildLHVStatementCSV(externalId: string, description: string, statementDate: string, amount: string): string {
	return [
		"Client account;Document number;Date;Beneficiary's/remitter's account;Beneficiary's/remitter's name;Debit/Credit (D/C);Amount;Reference number;Archival ID;Details;Currency;Personal identification code or registry code;Beneficiary's/remitter's bank's BIC;Payment initiator's name;Entry reference;Account service provider's reference",
		`${MAIN_DEMO_BANK_ACCOUNT};DOC-${externalId};${statementDate};EE867700771000681884;E2E Import Client;C;${amount};REF-${externalId};ARCH-${externalId};${description};EUR;12345678;LHVBEE22;;ENTRY-${externalId};${externalId}`
	].join('\n');
}

function responsePath(response: Response): string {
	return new URL(response.url()).pathname;
}

function isTenantBankAccountsResponse(response: Response): boolean {
	return (
		response.request().method() === 'GET' &&
		response.status() === 200 &&
		/\/api\/v1\/tenants\/[^/]+\/bank-accounts$/.test(responsePath(response))
	);
}

function isBankTransactionsResponse(response: Response): boolean {
	return (
		response.request().method() === 'GET' &&
		response.status() === 200 &&
		/\/api\/v1\/tenants\/[^/]+\/bank-accounts\/[^/]+\/transactions$/.test(responsePath(response))
	);
}

function isCreatePaymentFromTransactionResponse(response: Response, transactionId: string): boolean {
	return (
		response.request().method() === 'POST' &&
		response.status() === 200 &&
		new RegExp(`/api/v1/tenants/[^/]+/bank-transactions/${transactionId}/create-payment$`).test(
			responsePath(response)
		)
	);
}

function isBankImportResponse(response: Response): boolean {
	return (
		response.request().method() === 'POST' &&
		response.status() === 200 &&
		/\/api\/v1\/tenants\/[^/]+\/bank-accounts\/[^/]+\/import$/.test(responsePath(response))
	);
}

interface BankTransactionResponse {
	id: string;
	description: string;
	status: 'UNMATCHED' | 'MATCHED' | 'RECONCILED';
	matched_payment_id?: string;
}

interface BankAccountResponse {
	id: string;
	account_number: string;
}

async function demoApiRequest<T>(page: Page, path: string): Promise<{ status: number; body: T }> {
	return page.evaluate(
		async ({ apiUrl, requestPath }) => {
			const token = localStorage.getItem('access_token') || sessionStorage.getItem('access_token');
			if (!token) throw new Error('Missing demo access token');

			const response = await fetch(`${apiUrl}${requestPath}`, {
				headers: {
					Authorization: `Bearer ${token}`
				}
			});
			const text = await response.text();
			return {
				status: response.status,
				body: text ? JSON.parse(text) : null
			};
		},
		{ apiUrl: DEMO_API_URL, requestPath: path }
	) as Promise<{ status: number; body: T }>;
}

async function fetchMainAccountTransactions(page: Page, testInfo: TestInfo): Promise<BankTransactionResponse[]> {
	const tenantId = getDemoCredentials(testInfo).tenantId;
	const accounts = await demoApiRequest<BankAccountResponse[]>(page, `/api/v1/tenants/${tenantId}/bank-accounts`);
	expect(accounts.status).toBe(200);

	const mainAccount = accounts.body.find((account) => account.account_number === MAIN_DEMO_BANK_ACCOUNT);
	expect(mainAccount, `Expected seeded bank account ${MAIN_DEMO_BANK_ACCOUNT}`).toBeTruthy();

	const transactions = await demoApiRequest<BankTransactionResponse[]>(
		page,
		`/api/v1/tenants/${tenantId}/bank-accounts/${mainAccount!.id}/transactions`
	);
	expect(transactions.status).toBe(200);
	return transactions.body;
}

async function openBankImport(page: Page, testInfo: TestInfo) {
	const bankAccountsResponse = page.waitForResponse(isTenantBankAccountsResponse);
	await navigateTo(page, '/banking/import', testInfo, { waitForNetworkIdle: false });
	await bankAccountsResponse;
	await waitForRouteReady(page, '.max-w-6xl h1, .max-w-6xl #import-csv-file, .max-w-6xl .bg-red-50');

	await expect(page.getByRole('heading', { name: 'Import Bank Transactions' })).toBeVisible();
	await expect(page.getByRole('button', { name: /back/i })).toBeVisible();
	await expect(page.getByText('Select a CSV file to preview')).toBeVisible();
}

async function openBankingLedger(page: Page, testInfo: TestInfo): Promise<BankTransactionResponse[]> {
	const bankAccountsResponse = page.waitForResponse(isTenantBankAccountsResponse);
	const transactionsResponse = page.waitForResponse(isBankTransactionsResponse);
	await navigateTo(page, '/banking', testInfo, { waitForNetworkIdle: false });
	await bankAccountsResponse;
	await transactionsResponse;
	await waitForRouteReady(page, '.max-w-7xl h1, .max-w-7xl table, .max-w-7xl .empty-state, .max-w-7xl .bg-red-50');
	return fetchMainAccountTransactions(page, testInfo);
}

async function uploadLHVStatement(page: Page, testInfo: TestInfo): Promise<{ description: string; displayAmount: string }> {
	const externalId = uniqueImportId(testInfo);
	const description = `E2E LHV import ${externalId}`;
	const { lhvAmount, displayAmount } = statementAmountFor(testInfo);
	const csvContent = buildLHVStatementCSV(externalId, description, statementDateFor(testInfo), lhvAmount);

	await page.getByLabel('Bank Format Preset').selectOption('lhv');
	await page.getByLabel('CSV File').setInputFiles({
		name: `lhv-${externalId}.csv`,
		mimeType: 'text/csv',
		buffer: Buffer.from(csvContent)
	});

	const preview = page.getByRole('table');
	await expect(preview).toContainText(MAIN_DEMO_BANK_ACCOUNT);
	await expect(preview).toContainText(description);
	await expect(page.getByRole('button', { name: 'Import Transactions' })).toBeEnabled();

	return { description, displayAmount };
}

test.describe('Bank Import View', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('previews and imports an LHV statement into the banking ledger', async ({ page }, testInfo) => {
		await openBankImport(page, testInfo);

		await expect(page.getByRole('heading', { name: 'Import Settings' })).toBeVisible();
		await expect(page.getByLabel('Bank Account')).toBeVisible();
		await expect(page.getByLabel('CSV File')).toBeVisible();
		await expect(page.getByLabel('Bank Format Preset')).toBeVisible();
		await expect(page.getByLabel('Skip duplicate transactions')).toBeChecked();
		await expect(page.getByRole('button', { name: 'Import Transactions' })).toBeDisabled();

		const bankSelect = page.getByLabel('Bank Account');
		await expect(bankSelect).toContainText('Main EUR Account');
		await expect(bankSelect).toContainText('Savings Account');

		const options = await bankSelect.locator('option').count();
		expect(options).toBeGreaterThanOrEqual(2);

		const presetSelect = page.getByLabel('Bank Format Preset');
		await expect(presetSelect.locator('option')).toHaveText([
			'Auto',
			'Generic CSV',
			'LHV (Estonia)',
			'CAMT.053 (ISO 20022)',
			'LHV CAMT.053'
		]);

		const { description, displayAmount } = await uploadLHVStatement(page, testInfo);

		await expect(page.getByRole('columnheader', { name: 'Col 0' })).toBeVisible();
		await expect(page.getByText('Showing first 2 rows')).toBeVisible();
		await expect(page.getByText(description)).toBeVisible();

		const importResponsePromise = page.waitForResponse(isBankImportResponse);
		const alertPromise = page.waitForEvent('dialog');
		await page.getByRole('button', { name: 'Import Transactions' }).click({ noWaitAfter: true });
		const [importResponse, alert] = await Promise.all([importResponsePromise, alertPromise]);
		expect(importResponse.status()).toBe(200);
		expect(alert.message()).toMatch(/Imported 1 transactions|Imporditud 1 tehingut/i);
		await alert.accept();
		const importResult = await importResponse.json();
		expect(importResult.transactions_imported).toBe(1);
		expect(importResult.duplicates_skipped).toBe(0);

		const transactions = await openBankingLedger(page, testInfo);
		const importedTransaction = transactions.find((transaction) => transaction.description === description);
		expect(importedTransaction?.status).toBe('UNMATCHED');

		const importedRow = page.getByRole('row', { name: new RegExp(description) });
		await expect(importedRow).toBeVisible({ timeout: 15000 });
		await expect(importedRow).toContainText('E2E Import Client');
		await expect(importedRow).toContainText(displayAmount);
		await expect(importedRow).toContainText(/Unmatched|Sobitamata/i);

		const createPaymentResponsePromise = page.waitForResponse((response) =>
			isCreatePaymentFromTransactionResponse(response, importedTransaction!.id)
		);
		const refreshedTransactionsPromise = page.waitForResponse(isBankTransactionsResponse);
		await importedRow.getByRole('button', { name: /create payment|loo makse/i }).click();

		const createPaymentResponse = await createPaymentResponsePromise;
		expect(createPaymentResponse.ok()).toBeTruthy();
		const createPaymentResult = (await createPaymentResponse.json()) as { payment_id: string };
		expect(createPaymentResult.payment_id).toMatch(/[0-9a-f-]{36}/i);

		await refreshedTransactionsPromise;
		const refreshedTransactions = await fetchMainAccountTransactions(page, testInfo);
		const matchedTransaction = refreshedTransactions.find((transaction) => transaction.description === description);
		expect(matchedTransaction?.status).toBe('MATCHED');
		expect(matchedTransaction?.matched_payment_id).toBe(createPaymentResult.payment_id);

		await expect(importedRow).toContainText(/Matched|Sobitatud/i);
		await expect(importedRow.getByRole('button', { name: /create payment|loo makse/i })).toHaveCount(0);
		await expect(importedRow.getByRole('button', { name: /unmatch|tühista sobitus/i })).toBeVisible();
	});
});
