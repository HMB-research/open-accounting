import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant, waitForRouteReady } from './utils';

const routeLoadTimeout = 30000;
const apiResponseTimeout = 30000;

async function openTaxPage(page: Page, testInfo: TestInfo): Promise<void> {
	await navigateTo(page, '/tax', testInfo, { waitForNetworkIdle: false });
	await waitForRouteReady(page, 'main h1, select#year, select#month, .declarations-list, .empty-state', routeLoadTimeout);
	await waitForTaxPageLoaded(page, routeLoadTimeout);
}

function waitForKMDListResponse(page: Page, timeout = apiResponseTimeout): Promise<void> {
	return page
		.waitForResponse((response) => {
			const url = new URL(response.url());
			return (
				response.request().method() === 'GET' &&
				/\/api\/v1\/tenants\/[^/]+\/tax\/kmd$/.test(url.pathname) &&
				response.status() === 200
			);
		}, { timeout })
		.then(() => undefined);
}

async function waitForTaxPageLoaded(page: Page, timeout = routeLoadTimeout): Promise<void> {
	await expect(async () => {
		const isLoading = await page.locator('.loading-spinner, .spinner').first().isVisible().catch(() => false);
		const hasList = await page.locator('.declarations-list').isVisible().catch(() => false);
		const hasEmpty = await page.locator('.empty-state').isVisible().catch(() => false);
		expect(isLoading === false && (hasList || hasEmpty)).toBeTruthy();
	}).toPass({ timeout });
}

async function generateDeclaration(page: Page, year: string, month: string): Promise<void> {
	await page.locator('select#year').selectOption(year);
	await page.locator('select#month').selectOption(month);

	const generateResponsePromise = page.waitForResponse((response) => {
		const url = new URL(response.url());
		return (
			response.request().method() === 'POST' &&
			/\/api\/v1\/tenants\/[^/]+\/tax\/kmd$/.test(url.pathname)
		);
	}, { timeout: apiResponseTimeout });
	const reloadPromise = waitForKMDListResponse(page);

	await page.getByRole('button', { name: /generate|genereeri/i }).click();

	const generateResponse = await generateResponsePromise;
	expect(generateResponse.status()).toBe(200);
	const generated = (await generateResponse.json()) as { year: number; month: number };
	expect(generated.year).toBe(Number(year));
	expect(generated.month).toBe(Number(month));
	await reloadPromise;
	await waitForTaxPageLoaded(page);
}

test.describe('Tax Overview View', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('renders VAT controls, generates a declaration, and lists VAT amounts', async ({
		page
	}, testInfo) => {
		await openTaxPage(page, testInfo);

		await expect(page.getByRole('heading', { name: /vat|tax|declaration|käibemaks|deklaratsioon/i }).first()).toBeVisible();
		await expect(page.locator('select#year')).toBeVisible();
		await expect(page.locator('select#month')).toBeVisible();
		await expect(page.locator('select#month option')).toHaveCount(12);

		const generateButton = page.getByRole('button', { name: /generate|create|new|genereeri|loo/i }).first();
		await expect(generateButton).toBeVisible();
		await expect(generateButton).toBeEnabled();
		await expect(page.locator('.declarations-list, .empty-state').first()).toBeVisible();

		await generateDeclaration(page, '2026', '6');

		const declarationsList = page.locator('.declarations-list');
		const generatedDeclaration = declarationsList.locator('.declaration-item').filter({ hasText: '2026-06' }).first();
		await expect(declarationsList).toBeVisible();
		await expect(generatedDeclaration).toBeVisible();
		await expect(generatedDeclaration).toContainText(/output vat|input vat|payable|väljundkäibemaks|sisendkäibemaks|tasumisele/i);
		await expect(generatedDeclaration).toContainText(/€|EUR|\d+[,.]\d{2}/);
	});
});
