import { expect, type Locator, type Page, type Response, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, ensureDemoTenant, navigateTo, waitForRouteReady } from './utils';

const balanceLoadedState = '.summary-card, .empty-state, .alert-error';
const summaryRequestPattern = /\/api\/v1\/tenants\/[^/]+\/reports\/balance-confirmations\?/;
const detailRequestPattern =
	/\/api\/v1\/tenants\/[^/]+\/reports\/balance-confirmations\/[^/?]+\?/;
const exportFormats = ['csv', 'xlsx', 'pdf'] as const;

type ExportFormat = (typeof exportFormats)[number];

export async function setupBalanceConfirmationsPage(
	page: Page,
	testInfo: TestInfo
): Promise<void> {
	await ensureAuthenticated(page, testInfo);
	await ensureDemoTenant(page, testInfo);
	await openBalanceConfirmationsPage(page, testInfo);
}

export async function openBalanceConfirmationsPage(page: Page, testInfo: TestInfo): Promise<void> {
	await navigateTo(page, '/reports/balance-confirmations', testInfo, {
		waitForNetworkIdle: false
	});
	await waitForBalancePageReady(page);
}

export async function waitForBalancePageReady(page: Page): Promise<void> {
	await waitForRouteReady(
		page,
		'main h1, #balanceType, #asOfDate, button.btn-primary, .summary-card, .empty-state, .alert-error',
		15000
	);
	await page
		.getByText(/^Loading\.\.\.$|^Laadimine\.\.\.$/i)
		.waitFor({ state: 'hidden', timeout: 15000 })
		.catch(() => {});
	await waitForBalanceDataState(page);
}

export async function waitForBalanceDataState(page: Page): Promise<void> {
	await waitForRouteReady(page, balanceLoadedState, 15000);
}

export async function generateBalanceSummary(page: Page): Promise<Response | null> {
	const responsePromise = page
		.waitForResponse(
			(response) =>
				summaryRequestPattern.test(response.url()) && response.request().method() === 'GET',
			{ timeout: 15000 }
		)
		.catch(() => null);

	await generateButton(page).click();
	const response = await responsePromise;
	await waitForBalanceDataState(page);
	return response;
}

export function balanceTypeSelect(page: Page): Locator {
	return page.locator('select#balanceType');
}

export function asOfDateInput(page: Page): Locator {
	return page.locator('input#asOfDate');
}

export function generateButton(page: Page): Locator {
	return page.locator('.controls-section button.btn-primary');
}

export function summaryCard(page: Page): Locator {
	return page.locator('.summary-card');
}

export function balanceTable(page: Page): Locator {
	return page.locator('table.table').first();
}

export function balanceModal(page: Page): Locator {
	return page.locator('[role="dialog"], .modal').first();
}

export function summaryExportButton(page: Page, format: ExportFormat): Locator {
	return page.getByTestId(`balance-summary-export-${format}`);
}

export function detailExportButton(page: Page, format: ExportFormat): Locator {
	return page.getByTestId(`balance-detail-export-${format}`);
}

export async function hasVisibleError(page: Page): Promise<boolean> {
	return page.locator('.alert-error').isVisible().catch(() => false);
}

export async function hasBalanceSummary(page: Page): Promise<boolean> {
	await waitForBalanceDataState(page);
	if (await hasVisibleError(page)) {
		return false;
	}
	return summaryCard(page).isVisible().catch(() => false);
}

export async function hasBalanceTable(page: Page): Promise<boolean> {
	if (!(await hasBalanceSummary(page))) {
		return false;
	}
	return balanceTable(page).isVisible().catch(() => false);
}

export async function expectTerminalBalanceState(page: Page): Promise<void> {
	await expect(page.locator('.empty-state, .alert-error').first()).toBeVisible();
}

export async function openFirstContactDetailModalIfAvailable(page: Page): Promise<boolean> {
	if (!(await hasBalanceTable(page))) {
		await expectTerminalBalanceState(page);
		return false;
	}

	const viewButton = page.locator('tbody button.btn-sm').first();
	await expect(viewButton).toBeVisible();

	const responsePromise = page
		.waitForResponse(
			(response) =>
				detailRequestPattern.test(response.url()) && response.request().method() === 'GET',
			{ timeout: 15000 }
		)
		.catch(() => null);

	await viewButton.click();
	await responsePromise;
	await expect(balanceModal(page)).toBeVisible({ timeout: 5000 });
	return true;
}

export async function openContactDetailModal(page: Page, contactName: string): Promise<void> {
	if (!(await hasBalanceTable(page))) {
		await expect(balanceTable(page)).toBeVisible();
	}

	const row = balanceTable(page).locator('tbody tr').filter({ hasText: contactName }).first();
	await expect(row).toBeVisible({ timeout: 10000 });
	const responsePromise = page.waitForResponse(
		(response) =>
			detailRequestPattern.test(response.url()) && response.request().method() === 'GET',
		{ timeout: 15000 }
	);

	await row.getByRole('button').click();
	const response = await responsePromise;
	expect(response.status()).toBe(200);
	await expect(balanceModal(page)).toBeVisible({ timeout: 5000 });
}

export async function expectExportResponse(
	page: Page,
	action: () => Promise<void>,
	format: ExportFormat,
	options: { detail?: boolean } = {}
): Promise<void> {
	const responsePromise = page.waitForResponse(
		(response) => {
			const url = response.url();
			const matchesRoute = options.detail
				? detailRequestPattern.test(url)
				: summaryRequestPattern.test(url);
			return (
				matchesRoute &&
				response.request().method() === 'GET' &&
				url.includes(`format=${format}`)
			);
		},
		{ timeout: 15000 }
	);

	await action();
	const response = await responsePromise;
	expect(response.status()).toBe(200);
}
