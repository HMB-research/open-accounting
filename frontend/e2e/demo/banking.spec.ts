import { test, expect, type Page, type Response, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant, waitForRouteReady } from './utils';

interface BankTransactionResponse {
	id: string;
	description: string;
	amount: string | number;
	status: 'UNMATCHED' | 'MATCHED' | 'RECONCILED';
}

interface BankAccountResponse {
	id: string;
	name: string;
	account_number: string;
	bank_name?: string;
	currency: string;
	is_active: boolean;
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

function isCreateBankAccountResponse(response: Response): boolean {
	return (
		response.request().method() === 'POST' &&
		response.status() === 201 &&
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

async function openBanking(page: Page, testInfo: TestInfo): Promise<BankTransactionResponse[]> {
	const bankAccountsResponse = page.waitForResponse(isTenantBankAccountsResponse);
	const transactionsResponse = page.waitForResponse(isBankTransactionsResponse);
	await navigateTo(page, '/banking', testInfo, { waitForNetworkIdle: false });
	await bankAccountsResponse;
	const response = await transactionsResponse;
	const transactions = response.json() as Promise<BankTransactionResponse[]>;
	await waitForRouteReady(page, '#bank-account-selector, table, .empty-state', 15000);
	return transactions;
}

function uniqueAccountSuffix(testInfo: TestInfo): string {
	return `${testInfo.parallelIndex}-${testInfo.retry}-${Date.now()}`;
}

test.describe('Demo Banking - Seed Data Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('displays bank accounts, transactions, amounts, and statuses', async ({ page }, testInfo) => {
		const transactions = await openBanking(page, testInfo);
		const accountSelector = page.locator('#bank-account-selector');
		await expect(accountSelector).toBeVisible({ timeout: 10000 });

		const accountNames = await accountSelector.locator('option').allTextContents();
		expect(accountNames.join(' ')).toMatch(/Main EUR|Savings|Swedbank|SEB/);

		const rows = page.locator('table tbody tr');
		await expect(rows.first()).toBeVisible({ timeout: 10000 });
		const count = await rows.count();
		expect(count).toBeGreaterThanOrEqual(1);
		expect(transactions.length).toBeGreaterThanOrEqual(1);

		const pageContent = await page.content();
		expect(pageContent).toMatch(/[\d,]+\.\d{2}/);

		expect(transactions.map((transaction) => transaction.status)).toEqual(
			expect.arrayContaining(['UNMATCHED', 'MATCHED', 'RECONCILED'])
		);
		await expect(rows.first()).toContainText(/matched|unmatched|reconciled|sobitatud|sobitamata/i);
	});

	test('creates a bank account from the banking page', async ({ page }, testInfo) => {
		await openBanking(page, testInfo);

		const suffix = uniqueAccountSuffix(testInfo);
		const accountName = `E2E Operating ${suffix}`;
		const accountNumber = `EE${suffix.replaceAll('-', '').slice(-12).padStart(18, '0')}`;
		const bankName = `E2E Bank ${suffix}`;

		await page.getByRole('button', { name: /add bank account|lisa pangakonto/i }).click();
		const modal = page.locator('.fixed.inset-0').filter({ hasText: /add bank account|lisa pangakonto/i }).first();
		await expect(modal).toBeVisible();

		await modal.getByLabel(/account name|konto nimi/i).fill(accountName);
		await modal.getByLabel(/account number|konto number/i).fill(accountNumber);
		await modal.getByLabel(/bank name|panga nimi/i).fill(bankName);
		await modal.getByLabel(/currency|valuuta/i).selectOption('EUR');

		const createResponsePromise = page.waitForResponse(isCreateBankAccountResponse);
		await modal.getByRole('button', { name: /^create$|loo/i }).click();
		const createResponse = await createResponsePromise;
		expect(createResponse.ok()).toBeTruthy();
		expect(createResponse.request().postDataJSON()).toMatchObject({
			name: accountName,
			account_number: accountNumber,
			bank_name: bankName,
			currency: 'EUR'
		});
		expect(createResponse.request().postDataJSON()).not.toHaveProperty('opening_balance');

		const createdAccount = (await createResponse.json()) as BankAccountResponse;
		expect(createdAccount.name).toBe(accountName);
		expect(createdAccount.account_number).toBe(accountNumber);
		expect(createdAccount.bank_name).toBe(bankName);
		expect(createdAccount.currency).toBe('EUR');
		expect(createdAccount.is_active).toBe(true);

		await expect(modal).toBeHidden({ timeout: 10000 });
		const selector = page.locator('#bank-account-selector');
		await expect(selector).toContainText(accountName);
		await expect(selector).toContainText(accountNumber);
	});
});
