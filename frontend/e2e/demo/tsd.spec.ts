import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, navigateTo, waitForRouteReady } from './utils';

const demoTsdYear = 2024;
const routeLoadTimeout = 30_000;
const apiResponseTimeout = 30_000;

interface TSDDeclarationResponse {
	id: string;
	period_year: number;
	period_month: number;
	total_payments: string | number;
	total_income_tax: string | number;
	total_social_tax: string | number;
	total_unemployment_employer: string | number;
	total_unemployment_employee: string | number;
	total_funded_pension: string | number;
	status: 'DRAFT' | 'SUBMITTED' | 'ACCEPTED' | 'REJECTED';
	emta_reference?: string | null;
}

function responsePath(responseUrl: string): string {
	return new URL(responseUrl).pathname;
}

function formatAmount(amount: string | number): string {
	return Number(amount).toFixed(2);
}

function totalTaxes(declaration: TSDDeclarationResponse): string {
	return [
		declaration.total_income_tax,
		declaration.total_social_tax,
		declaration.total_unemployment_employer,
		declaration.total_unemployment_employee,
		declaration.total_funded_pension
	]
		.reduce((sum, amount) => sum + Number(amount), 0)
		.toFixed(2);
}

function statusPattern(status: TSDDeclarationResponse['status']): RegExp {
	switch (status) {
		case 'DRAFT':
			return /draft|mustand/i;
		case 'SUBMITTED':
			return /submitted|esitatud/i;
		case 'ACCEPTED':
			return /accepted|aktsepteeritud/i;
		case 'REJECTED':
			return /rejected|tagasi/i;
	}
}

function waitForTSDListResponse(page: Page, expectedYear?: number) {
	return page.waitForResponse((response) => {
		if (response.request().method() !== 'GET' || response.status() !== 200) {
			return false;
		}

		const url = new URL(response.url());
		const isTsdList = /\/api\/v1\/tenants\/[^/]+\/tsd$/.test(url.pathname);
		if (!isTsdList) {
			return false;
		}

		return expectedYear === undefined || url.searchParams.get('year') === String(expectedYear);
	}, { timeout: apiResponseTimeout });
}

function waitForTSDExportResponse(page: Page, declaration: TSDDeclarationResponse, format: 'xml' | 'csv') {
	return page.waitForResponse((response) => {
		return (
			response.request().method() === 'GET' &&
			response.status() === 200 &&
			new RegExp(`/api/v1/tenants/[^/]+/tsd/${declaration.period_year}/${declaration.period_month}/${format}$`)
				.test(responsePath(response.url()))
		);
	}, { timeout: apiResponseTimeout });
}

function waitForTSDSubmitResponse(page: Page, declaration: TSDDeclarationResponse) {
	return page.waitForResponse((response) => {
		return (
			response.request().method() === 'POST' &&
			response.status() === 200 &&
			new RegExp(`/api/v1/tenants/[^/]+/tsd/${declaration.period_year}/${declaration.period_month}/submit$`)
				.test(responsePath(response.url()))
		);
	}, { timeout: apiResponseTimeout });
}

async function waitForTSDLoaded(page: Page): Promise<void> {
	await waitForRouteReady(page, 'table tbody tr, .empty-state', routeLoadTimeout);
	await expect(page.getByText(/^Loading\.\.\.$|^Laadimine\.\.\.$/i)).toHaveCount(0, {
		timeout: routeLoadTimeout
	});
}

async function openTSD(page: Page, testInfo: TestInfo): Promise<void> {
	const initialYear = new Date().getFullYear();
	const listPromise = waitForTSDListResponse(page, initialYear);
	await navigateTo(page, '/tsd', testInfo, { waitForNetworkIdle: false });
	expect((await listPromise).ok()).toBeTruthy();
	await waitForRouteReady(page, 'main h1, .container h1', routeLoadTimeout);
	await waitForTSDLoaded(page);
}

async function selectTSDYear(page: Page, year: number): Promise<TSDDeclarationResponse[]> {
	const listPromise = waitForTSDListResponse(page, year);
	await page.locator('select#yearFilter').selectOption(String(year));
	const response = await listPromise;
	expect(response.ok()).toBeTruthy();
	await waitForTSDLoaded(page);
	return (await response.json()) as TSDDeclarationResponse[];
}

test.describe('Demo TSD Declarations', () => {
	test('covers list, exports, and manual submission controls', async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await openTSD(page, testInfo);

		await expect(page.getByRole('heading', { level: 1, name: /tsd/i })).toBeVisible();
		await expect(page.locator('.info-banner')).toContainText(/e-mta|emta|manual|käsitsi/i);
		await expect(page.locator('.info-banner a[href="https://www.emta.ee"]')).toBeVisible();

		const yearFilter = page.locator('select#yearFilter');
		await expect(yearFilter).toBeVisible();
		const yearOptions = await yearFilter.locator('option').allTextContents();
		expect(yearOptions).toContain(String(demoTsdYear));

		const declarations = await selectTSDYear(page, demoTsdYear);
		expect(declarations.length).toBeGreaterThanOrEqual(3);
		expect(declarations.map((declaration) => `${declaration.period_year}-${declaration.period_month}`)).toEqual(
			expect.arrayContaining(['2024-10', '2024-11', '2024-12'])
		);
		expect(declarations.some((declaration) => declaration.status === 'DRAFT')).toBeTruthy();
		expect(declarations.some((declaration) => declaration.status === 'SUBMITTED')).toBeTruthy();

		const rows = page.locator('table tbody tr');
		await expect(rows).toHaveCount(declarations.length);

		const firstDeclaration = declarations[0];
		const firstRow = rows.first();
		await expect(firstRow).toContainText(String(firstDeclaration.period_year));
		await expect(firstRow).toContainText(formatAmount(firstDeclaration.total_payments));
		await expect(firstRow).toContainText(formatAmount(firstDeclaration.total_income_tax));
		await expect(firstRow).toContainText(formatAmount(firstDeclaration.total_social_tax));
		await expect(firstRow).toContainText(totalTaxes(firstDeclaration));
		await expect(firstRow).toContainText(statusPattern(firstDeclaration.status));
		await expect(firstRow.getByRole('button', { name: 'XML' })).toBeVisible();
		await expect(firstRow.getByRole('button', { name: 'CSV' })).toBeVisible();

		const xmlResponsePromise = waitForTSDExportResponse(page, firstDeclaration, 'xml');
		await firstRow.getByRole('button', { name: 'XML' }).click();
		expect((await xmlResponsePromise).ok()).toBeTruthy();

		const csvResponsePromise = waitForTSDExportResponse(page, firstDeclaration, 'csv');
		await firstRow.getByRole('button', { name: 'CSV' }).click();
		expect((await csvResponsePromise).ok()).toBeTruthy();

		const draftIndex = declarations.findIndex((declaration) => declaration.status === 'DRAFT');
		expect(draftIndex).toBeGreaterThanOrEqual(0);
		const draftRow = rows.nth(draftIndex);
		await draftRow.getByRole('button', { name: /mark.*submitted|märgi.*esitatuks/i }).click();

		const submitDialog = page.getByRole('dialog');
		await expect(submitDialog).toBeVisible();
		await expect(submitDialog.getByRole('heading', { name: /mark.*tsd|märgi.*tsd/i })).toBeVisible();
		const referenceInput = submitDialog.getByLabel(/emta|e-mta|reference|viite/i);
		await expect(referenceInput).toBeVisible();
		await submitDialog.getByRole('button', { name: /cancel|tühista/i }).click();
		await expect(submitDialog).toBeHidden();

		await draftRow.getByRole('button', { name: /mark.*submitted|märgi.*esitatuks/i }).click();
		await expect(submitDialog).toBeVisible();
		const emtaReference = `EMTA-E2E-${Date.now()}`;
		await referenceInput.fill(emtaReference);

		const submitResponsePromise = waitForTSDSubmitResponse(page, declarations[draftIndex]);
		const refreshResponsePromise = waitForTSDListResponse(page, demoTsdYear);
		await submitDialog.getByRole('button', { name: /mark.*submitted|märgi.*esitatuks/i }).click();

		const submitResponse = await submitResponsePromise;
		expect(submitResponse.ok()).toBeTruthy();
		expect(submitResponse.request().postDataJSON()).toMatchObject({
			emta_reference: emtaReference
		});

		const refreshedDeclarations = (await (await refreshResponsePromise).json()) as TSDDeclarationResponse[];
		const submittedDeclaration = refreshedDeclarations.find(
			(declaration) =>
				declaration.period_year === declarations[draftIndex].period_year &&
				declaration.period_month === declarations[draftIndex].period_month
		);
		expect(submittedDeclaration?.status).toBe('SUBMITTED');
		expect(submittedDeclaration?.emta_reference).toBe(emtaReference);

		await expect(submitDialog).toBeHidden();
		await waitForTSDLoaded(page);
		await expect(rows.nth(draftIndex)).toContainText(statusPattern('SUBMITTED'));
		await expect(rows.nth(draftIndex)).toContainText(emtaReference);
		await expect(rows.nth(draftIndex).getByRole('button', { name: /mark.*submitted|märgi.*esitatuks/i })).toHaveCount(0);

		await expect(page.locator('.workflow-steps li')).toHaveCount(6);
		await expect(page.locator('.workflow-info')).toContainText(/xml|e-mta|emta|tsd/i);
	});
});
