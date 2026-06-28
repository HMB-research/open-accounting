import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, ensureDemoTenant, navigateTo } from './utils';

/**
 * Wait for dashboard to finish loading data
 */
async function waitForDashboardLoaded(page: Page) {
	await expect(async () => {
		const isLoading = await page.getByText(/^Loading\.\.\.$/i).first().isVisible().catch(() => false);
		const hasSummary = await page.locator('.summary-grid .summary-card').first().isVisible().catch(() => false);
		const hasChart = await page.locator('.chart-card').first().isVisible().catch(() => false);
		expect(isLoading === false && hasSummary && hasChart).toBeTruthy();
	}).toPass({ timeout: 15000 });
}

function waitForDashboardResponse(page: Page, pathPattern: RegExp): Promise<void> {
	return page
		.waitForResponse((response) => {
			const url = new URL(response.url());
			return (
				response.request().method() === 'GET' &&
				pathPattern.test(url.pathname) &&
				response.status() === 200
			);
		})
		.then(() => undefined);
}

async function openDashboard(page: Page, testInfo: TestInfo) {
	const tenantsLoaded = waitForDashboardResponse(page, /\/api\/v1\/me\/tenants$/);
	const summaryLoaded = waitForDashboardResponse(
		page,
		/\/api\/v1\/tenants\/[^/]+\/analytics\/dashboard$/
	);
	const revenueExpenseLoaded = waitForDashboardResponse(
		page,
		/\/api\/v1\/tenants\/[^/]+\/analytics\/revenue-expense$/
	);
	const cashFlowLoaded = waitForDashboardResponse(
		page,
		/\/api\/v1\/tenants\/[^/]+\/analytics\/cash-flow$/
	);

	await navigateTo(page, '/dashboard', testInfo, { waitForNetworkIdle: false });
	await Promise.all([tenantsLoaded, summaryLoaded, revenueExpenseLoaded, cashFlowLoaded]);
	await waitForDashboardLoaded(page);
}

async function expectDashboardShell(page: Page) {
	const tenantSelector = page.locator('.tenant-selector select').first();
	await expect(tenantSelector).toBeVisible();
	const selectedTenant = await tenantSelector.locator('option:checked').textContent();
	expect(selectedTenant?.trim().length, 'Tenant selector should have selected text').toBeGreaterThan(0);

	await expect(page.getByRole('heading', { name: /dashboard/i })).toBeVisible();
	await expect(page.getByRole('button', { name: /New Organization|uus organisatsioon|\+/i }).first()).toBeVisible();

	const nav = page.getByRole('navigation');
	await expect(nav).toBeVisible();
	await expect(async () => {
		expect(await nav.getByRole('link').count()).toBeGreaterThan(0);
	}).toPass({ timeout: 15000 });

	const summaryCards = page.locator('.summary-grid .summary-card');
	await expect(async () => {
		expect(await summaryCards.count()).toBeGreaterThanOrEqual(5);
	}).toPass({ timeout: 5000 });
	const summaryGrid = page.locator('.summary-grid').first();
	await expect(summaryGrid.getByText(/Revenue|Tulu/i).first()).toBeVisible();
	await expect(summaryGrid.getByText(/Expenses|Kulud/i).first()).toBeVisible();
	await expect(summaryGrid.getByText(/Net Income|Puhaskasum/i).first()).toBeVisible();
	await expect(summaryGrid.getByText(/Receivables|Nõuded/i).first()).toBeVisible();
	await expect(summaryGrid.getByText(/Payables|Kohustused/i).first()).toBeVisible();
	await expect(page.getByText(/Cash Flow|rahavoog/i).first()).toBeVisible();
	await expect(page.getByText(/Recent Activity|viimased tegevused/i).first()).toBeVisible();
	await expect(page.getByText(/Revenue vs Expenses|Tulud vs Kulud/i).first()).toBeVisible();
	await expect(page.getByText(/Quick Actions|kiirtoimingud/i).first()).toBeVisible();
}

test.describe('Demo Dashboard - Seeded Data Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await openDashboard(page, testInfo);
	});

	test('renders tenant dashboard analytics and navigation shell', async ({ page }) => {
		await expectDashboardShell(page);
	});
});
