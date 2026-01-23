import { browser } from '$app/environment';
import { env } from '$env/dynamic/public';
import Decimal from 'decimal.js';
import { authStore } from './stores/auth';

/**
 * Get the API base URL.
 *
 * IMPORTANT: This must be a function (lazy evaluation) instead of a constant.
 *
 * Root Cause:
 * -----------
 * $env/dynamic/public reads environment variables at runtime on the server,
 * then injects them into the client during SSR hydration. If we read the value
 * at module initialization time (const API_BASE = env.PUBLIC_API_URL), the client
 * may not have the values yet (before hydration completes), resulting in undefined
 * and falling back to localhost:8080.
 *
 * Solution:
 * ---------
 * Use a function that reads the env value when actually needed (at request time),
 * not at module initialization time. This ensures the value is read after hydration.
 *
 * @returns The API base URL from PUBLIC_API_URL env var, or localhost:8080 as fallback
 */
export function getApiBase(): string {
	let url = env.PUBLIC_API_URL || 'http://localhost:8080';

	// Ensure URL has a protocol - if missing, add https://
	// This prevents URLs like "example.com/api" being treated as relative paths
	if (url && !url.startsWith('http://') && !url.startsWith('https://')) {
		url = `https://${url}`;
	}

	return url;
}

/**
 * Build a query string from a filter object.
 * Handles undefined/null values by skipping them, and converts
 * boolean values to 'true' string.
 *
 * @param filter - Object with filter parameters
 * @returns Query string with leading '?' or empty string if no params
 */
export function buildQuery(filter?: object): string {
	if (!filter) return '';

	const params = new URLSearchParams();

	for (const [key, value] of Object.entries(filter)) {
		if (value === undefined || value === null || value === '') {
			continue;
		}
		if (typeof value === 'boolean') {
			params.set(key, 'true');
		} else {
			params.set(key, String(value));
		}
	}

	const queryString = params.toString();
	return queryString ? `?${queryString}` : '';
}

interface TokenResponse {
	access_token: string;
	refresh_token: string;
	token_type: string;
	expires_in: number;
	user: {
		id: string;
		email: string;
		name: string;
	};
}

interface ApiError {
	error: string;
}

export interface RetryConfig {
	maxRetries: number;
	baseDelayMs: number;
	maxDelayMs: number;
}

export const DEFAULT_RETRY_CONFIG: RetryConfig = {
	maxRetries: 3,
	baseDelayMs: 1000,
	maxDelayMs: 10000
};

/**
 * Minimal retry config for testing - fast retries with minimal delay
 */
export const TEST_RETRY_CONFIG: RetryConfig = {
	maxRetries: 3,
	baseDelayMs: 10,
	maxDelayMs: 50
};

/**
 * Check if an error is retryable (network errors or server errors)
 */
export function isRetryableError(error: unknown, status?: number): boolean {
	// Network errors (fetch failed)
	if (error instanceof TypeError && error.message.includes('fetch')) {
		return true;
	}

	// Server errors (5xx) are retryable
	if (status && status >= 500 && status <= 599) {
		return true;
	}

	// Rate limiting (429) is retryable
	if (status === 429) {
		return true;
	}

	return false;
}

/**
 * Calculate delay with exponential backoff and jitter
 */
export function calculateBackoffDelay(attempt: number, config: RetryConfig): number {
	// Exponential backoff: base * 2^attempt
	const exponentialDelay = config.baseDelayMs * Math.pow(2, attempt);

	// Add jitter (0-25% of delay) to prevent thundering herd
	const jitter = exponentialDelay * 0.25 * Math.random();

	// Cap at max delay
	return Math.min(exponentialDelay + jitter, config.maxDelayMs);
}

/**
 * Sleep for specified milliseconds
 */
function sleep(ms: number): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, ms));
}

class ApiClient {
	/**
	 * Get the current access token from the auth store
	 */
	private get accessToken(): string | null {
		return authStore.getAccessToken();
	}

	/**
	 * Get the current refresh token from the auth store
	 */
	private get refreshToken(): string | null {
		return authStore.getRefreshToken();
	}

	/**
	 * Set tokens after login - use the auth store
	 * @param access Access token
	 * @param refresh Refresh token
	 * @param rememberMe Whether to persist tokens across browser sessions
	 */
	setTokens(access: string, refresh: string, rememberMe: boolean = false) {
		authStore.setTokens(access, refresh, rememberMe);
	}

	/**
	 * Clear tokens on logout - use the auth store
	 */
	clearTokens() {
		authStore.clearTokens();
	}

	/**
	 * Check if user is authenticated
	 */
	get isAuthenticated(): boolean {
		return !!this.accessToken;
	}

	private async request<T>(
		method: string,
		path: string,
		body?: unknown,
		skipAuth = false,
		retryConfig: RetryConfig = DEFAULT_RETRY_CONFIG
	): Promise<T> {
		const headers: Record<string, string> = {
			'Content-Type': 'application/json'
		};

		if (!skipAuth && this.accessToken) {
			headers['Authorization'] = `Bearer ${this.accessToken}`;
		}

		let lastError: Error | null = null;
		let lastStatus: number | undefined;

		for (let attempt = 0; attempt <= retryConfig.maxRetries; attempt++) {
			try {
				const response = await fetch(`${getApiBase()}${path}`, {
					method,
					headers,
					body: body ? JSON.stringify(body) : undefined
				});

				lastStatus = response.status;

				// Handle token refresh on 401
				if (response.status === 401 && !skipAuth) {
					if (this.refreshToken) {
						const refreshed = await this.refreshAccessToken();
						if (refreshed) {
							return this.request(method, path, body, false, retryConfig);
						}
					}
					// Refresh failed or no refresh token - clear tokens and redirect to login
					this.clearTokens();
					if (browser) {
						window.location.href = '/login';
					}
					throw new Error('Session expired. Please log in again.');
				}

				// Check if we should retry server errors
				if (isRetryableError(null, response.status) && attempt < retryConfig.maxRetries) {
					const delay = calculateBackoffDelay(attempt, retryConfig);
					await sleep(delay);
					continue;
				}

				// Process response
				return await this.processResponse<T>(response);
			} catch (error) {
				lastError = error instanceof Error ? error : new Error(String(error));

				// Only retry on retryable errors
				if (isRetryableError(error, lastStatus) && attempt < retryConfig.maxRetries) {
					const delay = calculateBackoffDelay(attempt, retryConfig);
					await sleep(delay);
					continue;
				}

				throw lastError;
			}
		}

		// Should not reach here, but handle just in case
		throw lastError || new Error('Request failed after retries');
	}

	/**
	 * Process the response and extract data
	 */
	private async processResponse<T>(response: Response): Promise<T> {

		let data: unknown;
		try {
			data = await response.json();
		} catch {
			// Server returned non-JSON response (HTML error page, empty body, etc.)
			if (!response.ok) {
				throw new Error(`Request failed with status ${response.status}`);
			}
			// For successful responses with no/invalid JSON, return empty object
			data = {};
		}

		if (!response.ok) {
			throw new Error((data as ApiError).error || `Request failed with status ${response.status}`);
		}

		return this.parseDecimals(data) as T;
	}

	private parseDecimals(obj: unknown): unknown {
		if (typeof obj === 'string' && /^-?\d+(\.\d+)?$/.test(obj)) {
			return new Decimal(obj);
		}
		if (Array.isArray(obj)) {
			return obj.map((item) => this.parseDecimals(item));
		}
		if (obj !== null && typeof obj === 'object') {
			const result: Record<string, unknown> = {};
			for (const [key, value] of Object.entries(obj)) {
				result[key] = this.parseDecimals(value);
			}
			return result;
		}
		return obj;
	}

	private async refreshAccessToken(): Promise<boolean> {
		try {
			const data = await this.request<{ access_token: string }>(
				'POST',
				'/api/v1/auth/refresh',
				{ refresh_token: this.refreshToken },
				true
			);
			authStore.updateAccessToken(data.access_token);
			return true;
		} catch {
			this.clearTokens();
			return false;
		}
	}

	// Auth endpoints
	async register(email: string, password: string, name: string) {
		return this.request<{ id: string; email: string; name: string }>(
			'POST',
			'/api/v1/auth/register',
			{ email, password, name },
			true
		);
	}

	async login(email: string, password: string, rememberMe: boolean = false, tenantId?: string): Promise<TokenResponse> {
		const data = await this.request<TokenResponse>(
			'POST',
			'/api/v1/auth/login',
			{ email, password, tenant_id: tenantId },
			true
		);
		this.setTokens(data.access_token, data.refresh_token, rememberMe);
		return data;
	}

	logout() {
		this.clearTokens();
	}

	// User endpoints
	async getCurrentUser() {
		return this.request<{ id: string; email: string; name: string; created_at: string }>(
			'GET',
			'/api/v1/me'
		);
	}

	async getMyTenants() {
		return this.request<TenantMembership[]>('GET', '/api/v1/me/tenants');
	}

	// Tenant endpoints
	async createTenant(name: string, slug: string, settings?: TenantSettings) {
		return this.request<Tenant>('POST', '/api/v1/tenants', { name, slug, settings });
	}

	async getTenant(tenantId: string) {
		return this.request<Tenant>('GET', `/api/v1/tenants/${tenantId}`);
	}

	async updateTenant(tenantId: string, data: { name?: string; settings?: Partial<TenantSettings> }) {
		return this.request<Tenant>('PUT', `/api/v1/tenants/${tenantId}`, data);
	}

	async completeOnboarding(tenantId: string) {
		return this.request<{ success: boolean }>('POST', `/api/v1/tenants/${tenantId}/complete-onboarding`);
	}

	// Account endpoints
	async listAccounts(tenantId: string, activeOnly = false) {
		const query = activeOnly ? '?active_only=true' : '';
		return this.request<Account[]>('GET', `/api/v1/tenants/${tenantId}/accounts${query}`);
	}

	async createAccount(tenantId: string, data: CreateAccountRequest) {
		return this.request<Account>('POST', `/api/v1/tenants/${tenantId}/accounts`, data);
	}

	async getAccount(tenantId: string, accountId: string) {
		return this.request<Account>('GET', `/api/v1/tenants/${tenantId}/accounts/${accountId}`);
	}

	// Journal entry endpoints
	async getJournalEntry(tenantId: string, entryId: string) {
		return this.request<JournalEntry>(
			'GET',
			`/api/v1/tenants/${tenantId}/journal-entries/${entryId}`
		);
	}

	async createJournalEntry(tenantId: string, data: CreateJournalEntryRequest) {
		return this.request<JournalEntry>('POST', `/api/v1/tenants/${tenantId}/journal-entries`, data);
	}

	async postJournalEntry(tenantId: string, entryId: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/journal-entries/${entryId}/post`
		);
	}

	async voidJournalEntry(tenantId: string, entryId: string, reason: string) {
		return this.request<JournalEntry>(
			'POST',
			`/api/v1/tenants/${tenantId}/journal-entries/${entryId}/void`,
			{ reason }
		);
	}

	// Report endpoints
	async getTrialBalance(tenantId: string, asOfDate?: string) {
		const query = asOfDate ? `?as_of_date=${asOfDate}` : '';
		return this.request<TrialBalance>(
			'GET',
			`/api/v1/tenants/${tenantId}/reports/trial-balance${query}`
		);
	}

	async getAccountBalance(tenantId: string, accountId: string, asOfDate?: string) {
		const query = asOfDate ? `?as_of_date=${asOfDate}` : '';
		return this.request<{ account_id: string; as_of_date: string; balance: Decimal }>(
			'GET',
			`/api/v1/tenants/${tenantId}/reports/account-balance/${accountId}${query}`
		);
	}

	async getBalanceSheet(tenantId: string, asOfDate?: string) {
		const query = asOfDate ? `?as_of=${asOfDate}` : '';
		return this.request<BalanceSheet>(
			'GET',
			`/api/v1/tenants/${tenantId}/reports/balance-sheet${query}`
		);
	}

	async getIncomeStatement(tenantId: string, startDate: string, endDate: string) {
		return this.request<IncomeStatement>(
			'GET',
			`/api/v1/tenants/${tenantId}/reports/income-statement?start=${startDate}&end=${endDate}`
		);
	}

	// Contact endpoints
	async listContacts(tenantId: string, filter?: ContactFilter) {
		const query = buildQuery(filter);
		return this.request<Contact[]>('GET', `/api/v1/tenants/${tenantId}/contacts${query}`);
	}

	async createContact(tenantId: string, data: CreateContactRequest) {
		return this.request<Contact>('POST', `/api/v1/tenants/${tenantId}/contacts`, data);
	}

	async getContact(tenantId: string, contactId: string) {
		return this.request<Contact>('GET', `/api/v1/tenants/${tenantId}/contacts/${contactId}`);
	}

	async updateContact(tenantId: string, contactId: string, data: UpdateContactRequest) {
		return this.request<Contact>('PUT', `/api/v1/tenants/${tenantId}/contacts/${contactId}`, data);
	}

	async deleteContact(tenantId: string, contactId: string) {
		return this.request<{ status: string }>(
			'DELETE',
			`/api/v1/tenants/${tenantId}/contacts/${contactId}`
		);
	}

	// Invoice endpoints
	async listInvoices(tenantId: string, filter?: InvoiceFilter) {
		const query = buildQuery(filter);
		return this.request<Invoice[]>('GET', `/api/v1/tenants/${tenantId}/invoices${query}`);
	}

	async createInvoice(tenantId: string, data: CreateInvoiceRequest) {
		return this.request<Invoice>('POST', `/api/v1/tenants/${tenantId}/invoices`, data);
	}

	async getInvoice(tenantId: string, invoiceId: string) {
		return this.request<Invoice>('GET', `/api/v1/tenants/${tenantId}/invoices/${invoiceId}`);
	}

	async sendInvoice(tenantId: string, invoiceId: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/invoices/${invoiceId}/send`
		);
	}

	async voidInvoice(tenantId: string, invoiceId: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/invoices/${invoiceId}/void`
		);
	}

	async downloadInvoicePDF(tenantId: string, invoiceId: string, invoiceNumber: string) {
		const headers: Record<string, string> = {};
		if (this.accessToken) {
			headers['Authorization'] = `Bearer ${this.accessToken}`;
		}

		const response = await fetch(
			`${getApiBase()}/api/v1/tenants/${tenantId}/invoices/${invoiceId}/pdf`,
			{ headers }
		);

		if (!response.ok) {
			throw new Error('Failed to download PDF');
		}

		const blob = await response.blob();
		const url = window.URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `invoice-${invoiceNumber}.pdf`;
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
		window.URL.revokeObjectURL(url);
	}

	// Payment endpoints
	async listPayments(tenantId: string, filter?: PaymentFilter) {
		const query = buildQuery(filter);
		return this.request<Payment[]>('GET', `/api/v1/tenants/${tenantId}/payments${query}`);
	}

	async createPayment(tenantId: string, data: CreatePaymentRequest) {
		return this.request<Payment>('POST', `/api/v1/tenants/${tenantId}/payments`, data);
	}

	async getPayment(tenantId: string, paymentId: string) {
		return this.request<Payment>('GET', `/api/v1/tenants/${tenantId}/payments/${paymentId}`);
	}

	async allocatePayment(tenantId: string, paymentId: string, invoiceId: string, amount: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/payments/${paymentId}/allocate`,
			{ invoice_id: invoiceId, amount }
		);
	}

	async getUnallocatedPayments(tenantId: string, type: 'RECEIVED' | 'MADE' = 'RECEIVED') {
		return this.request<Payment[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/payments/unallocated?type=${type}`
		);
	}

	// Quote endpoints
	async listQuotes(tenantId: string, filter?: QuoteFilter) {
		const query = buildQuery(filter);
		return this.request<Quote[]>('GET', `/api/v1/tenants/${tenantId}/quotes${query}`);
	}

	async createQuote(tenantId: string, data: CreateQuoteRequest) {
		return this.request<Quote>('POST', `/api/v1/tenants/${tenantId}/quotes`, data);
	}

	async getQuote(tenantId: string, quoteId: string) {
		return this.request<Quote>('GET', `/api/v1/tenants/${tenantId}/quotes/${quoteId}`);
	}

	async updateQuote(tenantId: string, quoteId: string, data: UpdateQuoteRequest) {
		return this.request<Quote>('PUT', `/api/v1/tenants/${tenantId}/quotes/${quoteId}`, data);
	}

	async deleteQuote(tenantId: string, quoteId: string) {
		return this.request<void>('DELETE', `/api/v1/tenants/${tenantId}/quotes/${quoteId}`);
	}

	async sendQuote(tenantId: string, quoteId: string) {
		return this.request<{ status: string }>('POST', `/api/v1/tenants/${tenantId}/quotes/${quoteId}/send`);
	}

	async acceptQuote(tenantId: string, quoteId: string) {
		return this.request<{ status: string }>('POST', `/api/v1/tenants/${tenantId}/quotes/${quoteId}/accept`);
	}

	async rejectQuote(tenantId: string, quoteId: string) {
		return this.request<{ status: string }>('POST', `/api/v1/tenants/${tenantId}/quotes/${quoteId}/reject`);
	}

	// Orders endpoints
	async listOrders(tenantId: string, filter?: OrderFilter) {
		const query = buildQuery(filter);
		return this.request<Order[]>('GET', `/api/v1/tenants/${tenantId}/orders${query}`);
	}

	async createOrder(tenantId: string, data: CreateOrderRequest) {
		return this.request<Order>('POST', `/api/v1/tenants/${tenantId}/orders`, data);
	}

	async getOrder(tenantId: string, orderId: string) {
		return this.request<Order>('GET', `/api/v1/tenants/${tenantId}/orders/${orderId}`);
	}

	async updateOrder(tenantId: string, orderId: string, data: UpdateOrderRequest) {
		return this.request<Order>('PUT', `/api/v1/tenants/${tenantId}/orders/${orderId}`, data);
	}

	async deleteOrder(tenantId: string, orderId: string) {
		return this.request<void>('DELETE', `/api/v1/tenants/${tenantId}/orders/${orderId}`);
	}

	async confirmOrder(tenantId: string, orderId: string) {
		return this.request<{ status: string }>('POST', `/api/v1/tenants/${tenantId}/orders/${orderId}/confirm`);
	}

	async processOrder(tenantId: string, orderId: string) {
		return this.request<{ status: string }>('POST', `/api/v1/tenants/${tenantId}/orders/${orderId}/process`);
	}

	async shipOrder(tenantId: string, orderId: string) {
		return this.request<{ status: string }>('POST', `/api/v1/tenants/${tenantId}/orders/${orderId}/ship`);
	}

	async deliverOrder(tenantId: string, orderId: string) {
		return this.request<{ status: string }>('POST', `/api/v1/tenants/${tenantId}/orders/${orderId}/deliver`);
	}

	async cancelOrder(tenantId: string, orderId: string) {
		return this.request<{ status: string }>('POST', `/api/v1/tenants/${tenantId}/orders/${orderId}/cancel`);
	}

	// Fixed Assets - Categories endpoints
	async listAssetCategories(tenantId: string) {
		return this.request<AssetCategory[]>('GET', `/api/v1/tenants/${tenantId}/asset-categories`);
	}

	async createAssetCategory(tenantId: string, data: CreateAssetCategoryRequest) {
		return this.request<AssetCategory>('POST', `/api/v1/tenants/${tenantId}/asset-categories`, data);
	}

	async getAssetCategory(tenantId: string, categoryId: string) {
		return this.request<AssetCategory>('GET', `/api/v1/tenants/${tenantId}/asset-categories/${categoryId}`);
	}

	async deleteAssetCategory(tenantId: string, categoryId: string) {
		return this.request<void>('DELETE', `/api/v1/tenants/${tenantId}/asset-categories/${categoryId}`);
	}

	// Fixed Assets endpoints
	async listAssets(tenantId: string, filter?: AssetFilter) {
		const params = new URLSearchParams();
		if (filter?.status) params.set('status', filter.status);
		if (filter?.category_id) params.set('category_id', filter.category_id);
		if (filter?.from_date) params.set('from_date', filter.from_date);
		if (filter?.to_date) params.set('to_date', filter.to_date);
		if (filter?.search) params.set('search', filter.search);
		const query = params.toString() ? `?${params.toString()}` : '';
		return this.request<FixedAsset[]>('GET', `/api/v1/tenants/${tenantId}/assets${query}`);
	}

	async createAsset(tenantId: string, data: CreateAssetRequest) {
		return this.request<FixedAsset>('POST', `/api/v1/tenants/${tenantId}/assets`, data);
	}

	async getAsset(tenantId: string, assetId: string) {
		return this.request<FixedAsset>('GET', `/api/v1/tenants/${tenantId}/assets/${assetId}`);
	}

	async updateAsset(tenantId: string, assetId: string, data: UpdateAssetRequest) {
		return this.request<FixedAsset>('PUT', `/api/v1/tenants/${tenantId}/assets/${assetId}`, data);
	}

	async deleteAsset(tenantId: string, assetId: string) {
		return this.request<void>('DELETE', `/api/v1/tenants/${tenantId}/assets/${assetId}`);
	}

	async activateAsset(tenantId: string, assetId: string) {
		return this.request<{ status: string }>('POST', `/api/v1/tenants/${tenantId}/assets/${assetId}/activate`);
	}

	async disposeAsset(tenantId: string, assetId: string, data: DisposeAssetRequest) {
		return this.request<{ status: string }>('POST', `/api/v1/tenants/${tenantId}/assets/${assetId}/dispose`, data);
	}

	async recordDepreciation(tenantId: string, assetId: string, data: RecordDepreciationRequest) {
		return this.request<DepreciationEntry>('POST', `/api/v1/tenants/${tenantId}/assets/${assetId}/depreciate`, data);
	}

	async getDepreciationHistory(tenantId: string, assetId: string) {
		return this.request<DepreciationEntry[]>('GET', `/api/v1/tenants/${tenantId}/assets/${assetId}/depreciation`);
	}

	// Inventory - Product Categories endpoints
	async listProductCategories(tenantId: string) {
		return this.request<ProductCategory[]>('GET', `/api/v1/tenants/${tenantId}/product-categories`);
	}

	async createProductCategory(tenantId: string, data: CreateProductCategoryRequest) {
		return this.request<ProductCategory>('POST', `/api/v1/tenants/${tenantId}/product-categories`, data);
	}

	async getProductCategory(tenantId: string, categoryId: string) {
		return this.request<ProductCategory>('GET', `/api/v1/tenants/${tenantId}/product-categories/${categoryId}`);
	}

	async deleteProductCategory(tenantId: string, categoryId: string) {
		return this.request<void>('DELETE', `/api/v1/tenants/${tenantId}/product-categories/${categoryId}`);
	}

	// Inventory - Products endpoints
	async listProducts(tenantId: string, filter?: ProductFilter) {
		const params = new URLSearchParams();
		if (filter?.product_type) params.set('product_type', filter.product_type);
		if (filter?.status) params.set('status', filter.status);
		if (filter?.category_id) params.set('category_id', filter.category_id);
		if (filter?.search) params.set('search', filter.search);
		if (filter?.low_stock) params.set('low_stock', 'true');
		const query = params.toString() ? `?${params.toString()}` : '';
		return this.request<Product[]>('GET', `/api/v1/tenants/${tenantId}/products${query}`);
	}

	async createProduct(tenantId: string, data: CreateProductRequest) {
		return this.request<Product>('POST', `/api/v1/tenants/${tenantId}/products`, data);
	}

	async getProduct(tenantId: string, productId: string) {
		return this.request<Product>('GET', `/api/v1/tenants/${tenantId}/products/${productId}`);
	}

	async updateProduct(tenantId: string, productId: string, data: UpdateProductRequest) {
		return this.request<Product>('PUT', `/api/v1/tenants/${tenantId}/products/${productId}`, data);
	}

	async deleteProduct(tenantId: string, productId: string) {
		return this.request<void>('DELETE', `/api/v1/tenants/${tenantId}/products/${productId}`);
	}

	async getProductStockLevels(tenantId: string, productId: string) {
		return this.request<StockLevel[]>('GET', `/api/v1/tenants/${tenantId}/products/${productId}/stock`);
	}

	async getProductMovements(tenantId: string, productId: string) {
		return this.request<InventoryMovement[]>('GET', `/api/v1/tenants/${tenantId}/products/${productId}/movements`);
	}

	// Inventory - Warehouses endpoints
	async listWarehouses(tenantId: string, activeOnly = false) {
		const query = activeOnly ? '?active_only=true' : '';
		return this.request<Warehouse[]>('GET', `/api/v1/tenants/${tenantId}/warehouses${query}`);
	}

	async createWarehouse(tenantId: string, data: CreateWarehouseRequest) {
		return this.request<Warehouse>('POST', `/api/v1/tenants/${tenantId}/warehouses`, data);
	}

	async getWarehouse(tenantId: string, warehouseId: string) {
		return this.request<Warehouse>('GET', `/api/v1/tenants/${tenantId}/warehouses/${warehouseId}`);
	}

	async updateWarehouse(tenantId: string, warehouseId: string, data: UpdateWarehouseRequest) {
		return this.request<Warehouse>('PUT', `/api/v1/tenants/${tenantId}/warehouses/${warehouseId}`, data);
	}

	async deleteWarehouse(tenantId: string, warehouseId: string) {
		return this.request<void>('DELETE', `/api/v1/tenants/${tenantId}/warehouses/${warehouseId}`);
	}

	// Inventory - Stock Operations
	async adjustStock(tenantId: string, data: AdjustStockRequest) {
		return this.request<InventoryMovement>('POST', `/api/v1/tenants/${tenantId}/inventory/adjust`, data);
	}

	async transferStock(tenantId: string, data: TransferStockRequest) {
		return this.request<{ status: string }>('POST', `/api/v1/tenants/${tenantId}/inventory/transfer`, data);
	}

	// Analytics endpoints
	async getDashboardSummary(tenantId: string) {
		return this.request<DashboardSummary>('GET', `/api/v1/tenants/${tenantId}/analytics/dashboard`);
	}

	async getRevenueExpenseChart(tenantId: string, months = 12) {
		return this.request<RevenueExpenseChart>(
			'GET',
			`/api/v1/tenants/${tenantId}/analytics/revenue-expense?months=${months}`
		);
	}

	async getCashFlowChart(tenantId: string, months = 12) {
		return this.request<CashFlowChart>(
			'GET',
			`/api/v1/tenants/${tenantId}/analytics/cash-flow?months=${months}`
		);
	}

	async getReceivablesAging(tenantId: string) {
		return this.request<AgingReport>('GET', `/api/v1/tenants/${tenantId}/reports/aging/receivables`);
	}

	async getPayablesAging(tenantId: string) {
		return this.request<AgingReport>('GET', `/api/v1/tenants/${tenantId}/reports/aging/payables`);
	}

	async getRecentActivity(tenantId: string, limit = 10) {
		return this.request<ActivityItem[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/analytics/activity?limit=${limit}`
		);
	}

	async getCashFlowAnalytics(tenantId: string, startDate: string, endDate: string) {
		// Backend expects months parameter, calculate months from date range
		const start = new Date(startDate);
		const end = new Date(endDate);
		const months = Math.max(1, Math.ceil((end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24 * 30)));
		return this.request<CashFlowChart>(
			'GET',
			`/api/v1/tenants/${tenantId}/analytics/cash-flow?months=${months}`
		);
	}

	// Recurring Invoice endpoints
	async listRecurringInvoices(tenantId: string, activeOnly = false) {
		const query = activeOnly ? '?active_only=true' : '';
		return this.request<RecurringInvoice[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/recurring-invoices${query}`
		);
	}

	async createRecurringInvoice(tenantId: string, data: CreateRecurringInvoiceRequest) {
		return this.request<RecurringInvoice>(
			'POST',
			`/api/v1/tenants/${tenantId}/recurring-invoices`,
			data
		);
	}

	async createRecurringInvoiceFromInvoice(
		tenantId: string,
		invoiceId: string,
		data: CreateFromInvoiceRequest
	) {
		return this.request<RecurringInvoice>(
			'POST',
			`/api/v1/tenants/${tenantId}/recurring-invoices/from-invoice/${invoiceId}`,
			data
		);
	}

	async getRecurringInvoice(tenantId: string, recurringId: string) {
		return this.request<RecurringInvoice>(
			'GET',
			`/api/v1/tenants/${tenantId}/recurring-invoices/${recurringId}`
		);
	}

	async updateRecurringInvoice(
		tenantId: string,
		recurringId: string,
		data: UpdateRecurringInvoiceRequest
	) {
		return this.request<RecurringInvoice>(
			'PUT',
			`/api/v1/tenants/${tenantId}/recurring-invoices/${recurringId}`,
			data
		);
	}

	async deleteRecurringInvoice(tenantId: string, recurringId: string) {
		return this.request<{ status: string }>(
			'DELETE',
			`/api/v1/tenants/${tenantId}/recurring-invoices/${recurringId}`
		);
	}

	async pauseRecurringInvoice(tenantId: string, recurringId: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/recurring-invoices/${recurringId}/pause`
		);
	}

	async resumeRecurringInvoice(tenantId: string, recurringId: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/recurring-invoices/${recurringId}/resume`
		);
	}

	async generateRecurringInvoice(tenantId: string, recurringId: string) {
		return this.request<GenerationResult>(
			'POST',
			`/api/v1/tenants/${tenantId}/recurring-invoices/${recurringId}/generate`
		);
	}

	async generateDueRecurringInvoices(tenantId: string) {
		return this.request<GenerationResult[]>(
			'POST',
			`/api/v1/tenants/${tenantId}/recurring-invoices/generate-due`
		);
	}

	// Email endpoints
	async getSMTPConfig(tenantId: string) {
		return this.request<SMTPConfig>('GET', `/api/v1/tenants/${tenantId}/settings/smtp`);
	}

	async updateSMTPConfig(tenantId: string, data: UpdateSMTPConfigRequest) {
		return this.request<{ status: string }>(
			'PUT',
			`/api/v1/tenants/${tenantId}/settings/smtp`,
			data
		);
	}

	async testSMTP(tenantId: string, recipientEmail: string) {
		return this.request<TestSMTPResponse>(
			'POST',
			`/api/v1/tenants/${tenantId}/settings/smtp/test`,
			{ recipient_email: recipientEmail }
		);
	}

	async listEmailTemplates(tenantId: string) {
		return this.request<EmailTemplate[]>('GET', `/api/v1/tenants/${tenantId}/email-templates`);
	}

	async updateEmailTemplate(tenantId: string, templateType: TemplateType, data: UpdateTemplateRequest) {
		return this.request<EmailTemplate>(
			'PUT',
			`/api/v1/tenants/${tenantId}/email-templates/${templateType}`,
			data
		);
	}

	async getEmailLog(tenantId: string, limit = 50) {
		return this.request<EmailLog[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/email-log?limit=${limit}`
		);
	}

	async emailInvoice(tenantId: string, invoiceId: string, data: SendInvoiceEmailRequest) {
		return this.request<EmailSentResponse>(
			'POST',
			`/api/v1/tenants/${tenantId}/invoices/${invoiceId}/email`,
			data
		);
	}

	async emailPaymentReceipt(tenantId: string, paymentId: string, data: SendPaymentReceiptRequest) {
		return this.request<EmailSentResponse>(
			'POST',
			`/api/v1/tenants/${tenantId}/payments/${paymentId}/email-receipt`,
			data
		);
	}

	// Banking endpoints
	async listBankAccounts(tenantId: string, activeOnly = false) {
		const query = activeOnly ? '?active_only=true' : '';
		return this.request<BankAccount[]>('GET', `/api/v1/tenants/${tenantId}/bank-accounts${query}`);
	}

	async createBankAccount(tenantId: string, data: CreateBankAccountRequest) {
		return this.request<BankAccount>('POST', `/api/v1/tenants/${tenantId}/bank-accounts`, data);
	}

	async getBankAccount(tenantId: string, accountId: string) {
		return this.request<BankAccount>('GET', `/api/v1/tenants/${tenantId}/bank-accounts/${accountId}`);
	}

	async updateBankAccount(tenantId: string, accountId: string, data: UpdateBankAccountRequest) {
		return this.request<BankAccount>(
			'PUT',
			`/api/v1/tenants/${tenantId}/bank-accounts/${accountId}`,
			data
		);
	}

	async deleteBankAccount(tenantId: string, accountId: string) {
		return this.request<void>('DELETE', `/api/v1/tenants/${tenantId}/bank-accounts/${accountId}`);
	}

	async listBankTransactions(
		tenantId: string,
		accountId: string,
		filters?: { status?: TransactionStatus; from_date?: string; to_date?: string }
	) {
		const params = new URLSearchParams();
		if (filters?.status) params.set('status', filters.status);
		if (filters?.from_date) params.set('from_date', filters.from_date);
		if (filters?.to_date) params.set('to_date', filters.to_date);
		const query = params.toString() ? `?${params.toString()}` : '';
		return this.request<BankTransaction[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/bank-accounts/${accountId}/transactions${query}`
		);
	}

	async getBankTransaction(tenantId: string, transactionId: string) {
		return this.request<BankTransaction>(
			'GET',
			`/api/v1/tenants/${tenantId}/bank-transactions/${transactionId}`
		);
	}

	async importBankTransactions(tenantId: string, accountId: string, data: ImportTransactionsRequest) {
		return this.request<ImportResult>(
			'POST',
			`/api/v1/tenants/${tenantId}/bank-accounts/${accountId}/import`,
			data
		);
	}

	async getImportHistory(tenantId: string, accountId: string) {
		return this.request<BankStatementImport[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/bank-accounts/${accountId}/import-history`
		);
	}

	async getMatchSuggestions(tenantId: string, transactionId: string) {
		return this.request<MatchSuggestion[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/bank-transactions/${transactionId}/suggestions`
		);
	}

	async matchBankTransaction(tenantId: string, transactionId: string, paymentId: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/bank-transactions/${transactionId}/match`,
			{ payment_id: paymentId }
		);
	}

	async unmatchBankTransaction(tenantId: string, transactionId: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/bank-transactions/${transactionId}/unmatch`
		);
	}

	async createPaymentFromTransaction(tenantId: string, transactionId: string) {
		return this.request<{ payment_id: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/bank-transactions/${transactionId}/create-payment`
		);
	}

	async listReconciliations(tenantId: string, accountId: string) {
		return this.request<BankReconciliation[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/bank-accounts/${accountId}/reconciliations`
		);
	}

	async createReconciliation(tenantId: string, accountId: string, data: CreateReconciliationRequest) {
		return this.request<BankReconciliation>(
			'POST',
			`/api/v1/tenants/${tenantId}/bank-accounts/${accountId}/reconciliation`,
			data
		);
	}

	async getReconciliation(tenantId: string, reconciliationId: string) {
		return this.request<BankReconciliation>(
			'GET',
			`/api/v1/tenants/${tenantId}/reconciliations/${reconciliationId}`
		);
	}

	async completeReconciliation(tenantId: string, reconciliationId: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/reconciliations/${reconciliationId}/complete`
		);
	}

	async autoMatchTransactions(tenantId: string, accountId: string, minConfidence = 0.7) {
		return this.request<{ matched: number }>(
			'POST',
			`/api/v1/tenants/${tenantId}/bank-accounts/${accountId}/auto-match?min_confidence=${minConfidence}`
		);
	}

	// Tax (KMD) endpoints
	async generateKMD(tenantId: string, data: CreateKMDRequest) {
		return this.request<KMDDeclaration>('POST', `/api/v1/tenants/${tenantId}/tax/kmd`, data);
	}

	async listKMD(tenantId: string) {
		return this.request<KMDDeclaration[]>('GET', `/api/v1/tenants/${tenantId}/tax/kmd`);
	}

	async downloadKMDXml(tenantId: string, year: number, month: number) {
		const headers: Record<string, string> = {};
		if (this.accessToken) {
			headers['Authorization'] = `Bearer ${this.accessToken}`;
		}

		const response = await fetch(
			`${getApiBase()}/api/v1/tenants/${tenantId}/tax/kmd/${year}/${month}/xml`,
			{ headers }
		);

		if (!response.ok) {
			throw new Error('Failed to download XML');
		}

		const blob = await response.blob();
		const url = window.URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `KMD_${year}_${String(month).padStart(2, '0')}.xml`;
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
		window.URL.revokeObjectURL(url);
	}

	// Cash Flow Statement endpoint
	async getCashFlowStatement(tenantId: string, startDate: string, endDate: string) {
		return this.request<CashFlowStatement>(
			'GET',
			`/api/v1/tenants/${tenantId}/reports/cash-flow?start_date=${startDate}&end_date=${endDate}`
		);
	}

	// Balance Confirmation endpoints
	async getBalanceConfirmationSummary(
		tenantId: string,
		type: BalanceConfirmationType,
		asOfDate: string
	) {
		return this.request<BalanceConfirmationSummary>(
			'GET',
			`/api/v1/tenants/${tenantId}/reports/balance-confirmations?type=${type}&as_of_date=${asOfDate}`
		);
	}

	async getBalanceConfirmation(
		tenantId: string,
		contactId: string,
		type: BalanceConfirmationType,
		asOfDate: string
	) {
		return this.request<BalanceConfirmation>(
			'GET',
			`/api/v1/tenants/${tenantId}/reports/balance-confirmations/${contactId}?type=${type}&as_of_date=${asOfDate}`
		);
	}

	// Payment Reminder endpoints
	async getOverdueInvoices(tenantId: string) {
		return this.request<OverdueInvoicesSummary>(
			'GET',
			`/api/v1/tenants/${tenantId}/invoices/overdue`
		);
	}

	async sendPaymentReminder(tenantId: string, invoiceId: string, message?: string) {
		return this.request<ReminderResult>('POST', `/api/v1/tenants/${tenantId}/invoices/reminders`, {
			invoice_id: invoiceId,
			message
		});
	}

	async sendBulkPaymentReminders(tenantId: string, invoiceIds: string[], message?: string) {
		return this.request<BulkReminderResult>(
			'POST',
			`/api/v1/tenants/${tenantId}/invoices/reminders/bulk`,
			{
				invoice_ids: invoiceIds,
				message
			}
		);
	}

	async getInvoiceReminderHistory(tenantId: string, invoiceId: string) {
		return this.request<PaymentReminder[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/invoices/${invoiceId}/reminders`
		);
	}

	// Cost Centers
	async listCostCenters(tenantId: string, activeOnly = false) {
		const query = activeOnly ? '?active_only=true' : '';
		return this.request<CostCenter[]>('GET', `/api/v1/tenants/${tenantId}/cost-centers${query}`);
	}

	async getCostCenter(tenantId: string, costCenterId: string) {
		return this.request<CostCenter>(
			'GET',
			`/api/v1/tenants/${tenantId}/cost-centers/${costCenterId}`
		);
	}

	async createCostCenter(tenantId: string, data: CreateCostCenterRequest) {
		return this.request<CostCenter>('POST', `/api/v1/tenants/${tenantId}/cost-centers`, data);
	}

	async updateCostCenter(tenantId: string, costCenterId: string, data: UpdateCostCenterRequest) {
		return this.request<CostCenter>(
			'PUT',
			`/api/v1/tenants/${tenantId}/cost-centers/${costCenterId}`,
			data
		);
	}

	async deleteCostCenter(tenantId: string, costCenterId: string) {
		return this.request<void>(
			'DELETE',
			`/api/v1/tenants/${tenantId}/cost-centers/${costCenterId}`
		);
	}

	async getCostCenterReport(tenantId: string, startDate?: string, endDate?: string) {
		const params = new URLSearchParams();
		if (startDate) params.append('start_date', startDate);
		if (endDate) params.append('end_date', endDate);
		const query = params.toString() ? `?${params.toString()}` : '';
		return this.request<CostCenterReport>(
			'GET',
			`/api/v1/tenants/${tenantId}/cost-centers/report${query}`
		);
	}

	// Payroll - Employee endpoints
	async listEmployees(tenantId: string, activeOnly = false) {
		const query = activeOnly ? '?active_only=true' : '';
		return this.request<Employee[]>('GET', `/api/v1/tenants/${tenantId}/employees${query}`);
	}

	async createEmployee(tenantId: string, data: CreateEmployeeRequest) {
		return this.request<Employee>('POST', `/api/v1/tenants/${tenantId}/employees`, data);
	}

	async getEmployee(tenantId: string, employeeId: string) {
		return this.request<Employee>('GET', `/api/v1/tenants/${tenantId}/employees/${employeeId}`);
	}

	async updateEmployee(tenantId: string, employeeId: string, data: UpdateEmployeeRequest) {
		return this.request<Employee>(
			'PUT',
			`/api/v1/tenants/${tenantId}/employees/${employeeId}`,
			data
		);
	}

	async setBaseSalary(tenantId: string, employeeId: string, amount: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/employees/${employeeId}/salary`,
			{ amount }
		);
	}

	// Payroll - Payroll Run endpoints
	async listPayrollRuns(tenantId: string, year?: number) {
		const query = year ? `?year=${year}` : '';
		return this.request<PayrollRun[]>('GET', `/api/v1/tenants/${tenantId}/payroll-runs${query}`);
	}

	async createPayrollRun(tenantId: string, data: CreatePayrollRunRequest) {
		return this.request<PayrollRun>('POST', `/api/v1/tenants/${tenantId}/payroll-runs`, data);
	}

	async getPayrollRun(tenantId: string, runId: string) {
		return this.request<PayrollRun>('GET', `/api/v1/tenants/${tenantId}/payroll-runs/${runId}`);
	}

	async calculatePayroll(tenantId: string, runId: string) {
		return this.request<PayrollRun>(
			'POST',
			`/api/v1/tenants/${tenantId}/payroll-runs/${runId}/calculate`
		);
	}

	async approvePayroll(tenantId: string, runId: string) {
		return this.request<PayrollRun>(
			'POST',
			`/api/v1/tenants/${tenantId}/payroll-runs/${runId}/approve`
		);
	}

	async getPayslips(tenantId: string, runId: string) {
		return this.request<Payslip[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/payroll-runs/${runId}/payslips`
		);
	}

	async generateTSD(tenantId: string, runId: string) {
		return this.request<TSDDeclaration>(
			'POST',
			`/api/v1/tenants/${tenantId}/payroll-runs/${runId}/tsd`
		);
	}

	// Payroll - Tax Preview
	async calculateTaxPreview(tenantId: string, grossSalary: string, basicExemption?: string, fundedPensionRate?: string) {
		return this.request<TaxCalculation>(
			'POST',
			`/api/v1/tenants/${tenantId}/payroll/tax-preview`,
			{ gross_salary: grossSalary, basic_exemption: basicExemption, funded_pension_rate: fundedPensionRate }
		);
	}

	// Payroll - TSD endpoints
	async listTSD(tenantId: string, year?: number) {
		const query = year ? `?year=${year}` : '';
		return this.request<TSDDeclaration[]>('GET', `/api/v1/tenants/${tenantId}/tsd${query}`);
	}

	async getTSD(tenantId: string, year: number, month: number) {
		return this.request<TSDDeclaration>(
			'GET',
			`/api/v1/tenants/${tenantId}/tsd/${year}/${month}`
		);
	}

	async downloadTSDXml(tenantId: string, year: number, month: number) {
		const headers: Record<string, string> = {};
		if (this.accessToken) {
			headers['Authorization'] = `Bearer ${this.accessToken}`;
		}

		const response = await fetch(
			`${getApiBase()}/api/v1/tenants/${tenantId}/tsd/${year}/${month}/xml`,
			{ headers }
		);

		if (!response.ok) {
			throw new Error('Failed to download TSD XML');
		}

		const blob = await response.blob();
		const url = window.URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `TSD_${year}_${String(month).padStart(2, '0')}.xml`;
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
		window.URL.revokeObjectURL(url);
	}

	async downloadTSDCsv(tenantId: string, year: number, month: number) {
		const headers: Record<string, string> = {};
		if (this.accessToken) {
			headers['Authorization'] = `Bearer ${this.accessToken}`;
		}

		const response = await fetch(
			`${getApiBase()}/api/v1/tenants/${tenantId}/tsd/${year}/${month}/csv`,
			{ headers }
		);

		if (!response.ok) {
			throw new Error('Failed to download TSD CSV');
		}

		const blob = await response.blob();
		const url = window.URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `TSD_${year}_${String(month).padStart(2, '0')}.csv`;
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
		window.URL.revokeObjectURL(url);
	}

	async markTSDSubmitted(tenantId: string, year: number, month: number, emtaReference: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/tsd/${year}/${month}/submit`,
			{ emta_reference: emtaReference }
		);
	}

	// Leave/Absence Management
	async listAbsenceTypes(tenantId: string, activeOnly = false) {
		const query = activeOnly ? '?active_only=true' : '';
		return this.request<AbsenceType[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/absence-types${query}`
		);
	}

	async getAbsenceType(tenantId: string, typeId: string) {
		return this.request<AbsenceType>(
			'GET',
			`/api/v1/tenants/${tenantId}/absence-types/${typeId}`
		);
	}

	async listLeaveBalances(tenantId: string, employeeId: string, year?: number) {
		const query = year ? `?year=${year}` : '';
		return this.request<LeaveBalance[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/employees/${employeeId}/leave-balances${query}`
		);
	}

	async getLeaveBalancesByYear(tenantId: string, employeeId: string, year: number) {
		return this.request<LeaveBalance[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/employees/${employeeId}/leave-balances/${year}`
		);
	}

	async updateLeaveBalance(
		tenantId: string,
		employeeId: string,
		year: number,
		typeId: string,
		data: UpdateLeaveBalanceRequest
	) {
		return this.request<LeaveBalance>(
			'PUT',
			`/api/v1/tenants/${tenantId}/employees/${employeeId}/leave-balances/${year}/${typeId}`,
			data
		);
	}

	async initializeLeaveBalances(tenantId: string, employeeId: string, year: number) {
		return this.request<LeaveBalance[]>(
			'POST',
			`/api/v1/tenants/${tenantId}/employees/${employeeId}/leave-balances/${year}/initialize`
		);
	}

	async listLeaveRecords(tenantId: string, employeeId?: string, year?: number) {
		const params = new URLSearchParams();
		if (employeeId) params.append('employee_id', employeeId);
		if (year) params.append('year', year.toString());
		const query = params.toString() ? `?${params.toString()}` : '';
		return this.request<LeaveRecord[]>(
			'GET',
			`/api/v1/tenants/${tenantId}/leave-records${query}`
		);
	}

	async createLeaveRecord(tenantId: string, data: CreateLeaveRecordRequest) {
		return this.request<LeaveRecord>(
			'POST',
			`/api/v1/tenants/${tenantId}/leave-records`,
			data
		);
	}

	async getLeaveRecord(tenantId: string, recordId: string) {
		return this.request<LeaveRecord>(
			'GET',
			`/api/v1/tenants/${tenantId}/leave-records/${recordId}`
		);
	}

	async approveLeaveRecord(tenantId: string, recordId: string) {
		return this.request<LeaveRecord>(
			'POST',
			`/api/v1/tenants/${tenantId}/leave-records/${recordId}/approve`
		);
	}

	async rejectLeaveRecord(tenantId: string, recordId: string, reason: string) {
		return this.request<LeaveRecord>(
			'POST',
			`/api/v1/tenants/${tenantId}/leave-records/${recordId}/reject`,
			{ reason }
		);
	}

	async cancelLeaveRecord(tenantId: string, recordId: string) {
		return this.request<LeaveRecord>(
			'POST',
			`/api/v1/tenants/${tenantId}/leave-records/${recordId}/cancel`
		);
	}

	// Plugin Registries (Admin)
	async listPluginRegistries() {
		return this.request<PluginRegistry[]>('GET', '/api/v1/admin/plugin-registries');
	}

	async addPluginRegistry(name: string, url: string, description?: string) {
		return this.request<PluginRegistry>('POST', '/api/v1/admin/plugin-registries', {
			name,
			url,
			description
		});
	}

	async removePluginRegistry(registryId: string) {
		return this.request<{ status: string }>(
			'DELETE',
			`/api/v1/admin/plugin-registries/${registryId}`
		);
	}

	async syncPluginRegistry(registryId: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/admin/plugin-registries/${registryId}/sync`
		);
	}

	// Plugins (Admin - Instance Level)
	async listPlugins() {
		return this.request<Plugin[]>('GET', '/api/v1/admin/plugins');
	}

	async searchPlugins(query: string) {
		return this.request<PluginSearchResult[]>(
			'GET',
			`/api/v1/admin/plugins/search?q=${encodeURIComponent(query)}`
		);
	}

	async getPluginPermissions() {
		return this.request<Record<string, PluginPermission>>('GET', '/api/v1/admin/plugins/permissions');
	}

	async installPlugin(repositoryUrl: string) {
		return this.request<Plugin>('POST', '/api/v1/admin/plugins/install', {
			repository_url: repositoryUrl
		});
	}

	async getPlugin(pluginId: string) {
		return this.request<Plugin>('GET', `/api/v1/admin/plugins/${pluginId}`);
	}

	async uninstallPlugin(pluginId: string) {
		return this.request<{ status: string }>('DELETE', `/api/v1/admin/plugins/${pluginId}`);
	}

	async enablePlugin(pluginId: string, permissions: string[]) {
		return this.request<Plugin>('POST', `/api/v1/admin/plugins/${pluginId}/enable`, {
			permissions
		});
	}

	async disablePlugin(pluginId: string) {
		return this.request<Plugin>('POST', `/api/v1/admin/plugins/${pluginId}/disable`);
	}

	// Tenant Plugin Management
	async listTenantPlugins(tenantId: string) {
		return this.request<TenantPlugin[]>('GET', `/api/v1/tenants/${tenantId}/plugins`);
	}

	async enableTenantPlugin(tenantId: string, pluginId: string, settings?: Record<string, unknown>) {
		return this.request<TenantPlugin>(
			'POST',
			`/api/v1/tenants/${tenantId}/plugins/${pluginId}/enable`,
			settings ? { settings } : undefined
		);
	}

	async disableTenantPlugin(tenantId: string, pluginId: string) {
		return this.request<{ status: string }>(
			'POST',
			`/api/v1/tenants/${tenantId}/plugins/${pluginId}/disable`
		);
	}

	async getTenantPluginSettings(tenantId: string, pluginId: string) {
		return this.request<TenantPluginSettings>(
			'GET',
			`/api/v1/tenants/${tenantId}/plugins/${pluginId}/settings`
		);
	}

	async updateTenantPluginSettings(tenantId: string, pluginId: string, settings: Record<string, unknown>) {
		return this.request<TenantPluginSettings>(
			'PUT',
			`/api/v1/tenants/${tenantId}/plugins/${pluginId}/settings`,
			{ settings }
		);
	}
}

// Types
export interface Tenant {
	id: string;
	name: string;
	slug: string;
	schema_name: string;
	settings: TenantSettings;
	is_active: boolean;
	onboarding_completed: boolean;
	created_at: string;
	updated_at: string;
}

export interface TenantSettings {
	default_currency: string;
	country_code: string;
	timezone: string;
	date_format: string;
	decimal_sep: string;
	thousands_sep: string;
	fiscal_year_start_month: number;
	vat_number?: string;
	reg_code?: string;
	address?: string;
	email?: string;
	phone?: string;
	logo?: string;
	pdf_primary_color?: string;
	pdf_footer_text?: string;
	bank_details?: string;
	invoice_terms?: string;
}

export interface TenantMembership {
	tenant: Tenant;
	role: string;
	is_default: boolean;
}

export interface Account {
	id: string;
	tenant_id: string;
	code: string;
	name: string;
	account_type: 'ASSET' | 'LIABILITY' | 'EQUITY' | 'REVENUE' | 'EXPENSE';
	parent_id?: string;
	is_active: boolean;
	is_system: boolean;
	description?: string;
	created_at: string;
}

export interface CreateAccountRequest {
	code: string;
	name: string;
	account_type: Account['account_type'];
	parent_id?: string;
	description?: string;
}

export interface JournalEntry {
	id: string;
	tenant_id: string;
	entry_number: string;
	entry_date: string;
	description: string;
	reference?: string;
	source_type?: string;
	source_id?: string;
	status: 'DRAFT' | 'POSTED' | 'VOIDED';
	lines: JournalEntryLine[];
	posted_at?: string;
	posted_by?: string;
	voided_at?: string;
	voided_by?: string;
	void_reason?: string;
	created_at: string;
	created_by: string;
}

export interface JournalEntryLine {
	id: string;
	account_id: string;
	account?: Account;
	description?: string;
	debit_amount: Decimal;
	credit_amount: Decimal;
	currency: string;
	exchange_rate: Decimal;
	base_debit: Decimal;
	base_credit: Decimal;
}

export interface CreateJournalEntryRequest {
	entry_date: string;
	description: string;
	reference?: string;
	source_type?: string;
	source_id?: string;
	lines: {
		account_id: string;
		description?: string;
		debit_amount: string;
		credit_amount: string;
		currency?: string;
		exchange_rate?: string;
	}[];
}

export interface TrialBalance {
	tenant_id: string;
	as_of_date: string;
	generated_at: string;
	accounts: AccountBalance[];
	total_debits: Decimal;
	total_credits: Decimal;
	is_balanced: boolean;
}

export interface AccountBalance {
	account_id: string;
	account_code: string;
	account_name: string;
	account_type: Account['account_type'];
	debit_balance: Decimal;
	credit_balance: Decimal;
	net_balance: Decimal;
}

export interface BalanceSheet {
	tenant_id: string;
	as_of_date: string;
	generated_at: string;
	assets: AccountBalance[];
	liabilities: AccountBalance[];
	equity: AccountBalance[];
	total_assets: Decimal;
	total_liabilities: Decimal;
	total_equity: Decimal;
	retained_earnings: Decimal;
	is_balanced: boolean;
}

export interface IncomeStatement {
	tenant_id: string;
	start_date: string;
	end_date: string;
	generated_at: string;
	revenue: AccountBalance[];
	expenses: AccountBalance[];
	total_revenue: Decimal;
	total_expenses: Decimal;
	net_income: Decimal;
}

// Contact types
export type ContactType = 'CUSTOMER' | 'SUPPLIER' | 'BOTH';

export interface Contact {
	id: string;
	tenant_id: string;
	code?: string;
	name: string;
	contact_type: ContactType;
	reg_code?: string;
	vat_number?: string;
	email?: string;
	phone?: string;
	address_line1?: string;
	address_line2?: string;
	city?: string;
	postal_code?: string;
	country_code: string;
	payment_terms_days: number;
	credit_limit?: Decimal;
	default_account_id?: string;
	is_active: boolean;
	notes?: string;
	created_at: string;
	updated_at: string;
}

export interface CreateContactRequest {
	code?: string;
	name: string;
	contact_type: ContactType;
	reg_code?: string;
	vat_number?: string;
	email?: string;
	phone?: string;
	address_line1?: string;
	address_line2?: string;
	city?: string;
	postal_code?: string;
	country_code?: string;
	payment_terms_days?: number;
	credit_limit?: string;
	default_account_id?: string;
	notes?: string;
}

export interface UpdateContactRequest {
	name?: string;
	reg_code?: string;
	vat_number?: string;
	email?: string;
	phone?: string;
	address_line1?: string;
	address_line2?: string;
	city?: string;
	postal_code?: string;
	country_code?: string;
	payment_terms_days?: number;
	credit_limit?: string;
	default_account_id?: string;
	notes?: string;
	is_active?: boolean;
}

export interface ContactFilter {
	type?: ContactType;
	active_only?: boolean;
	search?: string;
}

// Invoice types
export type InvoiceType = 'SALES' | 'PURCHASE' | 'CREDIT_NOTE';
export type InvoiceStatus = 'DRAFT' | 'SENT' | 'PARTIALLY_PAID' | 'PAID' | 'OVERDUE' | 'VOIDED';

export interface Invoice {
	id: string;
	tenant_id: string;
	invoice_number: string;
	invoice_type: InvoiceType;
	contact_id: string;
	contact?: Contact;
	issue_date: string;
	due_date: string;
	currency: string;
	exchange_rate: Decimal;
	subtotal: Decimal;
	vat_amount: Decimal;
	total: Decimal;
	base_subtotal: Decimal;
	base_vat_amount: Decimal;
	base_total: Decimal;
	amount_paid: Decimal;
	status: InvoiceStatus;
	reference?: string;
	notes?: string;
	lines: InvoiceLine[];
	journal_entry_id?: string;
	einvoice_sent_at?: string;
	einvoice_id?: string;
	created_at: string;
	created_by: string;
	updated_at: string;
}

export interface InvoiceLine {
	id: string;
	tenant_id: string;
	invoice_id: string;
	line_number: number;
	description: string;
	quantity: Decimal;
	unit?: string;
	unit_price: Decimal;
	discount_percent: Decimal;
	vat_rate: Decimal;
	line_subtotal: Decimal;
	line_vat: Decimal;
	line_total: Decimal;
	account_id?: string;
	product_id?: string;
}

export interface CreateInvoiceRequest {
	invoice_type: InvoiceType;
	contact_id: string;
	issue_date: string;
	due_date: string;
	currency?: string;
	exchange_rate?: string;
	reference?: string;
	notes?: string;
	lines: CreateInvoiceLineRequest[];
}

export interface CreateInvoiceLineRequest {
	description: string;
	quantity: string;
	unit?: string;
	unit_price: string;
	discount_percent?: string;
	vat_rate: string;
	account_id?: string;
	product_id?: string;
}

export interface InvoiceFilter {
	type?: InvoiceType;
	status?: InvoiceStatus;
	contact_id?: string;
	from_date?: string;
	to_date?: string;
	search?: string;
}

// Payment types
export type PaymentType = 'RECEIVED' | 'MADE';

export interface Payment {
	id: string;
	tenant_id: string;
	payment_number: string;
	payment_type: PaymentType;
	contact_id?: string;
	payment_date: string;
	amount: Decimal;
	currency: string;
	exchange_rate: Decimal;
	base_amount: Decimal;
	payment_method?: string;
	bank_account?: string;
	reference?: string;
	notes?: string;
	allocations: PaymentAllocation[];
	journal_entry_id?: string;
	created_at: string;
	created_by: string;
}

export interface PaymentAllocation {
	id: string;
	tenant_id: string;
	payment_id: string;
	invoice_id: string;
	amount: Decimal;
	created_at: string;
}

export interface CreatePaymentRequest {
	payment_type: PaymentType;
	contact_id?: string;
	payment_date: string;
	amount: string;
	currency?: string;
	exchange_rate?: string;
	payment_method?: string;
	bank_account?: string;
	reference?: string;
	notes?: string;
	allocations?: AllocationRequest[];
}

export interface AllocationRequest {
	invoice_id: string;
	amount: string;
}

export interface PaymentFilter {
	type?: PaymentType;
	method?: string;
	contact_id?: string;
	from_date?: string;
	to_date?: string;
}

// Quote types
export type QuoteStatus = 'DRAFT' | 'SENT' | 'ACCEPTED' | 'REJECTED' | 'EXPIRED' | 'CONVERTED';

export interface Quote {
	id: string;
	tenant_id: string;
	quote_number: string;
	contact_id: string;
	contact?: Contact;
	quote_date: string;
	valid_until?: string;
	status: QuoteStatus;
	currency: string;
	exchange_rate: Decimal;
	subtotal: Decimal;
	vat_amount: Decimal;
	total: Decimal;
	notes?: string;
	converted_to_order_id?: string;
	converted_to_invoice_id?: string;
	lines: QuoteLine[];
	created_at: string;
	created_by: string;
	updated_at: string;
}

export interface QuoteLine {
	id: string;
	tenant_id: string;
	quote_id: string;
	line_number: number;
	description: string;
	quantity: Decimal;
	unit?: string;
	unit_price: Decimal;
	discount_percent: Decimal;
	vat_rate: Decimal;
	line_subtotal: Decimal;
	line_vat: Decimal;
	line_total: Decimal;
	product_id?: string;
}

export interface CreateQuoteRequest {
	contact_id: string;
	quote_date: string;
	valid_until?: string;
	currency?: string;
	exchange_rate?: string;
	notes?: string;
	lines: CreateQuoteLineRequest[];
}

export interface CreateQuoteLineRequest {
	description: string;
	quantity: string;
	unit?: string;
	unit_price: string;
	discount_percent?: string;
	vat_rate: string;
	product_id?: string;
}

export interface UpdateQuoteRequest {
	contact_id: string;
	quote_date: string;
	valid_until?: string;
	currency?: string;
	exchange_rate?: string;
	notes?: string;
	lines: CreateQuoteLineRequest[];
}

export interface QuoteFilter {
	status?: QuoteStatus;
	contact_id?: string;
	from_date?: string;
	to_date?: string;
	search?: string;
}

// Order types
export type OrderStatus = 'PENDING' | 'CONFIRMED' | 'PROCESSING' | 'SHIPPED' | 'DELIVERED' | 'CANCELLED';

export interface Order {
	id: string;
	tenant_id: string;
	order_number: string;
	contact_id: string;
	contact?: Contact;
	order_date: string;
	expected_delivery?: string;
	status: OrderStatus;
	currency: string;
	exchange_rate: Decimal;
	subtotal: Decimal;
	vat_amount: Decimal;
	total: Decimal;
	notes?: string;
	quote_id?: string;
	converted_to_invoice_id?: string;
	lines: OrderLine[];
	created_at: string;
	created_by: string;
	updated_at: string;
}

export interface OrderLine {
	id: string;
	tenant_id: string;
	order_id: string;
	line_number: number;
	description: string;
	quantity: Decimal;
	unit?: string;
	unit_price: Decimal;
	discount_percent: Decimal;
	vat_rate: Decimal;
	line_subtotal: Decimal;
	line_vat: Decimal;
	line_total: Decimal;
	product_id?: string;
}

export interface CreateOrderRequest {
	contact_id: string;
	order_date: string;
	expected_delivery?: string;
	currency?: string;
	exchange_rate?: string;
	notes?: string;
	quote_id?: string;
	lines: CreateOrderLineRequest[];
}

export interface CreateOrderLineRequest {
	description: string;
	quantity: string;
	unit?: string;
	unit_price: string;
	discount_percent?: string;
	vat_rate: string;
	product_id?: string;
}

export interface UpdateOrderRequest {
	contact_id: string;
	order_date: string;
	expected_delivery?: string;
	currency?: string;
	exchange_rate?: string;
	notes?: string;
	lines: CreateOrderLineRequest[];
}

export interface OrderFilter {
	status?: OrderStatus;
	contact_id?: string;
	from_date?: string;
	to_date?: string;
	search?: string;
}

// Fixed Asset types
export type AssetStatus = 'DRAFT' | 'ACTIVE' | 'DISPOSED' | 'SOLD';
export type DepreciationMethod = 'STRAIGHT_LINE' | 'DECLINING_BALANCE' | 'UNITS_OF_PRODUCTION';
export type DisposalMethod = 'SOLD' | 'SCRAPPED' | 'DONATED' | 'LOST';

export interface AssetCategory {
	id: string;
	tenant_id: string;
	name: string;
	description?: string;
	depreciation_method: DepreciationMethod;
	default_useful_life_months: number;
	default_residual_value_percent: Decimal;
	asset_account_id?: string;
	depreciation_expense_account_id?: string;
	accumulated_depreciation_account_id?: string;
	created_at: string;
	updated_at: string;
}

export interface FixedAsset {
	id: string;
	tenant_id: string;
	asset_number: string;
	name: string;
	description?: string;
	category_id?: string;
	category?: AssetCategory;
	status: AssetStatus;
	purchase_date: string;
	purchase_cost: Decimal;
	supplier_id?: string;
	serial_number?: string;
	location?: string;
	depreciation_method: DepreciationMethod;
	useful_life_months: number;
	residual_value: Decimal;
	depreciation_start_date?: string;
	accumulated_depreciation: Decimal;
	book_value: Decimal;
	last_depreciation_date?: string;
	disposal_date?: string;
	disposal_method?: DisposalMethod;
	disposal_proceeds?: Decimal;
	disposal_notes?: string;
	asset_account_id?: string;
	depreciation_expense_account_id?: string;
	accumulated_depreciation_account_id?: string;
	created_at: string;
	created_by: string;
	updated_at: string;
}

export interface DepreciationEntry {
	id: string;
	tenant_id: string;
	asset_id: string;
	depreciation_date: string;
	period_start: string;
	period_end: string;
	depreciation_amount: Decimal;
	accumulated_total: Decimal;
	book_value_after: Decimal;
	journal_entry_id?: string;
	created_at: string;
	created_by: string;
}

export interface CreateAssetCategoryRequest {
	name: string;
	description?: string;
	depreciation_method?: DepreciationMethod;
	default_useful_life_months?: number;
	default_residual_value_percent?: string;
	asset_account_id?: string;
	depreciation_expense_account_id?: string;
	accumulated_depreciation_account_id?: string;
}

export interface CreateAssetRequest {
	name: string;
	description?: string;
	category_id?: string;
	purchase_date: string;
	purchase_cost: string;
	supplier_id?: string;
	serial_number?: string;
	location?: string;
	depreciation_method?: DepreciationMethod;
	useful_life_months?: number;
	residual_value?: string;
	depreciation_start_date?: string;
	asset_account_id?: string;
	depreciation_expense_account_id?: string;
	accumulated_depreciation_account_id?: string;
}

export interface UpdateAssetRequest {
	name: string;
	description?: string;
	category_id?: string;
	serial_number?: string;
	location?: string;
	depreciation_method?: DepreciationMethod;
	useful_life_months?: number;
	residual_value?: string;
	asset_account_id?: string;
	depreciation_expense_account_id?: string;
	accumulated_depreciation_account_id?: string;
}

export interface DisposeAssetRequest {
	disposal_date: string;
	disposal_method: DisposalMethod;
	disposal_proceeds?: string;
	disposal_notes?: string;
}

export interface AssetFilter {
	status?: AssetStatus;
	category_id?: string;
	from_date?: string;
	to_date?: string;
	search?: string;
}

export interface RecordDepreciationRequest {
	period_start: string;
	period_end: string;
}

// Inventory types
export type ProductType = 'GOODS' | 'SERVICE';
export type ProductStatus = 'ACTIVE' | 'INACTIVE';
export type MovementType = 'IN' | 'OUT' | 'ADJUSTMENT' | 'TRANSFER';

export interface Product {
	id: string;
	tenant_id: string;
	code: string;
	name: string;
	description?: string;
	product_type: ProductType;
	category_id?: string;
	unit?: string;
	purchase_price: Decimal;
	sales_price: Decimal;
	vat_rate: Decimal;
	min_stock_level: Decimal;
	current_stock: Decimal;
	reorder_point: Decimal;
	sale_account_id?: string;
	purchase_account_id?: string;
	inventory_account_id?: string;
	track_inventory: boolean;
	is_active: boolean;
	barcode?: string;
	supplier_id?: string;
	lead_time_days: number;
	created_at: string;
	updated_at: string;
}

export interface ProductCategory {
	id: string;
	tenant_id: string;
	name: string;
	description?: string;
	parent_id?: string;
	created_at: string;
	updated_at: string;
}

export interface Warehouse {
	id: string;
	tenant_id: string;
	code: string;
	name: string;
	address?: string;
	is_default: boolean;
	is_active: boolean;
	created_at: string;
	updated_at: string;
}

export interface StockLevel {
	id: string;
	tenant_id: string;
	product_id: string;
	warehouse_id: string;
	quantity: Decimal;
	reserved_qty: Decimal;
	available_qty: Decimal;
	last_updated: string;
}

export interface InventoryMovement {
	id: string;
	tenant_id: string;
	product_id: string;
	warehouse_id: string;
	movement_type: MovementType;
	quantity: Decimal;
	unit_cost: Decimal;
	total_cost: Decimal;
	reference?: string;
	source_type?: string;
	source_id?: string;
	to_warehouse_id?: string;
	notes?: string;
	movement_date: string;
	created_at: string;
	created_by: string;
}

export interface CreateProductRequest {
	code?: string;
	name: string;
	description?: string;
	product_type: string;
	category_id?: string;
	unit?: string;
	purchase_price?: string;
	sales_price: string;
	vat_rate?: string;
	min_stock_level?: string;
	reorder_point?: string;
	sale_account_id?: string;
	purchase_account_id?: string;
	inventory_account_id?: string;
	track_inventory?: boolean;
	barcode?: string;
	supplier_id?: string;
	lead_time_days?: number;
}

export interface UpdateProductRequest {
	name: string;
	description?: string;
	category_id?: string;
	unit?: string;
	purchase_price?: string;
	sales_price: string;
	vat_rate?: string;
	min_stock_level?: string;
	reorder_point?: string;
	sale_account_id?: string;
	purchase_account_id?: string;
	inventory_account_id?: string;
	track_inventory?: boolean;
	is_active?: boolean;
	barcode?: string;
	supplier_id?: string;
	lead_time_days?: number;
}

export interface CreateProductCategoryRequest {
	name: string;
	description?: string;
	parent_id?: string;
}

export interface CreateWarehouseRequest {
	code: string;
	name: string;
	address?: string;
	is_default?: boolean;
}

export interface UpdateWarehouseRequest {
	name: string;
	address?: string;
	is_default?: boolean;
	is_active?: boolean;
}

export interface AdjustStockRequest {
	product_id: string;
	warehouse_id: string;
	quantity: string;
	unit_cost?: string;
	reason?: string;
}

export interface TransferStockRequest {
	product_id: string;
	from_warehouse_id: string;
	to_warehouse_id: string;
	quantity: string;
	notes?: string;
}

export interface ProductFilter {
	product_type?: ProductType;
	status?: ProductStatus;
	category_id?: string;
	search?: string;
	low_stock?: boolean;
}

// Analytics types
export interface DashboardSummary {
	total_revenue: Decimal;
	total_expenses: Decimal;
	net_income: Decimal;
	revenue_change: Decimal;
	expenses_change: Decimal;
	total_receivables: Decimal;
	total_payables: Decimal;
	overdue_receivables: Decimal;
	overdue_payables: Decimal;
	draft_invoices: number;
	pending_invoices: number;
	overdue_invoices: number;
	period_start: string;
	period_end: string;
}

export interface RevenueExpenseChart {
	labels: string[];
	revenue: Decimal[];
	expenses: Decimal[];
	profit: Decimal[];
}

export interface CashFlowChart {
	labels: string[];
	inflows: Decimal[];
	outflows: Decimal[];
	net: Decimal[];
}

export interface AgingBucket {
	label: string;
	amount: Decimal;
	count: number;
}

export interface AgingReport {
	report_type: string;
	as_of_date: string;
	total: Decimal;
	buckets: AgingBucket[];
}

export type ActivityType = 'INVOICE' | 'PAYMENT' | 'ENTRY' | 'CONTACT';

export interface ActivityItem {
	id: string;
	type: ActivityType;
	action: string;
	description: string;
	amount?: string;
	created_at: string;
}

// Recurring Invoice types
export type Frequency = 'WEEKLY' | 'BIWEEKLY' | 'MONTHLY' | 'QUARTERLY' | 'YEARLY';

export interface RecurringInvoice {
	id: string;
	tenant_id: string;
	name: string;
	contact_id: string;
	contact_name?: string;
	invoice_type: string;
	currency: string;
	frequency: Frequency;
	start_date: string;
	end_date?: string;
	next_generation_date: string;
	payment_terms_days: number;
	reference?: string;
	notes?: string;
	is_active: boolean;
	last_generated_at?: string;
	generated_count: number;
	lines: RecurringInvoiceLine[];
	created_at: string;
	created_by: string;
	updated_at: string;
	// Email configuration
	send_email_on_generation: boolean;
	email_template_type?: string;
	recipient_email_override?: string;
	attach_pdf_to_email: boolean;
	email_subject_override?: string;
	email_message?: string;
}

export interface RecurringInvoiceLine {
	id: string;
	recurring_invoice_id: string;
	line_number: number;
	description: string;
	quantity: Decimal;
	unit?: string;
	unit_price: Decimal;
	discount_percent: Decimal;
	vat_rate: Decimal;
	account_id?: string;
	product_id?: string;
}

export interface CreateRecurringInvoiceRequest {
	name: string;
	contact_id: string;
	invoice_type?: string;
	currency?: string;
	frequency: Frequency;
	start_date: string;
	end_date?: string;
	payment_terms_days?: number;
	reference?: string;
	notes?: string;
	lines: CreateRecurringInvoiceLineRequest[];
	// Email configuration
	send_email_on_generation?: boolean;
	email_template_type?: string;
	recipient_email_override?: string;
	attach_pdf_to_email?: boolean;
	email_subject_override?: string;
	email_message?: string;
}

export interface CreateRecurringInvoiceLineRequest {
	description: string;
	quantity: string;
	unit?: string;
	unit_price: string;
	discount_percent?: string;
	vat_rate: string;
	account_id?: string;
	product_id?: string;
}

export interface UpdateRecurringInvoiceRequest {
	name?: string;
	contact_id?: string;
	frequency?: Frequency;
	end_date?: string;
	payment_terms_days?: number;
	reference?: string;
	notes?: string;
	lines?: CreateRecurringInvoiceLineRequest[];
	// Email configuration
	send_email_on_generation?: boolean;
	email_template_type?: string;
	recipient_email_override?: string;
	attach_pdf_to_email?: boolean;
	email_subject_override?: string;
	email_message?: string;
}

export interface CreateFromInvoiceRequest {
	name: string;
	frequency: Frequency;
	start_date: string;
	end_date?: string;
	payment_terms_days?: number;
}

export interface GenerationResult {
	recurring_invoice_id: string;
	generated_invoice_id: string;
	generated_invoice_number: string;
	// Email delivery status
	email_sent: boolean;
	email_status?: string;
	email_log_id?: string;
	email_error?: string;
}

// Email types
export type TemplateType = 'INVOICE_SEND' | 'PAYMENT_RECEIPT' | 'OVERDUE_REMINDER';
export type EmailStatus = 'PENDING' | 'SENT' | 'FAILED';

export interface SMTPConfig {
	smtp_host: string;
	smtp_port: number;
	smtp_username: string;
	smtp_password?: string;
	smtp_from_email: string;
	smtp_from_name: string;
	smtp_use_tls: boolean;
}

export interface UpdateSMTPConfigRequest {
	smtp_host: string;
	smtp_port: number;
	smtp_username: string;
	smtp_password?: string;
	smtp_from_email: string;
	smtp_from_name: string;
	smtp_use_tls: boolean;
}

export interface TestSMTPResponse {
	success: boolean;
	message: string;
}

export interface EmailTemplate {
	id: string;
	tenant_id: string;
	template_type: TemplateType;
	subject: string;
	body_html: string;
	body_text?: string;
	is_active: boolean;
	created_at: string;
	updated_at: string;
}

export interface UpdateTemplateRequest {
	subject: string;
	body_html: string;
	body_text?: string;
	is_active: boolean;
}

export interface EmailLog {
	id: string;
	tenant_id: string;
	email_type: string;
	recipient_email: string;
	recipient_name?: string;
	subject: string;
	status: EmailStatus;
	sent_at?: string;
	error_message?: string;
	related_id?: string;
	created_at: string;
}

export interface SendInvoiceEmailRequest {
	recipient_email: string;
	recipient_name?: string;
	subject?: string;
	message?: string;
	attach_pdf: boolean;
}

export interface SendPaymentReceiptRequest {
	recipient_email: string;
	recipient_name?: string;
	subject?: string;
	message?: string;
}

export interface EmailSentResponse {
	success: boolean;
	log_id: string;
	message: string;
}

// Banking types
export type TransactionStatus = 'UNMATCHED' | 'MATCHED' | 'RECONCILED';
export type ReconciliationStatus = 'IN_PROGRESS' | 'COMPLETED';

export interface BankAccount {
	id: string;
	tenant_id: string;
	name: string;
	account_number: string;
	bank_name?: string;
	currency: string;
	opening_balance: Decimal;
	current_balance: Decimal;
	gl_account_id?: string;
	is_active: boolean;
	created_at: string;
	updated_at: string;
}

export interface BankTransaction {
	id: string;
	tenant_id: string;
	bank_account_id: string;
	transaction_date: string;
	value_date?: string;
	description: string;
	reference?: string;
	amount: Decimal;
	currency: string;
	counterparty_name?: string;
	counterparty_account?: string;
	status: TransactionStatus;
	matched_payment_id?: string;
	reconciliation_id?: string;
	import_id?: string;
	created_at: string;
}

export interface BankReconciliation {
	id: string;
	tenant_id: string;
	bank_account_id: string;
	statement_date: string;
	opening_balance: Decimal;
	closing_balance: Decimal;
	calculated_balance?: Decimal;
	difference?: Decimal;
	status: ReconciliationStatus;
	completed_at?: string;
	completed_by?: string;
	transactions_matched: number;
	transactions_unmatched: number;
	created_at: string;
	created_by: string;
}

export interface BankStatementImport {
	id: string;
	tenant_id: string;
	bank_account_id: string;
	file_name: string;
	transactions_imported: number;
	transactions_matched: number;
	transactions_duplicates: number;
	created_at: string;
	created_by: string;
}

export interface MatchSuggestion {
	payment_id: string;
	payment_number: string;
	payment_date: string;
	amount: Decimal;
	contact_name?: string;
	reference?: string;
	confidence: number;
	match_reason: string;
}

export interface CreateBankAccountRequest {
	name: string;
	account_number: string;
	bank_name?: string;
	currency?: string;
	opening_balance?: string;
	gl_account_id?: string;
}

export interface UpdateBankAccountRequest {
	name?: string;
	bank_name?: string;
	gl_account_id?: string;
	is_active?: boolean;
}

export interface ImportTransactionsRequest {
	csv_content: string;
	file_name: string;
	mapping: CSVColumnMapping;
	skip_duplicates?: boolean;
}

export interface CSVColumnMapping {
	date_column: number;
	description_column: number;
	amount_column: number;
	reference_column?: number;
	counterparty_column?: number;
	date_format: string;
	decimal_separator: string;
	thousands_separator?: string;
	skip_header: boolean;
}

export interface ImportResult {
	import_id: string;
	transactions_imported: number;
	transactions_duplicates: number;
	errors: string[];
}

export interface CreateReconciliationRequest {
	statement_date: string;
	opening_balance: string;
	closing_balance: string;
}

// Tax (KMD) types
export interface KMDRow {
	code: string;
	description: string;
	tax_base: Decimal;
	tax_amount: Decimal;
}

export interface KMDDeclaration {
	id: string;
	tenant_id: string;
	year: number;
	month: number;
	status: 'DRAFT' | 'SUBMITTED' | 'ACCEPTED';
	total_output_vat: Decimal;
	total_input_vat: Decimal;
	rows: KMDRow[];
	submitted_at?: string;
	created_at: string;
	updated_at: string;
}

export interface CreateKMDRequest {
	year: number;
	month: number;
}

// Cash Flow types
export interface CashFlowItem {
	code: string;
	description: string;
	description_et: string;
	amount: string;
	is_subtotal: boolean;
}

export interface CashFlowStatement {
	tenant_id: string;
	start_date: string;
	end_date: string;
	operating_activities: CashFlowItem[];
	investing_activities: CashFlowItem[];
	financing_activities: CashFlowItem[];
	total_operating: string;
	total_investing: string;
	total_financing: string;
	net_cash_change: string;
	opening_cash: string;
	closing_cash: string;
	generated_at: string;
}

// Balance Confirmation types
export type BalanceConfirmationType = 'RECEIVABLE' | 'PAYABLE';

export interface BalanceInvoice {
	invoice_id: string;
	invoice_number: string;
	invoice_date: string;
	due_date: string;
	total_amount: string;
	amount_paid: string;
	outstanding_amount: string;
	currency: string;
	days_overdue: number;
}

export interface ContactBalance {
	contact_id: string;
	contact_name: string;
	contact_code?: string;
	contact_email?: string;
	balance: string;
	invoice_count: number;
	oldest_invoice?: string;
}

export interface BalanceConfirmationSummary {
	type: BalanceConfirmationType;
	as_of_date: string;
	total_balance: string;
	contact_count: number;
	invoice_count: number;
	contacts: ContactBalance[];
	generated_at: string;
}

export interface BalanceConfirmation {
	id: string;
	tenant_id: string;
	contact_id: string;
	contact_name: string;
	contact_code?: string;
	contact_email?: string;
	type: BalanceConfirmationType;
	as_of_date: string;
	total_balance: string;
	invoices: BalanceInvoice[];
	generated_at: string;
}

// Payment Reminder types
export type ReminderStatus = 'PENDING' | 'SENT' | 'FAILED' | 'CANCELLED';

export interface PaymentReminder {
	id: string;
	tenant_id: string;
	invoice_id: string;
	invoice_number: string;
	contact_id: string;
	contact_name: string;
	contact_email: string;
	reminder_number: number;
	status: ReminderStatus;
	sent_at?: string;
	error_message?: string;
	created_at: string;
	updated_at: string;
}

export interface OverdueInvoice {
	id: string;
	invoice_number: string;
	contact_id: string;
	contact_name: string;
	contact_email?: string;
	issue_date: string;
	due_date: string;
	total: string;
	amount_paid: string;
	outstanding_amount: string;
	currency: string;
	days_overdue: number;
	reminder_count: number;
	last_reminder_at?: string;
}

export interface OverdueInvoicesSummary {
	total_overdue: string;
	invoice_count: number;
	contact_count: number;
	average_days_overdue: number;
	invoices: OverdueInvoice[];
	generated_at: string;
}

export interface SendReminderRequest {
	invoice_id: string;
	message?: string;
}

export interface SendBulkRemindersRequest {
	invoice_ids: string[];
	message?: string;
}

export interface ReminderResult {
	invoice_id: string;
	invoice_number: string;
	success: boolean;
	message: string;
	reminder_id?: string;
}

export interface BulkReminderResult {
	total_requested: number;
	successful: number;
	failed: number;
	results: ReminderResult[];
}

// Payroll types
export type EmploymentType = 'FULL_TIME' | 'PART_TIME' | 'CONTRACT';
export type PayrollStatus = 'DRAFT' | 'CALCULATED' | 'APPROVED' | 'PAID' | 'DECLARED';
export type TSDStatus = 'DRAFT' | 'SUBMITTED' | 'ACCEPTED' | 'REJECTED';

export interface Employee {
	id: string;
	tenant_id: string;
	employee_number?: string;
	first_name: string;
	last_name: string;
	personal_code?: string;
	email?: string;
	phone?: string;
	address?: string;
	bank_account?: string;
	start_date: string;
	end_date?: string;
	position?: string;
	department?: string;
	employment_type: EmploymentType;
	tax_residency: string;
	apply_basic_exemption: boolean;
	basic_exemption_amount: Decimal;
	funded_pension_rate: Decimal;
	is_active: boolean;
	created_at: string;
	updated_at: string;
}

export interface CreateEmployeeRequest {
	employee_number?: string;
	first_name: string;
	last_name: string;
	personal_code?: string;
	email?: string;
	phone?: string;
	address?: string;
	bank_account?: string;
	start_date: string;
	position?: string;
	department?: string;
	employment_type: EmploymentType;
	apply_basic_exemption: boolean;
	basic_exemption_amount?: string;
	funded_pension_rate?: string;
}

export interface UpdateEmployeeRequest {
	employee_number?: string;
	first_name?: string;
	last_name?: string;
	personal_code?: string;
	email?: string;
	phone?: string;
	address?: string;
	bank_account?: string;
	end_date?: string;
	position?: string;
	department?: string;
	employment_type?: EmploymentType;
	apply_basic_exemption?: boolean;
	basic_exemption_amount?: string;
	funded_pension_rate?: string;
	is_active?: boolean;
}

export interface SalaryComponent {
	id: string;
	tenant_id: string;
	employee_id: string;
	component_type: string;
	name: string;
	amount: Decimal;
	is_taxable: boolean;
	is_recurring: boolean;
	effective_from: string;
	effective_to?: string;
	created_at: string;
}

export interface PayrollRun {
	id: string;
	tenant_id: string;
	period_year: number;
	period_month: number;
	status: PayrollStatus;
	payment_date?: string;
	total_gross: Decimal;
	total_net: Decimal;
	total_employer_cost: Decimal;
	notes?: string;
	created_by?: string;
	approved_by?: string;
	approved_at?: string;
	created_at: string;
	updated_at: string;
	payslips?: Payslip[];
}

export interface CreatePayrollRunRequest {
	period_year: number;
	period_month: number;
	payment_date?: string;
	notes?: string;
}

export interface Payslip {
	id: string;
	tenant_id: string;
	payroll_run_id: string;
	employee_id: string;
	gross_salary: Decimal;
	taxable_income: Decimal;
	income_tax: Decimal;
	unemployment_insurance_employee: Decimal;
	funded_pension: Decimal;
	other_deductions: Decimal;
	net_salary: Decimal;
	social_tax: Decimal;
	unemployment_insurance_employer: Decimal;
	total_employer_cost: Decimal;
	basic_exemption_applied: Decimal;
	payment_status: string;
	paid_at?: string;
	created_at: string;
	employee?: Employee;
}

export interface TSDDeclaration {
	id: string;
	tenant_id: string;
	period_year: number;
	period_month: number;
	payroll_run_id?: string;
	total_payments: Decimal;
	total_income_tax: Decimal;
	total_social_tax: Decimal;
	total_unemployment_employer: Decimal;
	total_unemployment_employee: Decimal;
	total_funded_pension: Decimal;
	status: TSDStatus;
	submitted_at?: string;
	emta_reference?: string;
	created_at: string;
	updated_at: string;
	rows?: TSDRow[];
}

export interface TSDRow {
	id: string;
	tenant_id: string;
	declaration_id: string;
	employee_id: string;
	personal_code: string;
	first_name: string;
	last_name: string;
	payment_type: string;
	gross_payment: Decimal;
	basic_exemption: Decimal;
	taxable_amount: Decimal;
	income_tax: Decimal;
	social_tax: Decimal;
	unemployment_insurance_employer: Decimal;
	unemployment_insurance_employee: Decimal;
	funded_pension: Decimal;
	created_at: string;
}

export interface TaxCalculation {
	gross_salary: Decimal;
	basic_exemption: Decimal;
	taxable_income: Decimal;
	income_tax: Decimal;
	unemployment_employee: Decimal;
	funded_pension: Decimal;
	total_deductions: Decimal;
	net_salary: Decimal;
	social_tax: Decimal;
	unemployment_employer: Decimal;
	total_employer_cost: Decimal;
}

// Leave/Absence Management Types
export type LeaveStatus = 'PENDING' | 'APPROVED' | 'REJECTED' | 'CANCELLED';

export interface AbsenceType {
	id: string;
	tenant_id: string;
	code: string;
	name: string;
	name_et: string;
	description?: string;
	is_paid: boolean;
	affects_salary: boolean;
	requires_document: boolean;
	document_type?: string;
	default_days_per_year: Decimal;
	max_carryover_days: Decimal;
	tsd_code?: string;
	emta_code?: string;
	is_system: boolean;
	is_active: boolean;
	sort_order: number;
	created_at: string;
	updated_at: string;
}

export interface LeaveBalance {
	id: string;
	tenant_id: string;
	employee_id: string;
	absence_type_id: string;
	year: number;
	entitled_days: Decimal;
	carryover_days: Decimal;
	used_days: Decimal;
	pending_days: Decimal;
	remaining_days: Decimal;
	notes?: string;
	created_at: string;
	updated_at: string;
	absence_type?: AbsenceType;
}

export interface LeaveRecord {
	id: string;
	tenant_id: string;
	employee_id: string;
	absence_type_id: string;
	start_date: string;
	end_date: string;
	total_days: Decimal;
	working_days: Decimal;
	status: LeaveStatus;
	document_number?: string;
	document_date?: string;
	document_url?: string;
	requested_at: string;
	requested_by?: string;
	approved_at?: string;
	approved_by?: string;
	rejected_at?: string;
	rejected_by?: string;
	rejection_reason?: string;
	payroll_run_id?: string;
	notes?: string;
	created_at: string;
	updated_at: string;
	absence_type?: AbsenceType;
	employee?: Employee;
}

export interface CreateLeaveRecordRequest {
	employee_id: string;
	absence_type_id: string;
	start_date: string;
	end_date: string;
	total_days: Decimal;
	working_days: Decimal;
	document_number?: string;
	document_date?: string;
	notes?: string;
}

export interface UpdateLeaveBalanceRequest {
	entitled_days?: Decimal;
	carryover_days?: Decimal;
	notes?: string;
}

export interface RejectLeaveRequest {
	reason: string;
}

// Plugin Types
export type PluginState = 'installed' | 'enabled' | 'disabled' | 'failed';
export type PermissionRisk = 'low' | 'medium' | 'high' | 'critical';
export type PermissionCategory = 'data' | 'system' | 'database' | 'dangerous';
export type RepositoryType = 'github' | 'gitlab';

export interface PluginRegistry {
	id: string;
	name: string;
	url: string;
	description?: string;
	is_official: boolean;
	is_active: boolean;
	last_synced_at?: string;
	created_at: string;
	updated_at: string;
}

export interface Plugin {
	id: string;
	name: string;
	display_name: string;
	description?: string;
	version: string;
	repository_url: string;
	repository_type: RepositoryType;
	author?: string;
	license?: string;
	homepage_url?: string;
	state: PluginState;
	granted_permissions: string[];
	manifest: PluginManifest;
	installed_at: string;
	updated_at: string;
}

export interface PluginManifest {
	name: string;
	display_name: string;
	version: string;
	description?: string;
	author?: string;
	license?: string;
	homepage?: string;
	min_app_version?: string;
	permissions: string[];
	backend?: PluginBackendConfig;
	frontend?: PluginFrontendConfig;
	database?: PluginDatabaseConfig;
	settings?: Record<string, unknown>;
}

export interface PluginBackendConfig {
	package?: string;
	entry?: string;
	hooks?: PluginHook[];
	routes?: PluginRoute[];
}

export interface PluginHook {
	event: string;
	handler: string;
}

export interface PluginRoute {
	method: string;
	path: string;
	handler: string;
}

export interface PluginFrontendConfig {
	components?: string;
	navigation?: PluginNavItem[];
	slots?: PluginSlot[];
}

export interface PluginNavItem {
	label: string;
	icon?: string;
	path: string;
	position?: string;
}

export interface PluginSlot {
	name: string;
	component: string;
}

export interface PluginDatabaseConfig {
	migrations?: string;
}

export interface PluginPermission {
	name: string;
	category: PermissionCategory;
	risk: PermissionRisk;
	description: string;
}

export interface TenantPlugin {
	id: string;
	tenant_id: string;
	plugin_id: string;
	is_enabled: boolean;
	settings: Record<string, unknown>;
	enabled_at?: string;
	created_at: string;
	updated_at: string;
	plugin?: Plugin;
}

export interface TenantPluginSettings {
	plugin_id: string;
	settings: Record<string, unknown>;
	schema?: Record<string, unknown>;
}

export interface PluginSearchResult {
	plugin: PluginInfo;
	registry: string;
}

export interface PluginInfo {
	name: string;
	display_name: string;
	description?: string;
	repository: string;
	version: string;
	author?: string;
	license?: string;
	tags?: string[];
}

// Cost Center Types
export type BudgetPeriod = 'MONTHLY' | 'QUARTERLY' | 'ANNUAL';

export interface CostCenter {
	id: string;
	tenant_id: string;
	code: string;
	name: string;
	description?: string;
	parent_id?: string;
	is_active: boolean;
	budget_amount?: string;
	budget_period: BudgetPeriod;
	created_at: string;
	updated_at: string;
	children?: CostCenter[];
	total_spent?: string;
	budget_used_percentage?: string;
}

export interface CreateCostCenterRequest {
	code: string;
	name: string;
	description?: string;
	parent_id?: string;
	is_active: boolean;
	budget_amount?: string;
	budget_period?: BudgetPeriod;
}

export interface UpdateCostCenterRequest {
	code: string;
	name: string;
	description?: string;
	parent_id?: string;
	is_active: boolean;
	budget_amount?: string;
	budget_period?: BudgetPeriod;
}

export interface CostCenterSummary {
	cost_center: CostCenter;
	total_expenses: string;
	budget_amount: string;
	budget_used_percentage: string;
	is_over_budget: boolean;
	period_start: string;
	period_end: string;
}

export interface CostCenterReport {
	tenant_id: string;
	period_start: string;
	period_end: string;
	generated_at: string;
	cost_centers: CostCenterSummary[];
	total_expenses: string;
	total_budget: string;
}

export const api = new ApiClient();
