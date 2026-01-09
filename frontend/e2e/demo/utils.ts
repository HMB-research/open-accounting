import { Page, expect, TestInfo } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

// Use environment variables for local testing, fall back to Railway for remote demo testing
export const DEMO_URL = process.env.BASE_URL || 'https://open-accounting.up.railway.app';
export const DEMO_API_URL = process.env.PUBLIC_API_URL || 'https://open-accounting-api.up.railway.app';

// Auth state directory - matches playwright.demo.config.ts
const AUTH_DIR = path.join(__dirname, '..', '..', '.auth');

// Demo user reserved for end users (README documentation)
// This user should NOT be used by automated tests to avoid conflicts
export const END_USER_DEMO = { email: 'demo1@example.com', password: 'demo12345', tenantSlug: 'demo1', tenantName: 'Demo Company 1', tenantId: 'b0000000-0000-0000-0001-000000000001' };

// Demo credentials for E2E tests (demo1-demo4)
// All 4 users are available in CI since it's isolated from public demo
// NOTE: demo1 may be in use by end users on live Railway demo - tests handle this gracefully
export const DEMO_CREDENTIALS = [
	{ email: 'demo1@example.com', password: 'demo12345', tenantSlug: 'demo1', tenantName: 'Demo Company 1', tenantId: 'b0000000-0000-0000-0001-000000000001' },
	{ email: 'demo2@example.com', password: 'demo12345', tenantSlug: 'demo2', tenantName: 'Demo Company 2', tenantId: 'b0000000-0000-0000-0002-000000000001' },
	{ email: 'demo3@example.com', password: 'demo12345', tenantSlug: 'demo3', tenantName: 'Demo Company 3', tenantId: 'b0000000-0000-0000-0003-000000000001' },
	{ email: 'demo4@example.com', password: 'demo12345', tenantSlug: 'demo4', tenantName: 'Demo Company 4', tenantId: 'b0000000-0000-0000-0004-000000000001' }
] as const;

/**
 * Get demo credentials for the current worker
 * @param testInfo - Playwright TestInfo object containing parallelIndex
 */
export function getDemoCredentials(testInfo: TestInfo) {
	const workerIndex = testInfo.parallelIndex % DEMO_CREDENTIALS.length;
	return DEMO_CREDENTIALS[workerIndex];
}

/**
 * Login as the demo user assigned to this worker
 */
export async function loginAsDemo(page: Page, testInfo: TestInfo): Promise<void> {
	const creds = getDemoCredentials(testInfo);
	const startTime = Date.now();

	// Navigate to login page
	await page.goto(`${DEMO_URL}/login`);
	await page.waitForLoadState('networkidle');

	// Wait for form elements to be ready
	await page.waitForSelector('input[type="email"], input[name="email"]', { timeout: 10000 });

	// Fill credentials
	const emailInput = page.locator('input[type="email"], input[name="email"]').first();
	const passwordInput = page.locator('input[type="password"]').first();
	await emailInput.fill(creds.email);
	await passwordInput.fill(creds.password);

	// Click sign in and wait for navigation
	// Support both English "Sign In" and Estonian "Logi sisse"
	const signInButton = page.getByRole('button', { name: /sign in|login|logi sisse/i });
	await signInButton.click();

	// Wait for navigation with better error handling
	try {
		await page.waitForURL(/dashboard/, { timeout: 30000 });
	} catch (error) {
		// Check if we're still on login page with an error
		const errorAlert = page.locator('.alert-error, [role="alert"]');
		const hasError = await errorAlert.isVisible().catch(() => false);
		if (hasError) {
			const errorText = await errorAlert.textContent().catch(() => 'Unknown error');
			throw new Error(`Login failed for ${creds.email}: ${errorText}`);
		}

		// Check current URL for debugging
		const currentUrl = page.url();
		throw new Error(`Login navigation timeout for ${creds.email}. Current URL: ${currentUrl}`);
	}

	// Use domcontentloaded instead of networkidle (more reliable for SPA)
	await page.waitForLoadState('domcontentloaded');
	// Wait for dashboard content to appear
	await page.waitForSelector('h1, .dashboard-header, [data-testid="dashboard"]', { timeout: 10000 }).catch(() => {
		// Dashboard loaded even if selector not found
	});
	console.log(`Login completed in ${Date.now() - startTime}ms for ${creds.email}`);
}

/**
 * Load auth state from file and apply to browser context.
 * Returns true if auth was loaded successfully, false otherwise.
 */
async function loadAuthState(page: Page, workerIndex: number): Promise<boolean> {
	const authFile = path.join(AUTH_DIR, `worker-${workerIndex}.json`);

	try {
		if (!fs.existsSync(authFile)) {
			console.log(`[Worker ${workerIndex}] Auth file not found: ${authFile}`);
			return false;
		}

		const authData = JSON.parse(fs.readFileSync(authFile, 'utf-8'));

		// Add cookies to the browser context
		if (authData.cookies && authData.cookies.length > 0) {
			await page.context().addCookies(authData.cookies);
			console.log(`[Worker ${workerIndex}] Loaded ${authData.cookies.length} cookies from auth file`);
		}

		// Add localStorage items
		if (authData.origins && authData.origins.length > 0) {
			for (const origin of authData.origins) {
				if (origin.localStorage && origin.localStorage.length > 0) {
					await page.goto(origin.origin, { waitUntil: 'domcontentloaded' });
					for (const item of origin.localStorage) {
						await page.evaluate(
							({ key, value }) => localStorage.setItem(key, value),
							{ key: item.name, value: item.value }
						);
					}
					console.log(`[Worker ${workerIndex}] Loaded ${origin.localStorage.length} localStorage items`);
				}
			}
		}

		return true;
	} catch (error) {
		console.log(`[Worker ${workerIndex}] Failed to load auth state: ${error}`);
		return false;
	}
}

/**
 * Ensure authentication - try to load saved auth state, fall back to login.
 * This is the preferred way to authenticate in tests for better performance.
 */
export async function ensureAuthenticated(page: Page, testInfo: TestInfo): Promise<void> {
	const workerIndex = testInfo.parallelIndex % DEMO_CREDENTIALS.length;
	const creds = getDemoCredentials(testInfo);
	const startTime = Date.now();

	// Try to load saved auth state
	const authLoaded = await loadAuthState(page, workerIndex);

	if (authLoaded) {
		// Navigate to dashboard to verify auth works
		await page.goto(`${DEMO_URL}/dashboard`);
		await page.waitForLoadState('domcontentloaded');

		// Check if we're on dashboard (auth valid) or redirected to login (auth invalid)
		const currentUrl = page.url();
		if (!currentUrl.includes('/login')) {
			console.log(`[Worker ${workerIndex}] Session reuse successful in ${Date.now() - startTime}ms`);
			return;
		}
		console.log(`[Worker ${workerIndex}] Saved auth invalid, performing fresh login`);
	}

	// Fall back to full login
	await loginAsDemo(page, testInfo);
}

export async function navigateTo(page: Page, path: string, testInfo?: TestInfo): Promise<void> {
	let url = `${DEMO_URL}${path}`;
	// Append tenant ID if testInfo is provided and path doesn't already have query params
	if (testInfo) {
		const creds = getDemoCredentials(testInfo);
		const separator = path.includes('?') ? '&' : '?';
		url = `${url}${separator}tenant=${creds.tenantId}`;
	}
	await page.goto(url);
	// Wait for DOM to be ready (removed fixed 500ms wait)
	await page.waitForLoadState('domcontentloaded');

	// Wait for any loading overlays to disappear instead of fixed wait
	const loadingIndicator = page.locator('.loading, .spinner, [data-loading="true"], .skeleton');
	if (await loadingIndicator.first().isVisible({ timeout: 100 }).catch(() => false)) {
		await loadingIndicator.first().waitFor({ state: 'hidden', timeout: 5000 }).catch(() => {
			// Loading indicator may have already disappeared
		});
	}
}

/**
 * Ensure the correct demo tenant is selected for this worker
 */
export async function ensureDemoTenant(page: Page, testInfo: TestInfo): Promise<void> {
	const creds = getDemoCredentials(testInfo);
	const selector = page.locator('select').first();

	if (await selector.isVisible()) {
		const options = await selector.locator('option').all();
		for (const option of options) {
			const text = await option.textContent();
			if (text && text.toLowerCase().includes(creds.tenantSlug)) {
				const value = await option.getAttribute('value');
				if (value) {
					await selector.selectOption(value);
					break;
				}
			}
		}
		await page.waitForLoadState('networkidle');
	}
}

// Keep backward-compatible exports for gradual migration
// NOTE: Using demo2 for tests, demo1 is reserved for end users
export const DEMO_EMAIL = 'demo2@example.com';
export const DEMO_PASSWORD = 'demo12345';

/**
 * @deprecated Use ensureDemoTenant instead
 */
export async function ensureAcmeTenant(page: Page): Promise<void> {
	const selector = page.locator('select').first();
	if (await selector.isVisible()) {
		const currentValue = await selector.inputValue();
		if (!currentValue.includes('demo')) {
			const options = await selector.locator('option').all();
			for (const option of options) {
				const text = await option.textContent();
				if (text && /demo/i.test(text)) {
					const value = await option.getAttribute('value');
					if (value) {
						await selector.selectOption(value);
						break;
					}
				}
			}
			await page.waitForLoadState('networkidle');
		}
	}
}

export async function assertTableRowCount(page: Page, minRows: number): Promise<void> {
	const rows = page.locator('table tbody tr');
	const count = await rows.count();
	expect(count).toBeGreaterThanOrEqual(minRows);
}

export async function assertTextVisible(page: Page, text: string | RegExp): Promise<void> {
	await expect(page.getByText(text).first()).toBeVisible({ timeout: 10000 });
}

/**
 * Wait for backend to be ready before running tests.
 * Polls the health endpoint until it responds successfully.
 */
export async function waitForBackendReady(baseUrl: string, maxWaitMs = 30000): Promise<boolean> {
	const healthUrl = `${baseUrl}/health`;
	const startTime = Date.now();
	const pollInterval = 1000;

	while (Date.now() - startTime < maxWaitMs) {
		try {
			const response = await fetch(healthUrl);
			if (response.ok) {
				console.log(`Backend ready after ${Date.now() - startTime}ms`);
				return true;
			}
		} catch {
			// Backend not ready yet, continue polling
		}
		await new Promise((resolve) => setTimeout(resolve, pollInterval));
	}

	console.warn(`Backend not ready after ${maxWaitMs}ms`);
	return false;
}

/**
 * Wait for a table to have data rows.
 * @param page - Playwright page object
 * @param minRows - Minimum number of rows expected (default: 1)
 * @param timeout - Maximum wait time in ms (default: 10000)
 */
export async function waitForTableData(
	page: Page,
	minRows = 1,
	timeout = 10000
): Promise<void> {
	const tableBody = page.locator('table tbody');
	await tableBody.waitFor({ state: 'visible', timeout });

	// Wait for at least minRows to appear
	await expect(async () => {
		const rows = await tableBody.locator('tr').count();
		expect(rows).toBeGreaterThanOrEqual(minRows);
	}).toPass({ timeout });
}

/**
 * Wait for a modal to be fully visible and ready for interaction.
 * @param page - Playwright page object
 * @param timeout - Maximum wait time in ms (default: 10000)
 */
export async function waitForModalReady(page: Page, timeout = 10000): Promise<void> {
	// Wait for modal container
	const modal = page.locator('[role="dialog"], .modal, [data-testid="modal"]');
	await modal.waitFor({ state: 'visible', timeout });

	// Wait for any loading indicators inside the modal to disappear
	const loadingIndicator = modal.locator('.loading, .spinner, [data-loading="true"]');
	if (await loadingIndicator.isVisible().catch(() => false)) {
		await loadingIndicator.waitFor({ state: 'hidden', timeout });
	}

	// Small delay for animations to complete
	await page.waitForTimeout(100);
}

/**
 * Wait for a form submission to complete.
 * Waits for network activity to settle and checks for success/error indicators.
 * @param page - Playwright page object
 * @param timeout - Maximum wait time in ms (default: 10000)
 */
export async function waitForFormSubmission(page: Page, timeout = 10000): Promise<void> {
	// Wait for network to settle after form submission
	await page.waitForLoadState('networkidle', { timeout });

	// Check if there's a success toast/message
	const successIndicator = page.locator('.toast-success, .alert-success, [data-testid="success-message"]');
	const errorIndicator = page.locator('.toast-error, .alert-error, [data-testid="error-message"]');

	// Wait a bit for any toast to appear
	await page.waitForTimeout(200);

	// If error indicator is visible, throw an error
	if (await errorIndicator.isVisible().catch(() => false)) {
		const errorText = await errorIndicator.textContent().catch(() => 'Unknown error');
		throw new Error(`Form submission failed: ${errorText}`);
	}
}

/**
 * Wait for page to be fully loaded and interactive.
 * More reliable than waitForLoadState('networkidle') alone.
 * @param page - Playwright page object
 * @param timeout - Maximum wait time in ms (default: 10000)
 */
export async function waitForPageReady(page: Page, timeout = 10000): Promise<void> {
	await page.waitForLoadState('domcontentloaded', { timeout });

	// Wait for any loading overlays to disappear
	const loadingOverlay = page.locator('.loading-overlay, [data-loading="true"], .skeleton');
	if (await loadingOverlay.first().isVisible().catch(() => false)) {
		await loadingOverlay.first().waitFor({ state: 'hidden', timeout });
	}
}
