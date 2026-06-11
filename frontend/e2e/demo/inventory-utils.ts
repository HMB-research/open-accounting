import { expect, type Locator, type Page, type TestInfo } from '@playwright/test';
import { DEMO_API_URL, ensureAuthenticated, ensureDemoTenant, getDemoCredentials, navigateTo, waitForRouteReady } from './utils';

interface ProductResponse {
	id: string;
	code?: string;
	name: string;
}

interface WarehouseResponse {
	id: string;
	code: string;
	name: string;
}

interface DemoApiOptions {
	method?: string;
	body?: unknown;
}

const inventoryTerminalState = '.filters, table.table, .empty-state, .alert-error';

export async function setupInventoryPage(page: Page, testInfo: TestInfo): Promise<void> {
	await ensureAuthenticated(page, testInfo);
	await ensureDemoTenant(page, testInfo);
	await openInventoryPage(page, testInfo);
}

export async function openInventoryPage(page: Page, testInfo: TestInfo): Promise<void> {
	await navigateTo(page, '/inventory', testInfo, { waitForNetworkIdle: false });
	await waitForInventoryPageReady(page);
}

export async function waitForInventoryPageReady(page: Page): Promise<void> {
	await waitForRouteReady(page, 'main h1, h1, .tabs, .filters, table.table, .empty-state, .alert-error', 15000);
	await expect(page.getByText(/^Loading\.\.\.$|^Laadimine\.\.\.$/i).first()).toBeHidden({ timeout: 15000 });
	await waitForRouteReady(page, inventoryTerminalState, 15000);
}

export function productsTab(page: Page): Locator {
	return page.getByRole('button', { name: 'Products' });
}

export function warehousesTab(page: Page): Locator {
	return page.getByRole('button', { name: 'Warehouses' });
}

export function categoriesTab(page: Page): Locator {
	return page.getByRole('button', { name: 'Product Categories' });
}

export function productRow(page: Page, codeOrName: string | RegExp): Locator {
	return page.getByRole('row', { name: codeOrName });
}

export function filterSearch(page: Page): Locator {
	return page.getByPlaceholder('Search');
}

export function filterType(page: Page): Locator {
	return page.locator('.filters select').nth(0);
}

export function filterStatus(page: Page): Locator {
	return page.locator('.filters select').nth(1);
}

export function filterCategory(page: Page): Locator {
	return page.locator('.filters select').nth(2);
}

export async function waitForProductsReload(
	page: Page,
	action: () => Promise<void>,
	expectedParams: Record<string, string> = {}
): Promise<void> {
	const responsePromise = page.waitForResponse((res) => {
		if (res.request().method() !== 'GET') return false;

		const url = new URL(res.url());
		if (!url.pathname.includes('/products')) return false;

		return Object.entries(expectedParams).every(([key, value]) => url.searchParams.get(key) === value);
	});

	await action();
	const response = await responsePromise;
	expect(response.ok()).toBeTruthy();
	await waitForInventoryPageReady(page);
}

export async function createInventoryProduct(
	page: Page,
	testInfo: TestInfo,
	suffix: string
): Promise<ProductResponse> {
	const tenantId = getDemoCredentials(testInfo).tenantId;
	const productCode = `E2E-INV-${suffix}`;
	const response = await demoApiRequest<ProductResponse>(
		page,
		`/api/v1/tenants/${tenantId}/products`,
		{
			method: 'POST',
			body: {
				name: `E2E Inventory ${suffix}`,
				code: productCode,
				product_type: 'GOODS',
				unit: 'pcs',
				sales_price: '19.90',
				vat_rate: '22',
				min_stock_level: '1',
				reorder_point: '2',
				track_inventory: true
			}
		}
	);
	expect(response.status).toBe(201);
	expect(response.body.code).toBe(productCode);
	return response.body;
}

export async function listWarehouses(page: Page, testInfo: TestInfo): Promise<WarehouseResponse[]> {
	const tenantId = getDemoCredentials(testInfo).tenantId;
	const response = await demoApiRequest<WarehouseResponse[]>(
		page,
		`/api/v1/tenants/${tenantId}/warehouses`
	);
	expect(response.status).toBe(200);
	expect(response.body.length).toBeGreaterThanOrEqual(2);
	return response.body;
}

async function demoApiRequest<T>(
	page: Page,
	path: string,
	options: DemoApiOptions = {}
): Promise<{ status: number; body: T }> {
	return page.evaluate(
		async ({ apiUrl, requestPath, requestOptions }) => {
			const token = localStorage.getItem('access_token') || sessionStorage.getItem('access_token');
			if (!token) throw new Error('Missing demo access token');

			const headers: Record<string, string> = {
				Authorization: `Bearer ${token}`
			};
			let body: BodyInit | undefined;
			if (requestOptions.body !== undefined) {
				headers['Content-Type'] = 'application/json';
				body = JSON.stringify(requestOptions.body);
			}

			const response = await fetch(`${apiUrl}${requestPath}`, {
				method: requestOptions.method || 'GET',
				headers,
				body
			});
			const text = await response.text();
			let parsed: unknown = {};
			if (text) {
				try {
					parsed = JSON.parse(text);
				} catch {
					parsed = { raw: text };
				}
			}
			return { status: response.status, body: parsed };
		},
		{ apiUrl: DEMO_API_URL, requestPath: path, requestOptions: options }
	) as Promise<{ status: number; body: T }>;
}
