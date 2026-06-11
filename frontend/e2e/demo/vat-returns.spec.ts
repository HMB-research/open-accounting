import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, navigateTo } from './utils';

interface KMDDeclarationResponse {
	id: string;
	year: number;
	month: number;
	status: string;
	total_output_vat: string | number;
	total_input_vat: string | number;
	rows: unknown[];
}

function responsePath(responseUrl: string) {
	return new URL(responseUrl).pathname;
}

function targetPeriod(testInfo: TestInfo) {
	return {
		year: 2024,
		month: (testInfo.repeatEachIndex % 12) + 1
	};
}

function formatAmount(amount: string | number) {
	return Number(amount).toFixed(2);
}

async function waitForKMDList(page: Page) {
	const response = await page.waitForResponse((candidate) => {
		return (
			candidate.request().method() === 'GET' &&
			/\/api\/v1\/tenants\/[^/]+\/tax\/kmd$/.test(responsePath(candidate.url()))
		);
	});
	expect(response.ok()).toBeTruthy();
}

async function waitForVATReturnsLoaded(page: Page) {
	await expect(page.locator('.generate-section')).toBeVisible({ timeout: 10000 });
	await expect(page.locator('.declarations-list')).toBeVisible({ timeout: 10000 });
	await expect(page.getByText(/^Loading\.\.\.$|^Laadimine\.\.\.$/i)).toHaveCount(0, {
		timeout: 10000
	});
}

async function openVATReturns(page: Page, testInfo: TestInfo) {
	const listPromise = waitForKMDList(page);
	await navigateTo(page, '/vat-returns', testInfo, { waitForNetworkIdle: false });
	await listPromise;
	await waitForVATReturnsLoaded(page);
}

test.describe('Demo VAT Returns (KMD)', () => {
	test('covers page controls, declaration generation, and detail rendering', async ({
		page
	}, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await openVATReturns(page, testInfo);

		await expect(page.getByRole('heading', { name: /vat|käibe/i })).toBeVisible();
		await expect(page.getByRole('heading', { name: /generate|genereeri/i })).toBeVisible();
		await expect(
			page.locator('.declarations-list').getByRole('heading', {
				name: /declarations|deklaratsioon/i
			})
		).toBeVisible();

		const yearSelect = page.locator('select#year');
		await expect(yearSelect).toBeVisible();
		const yearOptions = await yearSelect.locator('option').allTextContents();
		expect(yearOptions).toEqual(expect.arrayContaining(['2024', '2025', '2026']));

		const monthSelect = page.locator('select#month');
		await expect(monthSelect).toBeVisible();
		await expect(monthSelect.locator('option')).toHaveCount(12);

		const period = targetPeriod(testInfo);
		const periodLabel = `${period.year}-${String(period.month).padStart(2, '0')}`;
		await yearSelect.selectOption(String(period.year));
		await monthSelect.selectOption(String(period.month));

		const generateResponsePromise = page.waitForResponse((response) => {
			return (
				response.request().method() === 'POST' &&
				/\/api\/v1\/tenants\/[^/]+\/tax\/kmd$/.test(responsePath(response.url()))
			);
		});
		const generateButton = page.getByRole('button', { name: /generate|genereeri/i });
		await expect(generateButton).toBeVisible();
		await expect(generateButton).toBeEnabled();
		await generateButton.click();

		const generateResponse = await generateResponsePromise;
		expect(generateResponse.ok()).toBeTruthy();
		const declaration = (await generateResponse.json()) as KMDDeclarationResponse;
		expect(declaration.year).toBe(period.year);
		expect(declaration.month).toBe(period.month);
		expect(declaration.status).toBe('DRAFT');
		expect(Array.isArray(declaration.rows)).toBeTruthy();

		const declarationRow = page.locator('.declarations-list tbody tr').filter({ hasText: periodLabel }).first();
		await expect(declarationRow).toBeVisible({ timeout: 10000 });
		await expect(declarationRow).toContainText(/draft|mustand/i);
		await expect(declarationRow).toContainText(/eur/i);

		const detailPanel = page.locator('.declaration-detail');
		await expect(detailPanel).toBeVisible({ timeout: 10000 });
		await expect(detailPanel.getByRole('heading', { name: new RegExp(`KMD ${periodLabel}`) })).toBeVisible();
		await expect(detailPanel).toContainText(`${formatAmount(declaration.total_output_vat)} EUR`);
		await expect(detailPanel).toContainText(`${formatAmount(declaration.total_input_vat)} EUR`);
		await expect(detailPanel.getByRole('button', { name: /export xml|ekspordi xml/i })).toBeVisible();
		await expect(detailPanel.locator('table thead')).toContainText(/row|code|description|tax|rida|kood|kirjeldus|maks/i);
	});
});
