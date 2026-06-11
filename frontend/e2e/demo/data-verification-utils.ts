import { expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, ensureDemoTenant, navigateTo, waitForRouteReady } from './utils';

export interface VerificationRoute {
	name: string;
	path: string;
	readySelector: string;
}

export const coreAccountingRoutes: VerificationRoute[] = [
	{
		name: 'Dashboard',
		path: '/dashboard',
		readySelector: 'main h1, .summary-card, .card, .chart-card, .dashboard-header'
	},
	{ name: 'Accounts', path: '/accounts', readySelector: 'main h1, table, .empty-state' },
	{ name: 'Journal', path: '/journal', readySelector: 'main h1, table, .empty-state' }
];

export const receivablesRoutes: VerificationRoute[] = [
	{ name: 'Contacts', path: '/contacts', readySelector: 'main h1, table, .empty-state' },
	{
		name: 'Invoices',
		path: '/invoices',
		readySelector: 'main h1, .workflow-hero, table, .empty-state'
	},
	{ name: 'Payments', path: '/payments', readySelector: 'main h1, table, .empty-state' }
];

export const payrollTaxRoutes: VerificationRoute[] = [
	{ name: 'Employees', path: '/employees', readySelector: 'main h1, table, .empty-state' },
	{
		name: 'Payroll',
		path: '/payroll',
		readySelector: 'main h1, table, .empty-state, .card'
	},
	{ name: 'TSD', path: '/tsd', readySelector: 'main h1, table, .empty-state' }
];

export const operationsRoutes: VerificationRoute[] = [
	{ name: 'Recurring invoices', path: '/recurring', readySelector: 'main h1, table, .empty-state' },
	{
		name: 'Banking',
		path: '/banking',
		readySelector: 'main h1, #bank-account-selector, table, .empty-state, .card'
	},
	{
		name: 'Reports',
		path: '/reports',
		readySelector: 'main h1, main .report-controls, main .reports-grid, main select, main button'
	}
];

export async function prepareDataVerificationPage(page: Page, testInfo: TestInfo): Promise<void> {
	await ensureAuthenticated(page, testInfo);
	await ensureDemoTenant(page, testInfo);
}

export async function verifyDemoRoutes(
	page: Page,
	testInfo: TestInfo,
	routes: VerificationRoute[]
): Promise<void> {
	for (const route of routes) {
		await verifyDemoRoute(page, testInfo, route);
	}
}

async function verifyDemoRoute(
	page: Page,
	testInfo: TestInfo,
	route: VerificationRoute
): Promise<void> {
	await navigateTo(page, route.path, testInfo, { waitForNetworkIdle: false });
	await waitForRouteReady(page, route.readySelector, 15000);
	await waitForPlainLoadingTextToClear(page);
	await expect(page.locator(route.readySelector).first(), `${route.name} content`).toBeVisible({
		timeout: 10000
	});
	await expectNoVisibleErrorAlert(page, route.name);
}

async function waitForPlainLoadingTextToClear(page: Page): Promise<void> {
	await page
		.getByText(/^Loading\.\.\.$|^Laadimine\.\.\.$/i)
		.waitFor({ state: 'hidden', timeout: 10000 })
		.catch(() => {});
}

async function expectNoVisibleErrorAlert(page: Page, routeName: string): Promise<void> {
	await expect(async () => {
		const visibleMessages = await page.locator('.alert-error').evaluateAll((elements) => {
			return elements
				.filter((element) => {
					const style = window.getComputedStyle(element);
					const rect = element.getBoundingClientRect();
					return (
						style.display !== 'none' &&
						style.visibility !== 'hidden' &&
						rect.width > 0 &&
						rect.height > 0
					);
				})
				.map((element) => element.textContent?.trim())
				.filter(Boolean);
		});

		expect(visibleMessages, `${routeName} should not show error alerts`).toHaveLength(0);
	}).toPass({ timeout: 5000 });
}
