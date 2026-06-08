import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant } from './utils';

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

async function openBankImport(page: Page, testInfo: TestInfo) {
	await navigateTo(page, '/banking/import', testInfo);

	await expect(page.getByRole('heading', { name: 'Import Bank Transactions' })).toBeVisible();
	await expect(page.getByRole('button', { name: /back/i })).toBeVisible();
	await expect(page.getByText('Select a CSV file to preview')).toBeVisible();
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

	test('displays bank import page with correct structure', async ({ page }, testInfo) => {
		await openBankImport(page, testInfo);

		await expect(page.getByRole('heading', { name: 'Import Settings' })).toBeVisible();
		await expect(page.getByLabel('Bank Account')).toBeVisible();
		await expect(page.getByLabel('CSV File')).toBeVisible();
		await expect(page.getByLabel('Bank Format Preset')).toBeVisible();
		await expect(page.getByLabel('Skip duplicate transactions')).toBeChecked();
		await expect(page.getByRole('button', { name: 'Import Transactions' })).toBeDisabled();
	});

	test('has bank account selector', async ({ page }, testInfo) => {
		await openBankImport(page, testInfo);

		const bankSelect = page.getByLabel('Bank Account');
		await expect(bankSelect).toContainText('Main EUR Account');
		await expect(bankSelect).toContainText('Savings Account');

		const options = await bankSelect.locator('option').count();
		expect(options).toBeGreaterThanOrEqual(2);
	});

	test('has bank format presets', async ({ page }, testInfo) => {
		await openBankImport(page, testInfo);

		const presetSelect = page.getByLabel('Bank Format Preset');
		await expect(presetSelect.locator('option')).toHaveText([
			'Auto',
			'Generic CSV',
			'LHV (Estonia)',
			'LHV CAMT.053'
		]);
	});

	test('previews selected LHV statement file', async ({ page }, testInfo) => {
		await openBankImport(page, testInfo);

		const { description } = await uploadLHVStatement(page, testInfo);

		await expect(page.getByRole('columnheader', { name: 'Col 0' })).toBeVisible();
		await expect(page.getByText('Showing first 2 rows')).toBeVisible();
		await expect(page.getByText(description)).toBeVisible();
	});

	test('imports an LHV statement and displays the transaction', async ({ page }, testInfo) => {
		await openBankImport(page, testInfo);

		const { description, displayAmount } = await uploadLHVStatement(page, testInfo);

		const alertPromise = page.waitForEvent('dialog');
		await page.getByRole('button', { name: 'Import Transactions' }).click({ noWaitAfter: true });
		const alert = await alertPromise;
		expect(alert.message()).toMatch(/Imported 1 transactions|Imporditud 1 tehingut/i);
		await alert.accept();

		await page.waitForURL(/\/banking/);
		await navigateTo(page, '/banking', testInfo);

		const importedRow = page.getByRole('row', { name: new RegExp(description) });
		await expect(importedRow).toBeVisible();
		await expect(importedRow).toContainText('E2E Import Client');
		await expect(importedRow).toContainText(displayAmount);
		await expect(importedRow).toContainText(/Unmatched|Sobitamata/i);
	});
});
