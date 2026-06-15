import { Page, expect, TestInfo } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';

// ESM-compatible __dirname
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

export const DEMO_URL = process.env.BASE_URL || 'http://localhost:5173';
export const DEMO_API_URL = process.env.PUBLIC_API_URL || 'http://localhost:8080';

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

export type DemoCredentials = (typeof DEMO_CREDENTIALS)[number];

/**
 * Get demo credentials for the current worker
 * @param testInfo - Playwright TestInfo object containing parallelIndex
 */
export function getDemoCredentials(testInfo: TestInfo) {
	const workerIndex = testInfo.parallelIndex % DEMO_CREDENTIALS.length;
	return DEMO_CREDENTIALS[workerIndex];
}

/**
 * Login with explicit demo credentials.
 */
export async function loginWithDemoCredentials(
	page: Page,
	creds: DemoCredentials,
	options: { rememberMe?: boolean; logPrefix?: string } = {}
): Promise<void> {
	const startTime = Date.now();
	const logPrefix = options.logPrefix ?? 'Login';

	await page.goto(`${DEMO_URL}/login`);
	await page.waitForLoadState('domcontentloaded');
	await page.waitForLoadState('networkidle').catch(() => {
		// Vite and background requests can keep the page busy.
	});

	for (let attempt = 1; attempt <= 2; attempt += 1) {
		await fillLoginForm(page, creds, options);

		try {
			const outcome = await waitForLoginOutcome(page);
			if (outcome === 'dashboard') {
				break;
			}

			const loginError = await readVisibleLoginError(page);
			if (loginError) {
				throw new Error(`Login failed for ${creds.email}: ${loginError}`);
			}

			if (attempt < 2) {
				console.log(`${logPrefix} stayed on login after submit for ${creds.email}; retrying`);
				await page.goto(`${DEMO_URL}/login`);
				await page.waitForLoadState('networkidle').catch(() => {});
				continue;
			}

			const currentUrl = page.url();
			const formState = await getLoginFormState(page);
			throw new Error(
				`Login navigation timeout for ${creds.email}. Current URL: ${currentUrl}. Form state: ${formState}`
			);
		} catch (error) {
			const loginError = await readVisibleLoginError(page);
			if (loginError) {
				throw new Error(`Login failed for ${creds.email}: ${loginError}`);
			}
			if (attempt < 2 && page.url().includes('/login')) {
				console.log(`${logPrefix} could not confirm dashboard for ${creds.email}; retrying`);
				await page.goto(`${DEMO_URL}/login`);
				await page.waitForLoadState('networkidle').catch(() => {});
				continue;
			}
			throw error;
		}
	}

	// Use domcontentloaded instead of networkidle (more reliable for SPA)
	await page.waitForLoadState('domcontentloaded');
	// Wait for dashboard content to appear
	await page.waitForSelector('h1, .dashboard-header, [data-testid="dashboard"]', { timeout: 10000 }).catch(() => {
		// Dashboard loaded even if selector not found
	});
	console.log(`${logPrefix} completed in ${Date.now() - startTime}ms for ${creds.email}`);
}

async function fillLoginForm(
	page: Page,
	creds: DemoCredentials,
	options: { rememberMe?: boolean; logPrefix?: string }
): Promise<void> {
	const logPrefix = options.logPrefix ?? 'Login';
	const emailInput = page.locator('input[type="email"], input[name="email"]').first();
	const passwordInput = page.locator('input[type="password"]').first();
	await emailInput.waitFor({ state: 'visible', timeout: 10000 });
	await passwordInput.waitFor({ state: 'visible', timeout: 10000 });
	await emailInput.fill(creds.email);
	await passwordInput.fill(creds.password);
	await expect(emailInput).toHaveValue(creds.email);
	await expect(passwordInput).toHaveValue(creds.password);

	if (options.rememberMe) {
		const rememberMeCheckbox = page.locator('input[type="checkbox"]').first();
		if (await rememberMeCheckbox.isVisible().catch(() => false)) {
			await rememberMeCheckbox.check();
			console.log(`${logPrefix} checked "Remember Me" for ${creds.email}`);
		}
	}

	const signInButton = page.locator('form button[type="submit"]').first();
	await expect(signInButton).toBeEnabled({ timeout: 10000 });
	await signInButton.click();
}

async function waitForLoginOutcome(page: Page): Promise<'dashboard' | 'login'> {
	const dashboardPromise = page
		.waitForURL(/dashboard/, { timeout: 30000 })
		.then((): 'dashboard' => 'dashboard');
	const loginReloadPromise = page
		.waitForURL((url) => url.href.endsWith('/login?'), { timeout: 5000 })
		.then((): 'login' => 'login')
		.catch(() => undefined);
	const firstOutcome = await Promise.race([dashboardPromise, loginReloadPromise]);

	if (firstOutcome) {
		return firstOutcome;
	}

	return dashboardPromise;
}

async function readVisibleLoginError(page: Page): Promise<string | null> {
	const errorAlert = page.locator('.alert-error, [role="alert"]').first();
	if (!(await errorAlert.isVisible().catch(() => false))) {
		return null;
	}

	return (await errorAlert.textContent().catch(() => 'Unknown error'))?.trim() || 'Unknown error';
}

async function getLoginFormState(page: Page): Promise<string> {
	return page
		.evaluate(() => {
			const email = document.querySelector<HTMLInputElement>('input[type="email"], input[name="email"]');
			const password = document.querySelector<HTMLInputElement>('input[type="password"]');
			const submit = document.querySelector<HTMLButtonElement>('form button[type="submit"]');
			return JSON.stringify({
				email: email?.value ?? '',
				passwordLength: password?.value.length ?? 0,
				submitDisabled: submit?.disabled ?? null
			});
		})
		.catch(() => 'unavailable');
}

/**
 * Login as the demo user assigned to this worker
 */
export async function loginAsDemo(page: Page, testInfo: TestInfo): Promise<void> {
	await loginWithDemoCredentials(page, getDemoCredentials(testInfo));
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
		const hasStoredAccessToken = authData.origins?.some((origin: { localStorage?: { name: string }[] }) =>
			origin.localStorage?.some((item) => item.name === 'access_token')
		);
		if (!hasStoredAccessToken) {
			console.log(`[Worker ${workerIndex}] Auth file has no access token: ${authFile}`);
			return false;
		}

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

interface EnsureAuthenticatedOptions {
	verifyDashboard?: boolean;
}

/**
 * Ensure authentication - try to load saved auth state, fall back to login.
 * This is the preferred way to authenticate in tests for better performance.
 */
export async function ensureAuthenticated(
	page: Page,
	testInfo: TestInfo,
	options: EnsureAuthenticatedOptions = {}
): Promise<void> {
	const workerIndex = testInfo.parallelIndex % DEMO_CREDENTIALS.length;
	const startTime = Date.now();

	// Try to load saved auth state
	const authLoaded = await loadAuthState(page, workerIndex);

	if (authLoaded) {
		if (!options.verifyDashboard) {
			console.log(`[Worker ${workerIndex}] Session loaded in ${Date.now() - startTime}ms`);
			return;
		}

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

interface NavigateToOptions {
	waitForNetworkIdle?: boolean;
}

export async function navigateTo(
	page: Page,
	path: string,
	testInfo?: TestInfo,
	options: NavigateToOptions = {}
): Promise<void> {
	let url = `${DEMO_URL}${path}`;
	const waitForNetworkIdle = options.waitForNetworkIdle ?? true;
	// Append tenant ID if testInfo is provided and path doesn't already have query params
	if (testInfo) {
		const creds = getDemoCredentials(testInfo);
		const separator = path.includes('?') ? '&' : '?';
		url = `${url}${separator}tenant=${creds.tenantId}`;
	}
	await page.goto(url);
	// Wait for DOM to be ready
	await page.waitForLoadState('domcontentloaded');

	// Wait for any loading overlays to disappear
	const loadingIndicator = page.locator('.loading, .spinner, [data-loading="true"], .skeleton');
	if (await loadingIndicator.first().isVisible({ timeout: 100 }).catch(() => false)) {
		await loadingIndicator.first().waitFor({ state: 'hidden', timeout: 5000 }).catch(() => {
			// Loading indicator may have already disappeared
		});
	}

	// Wait for main content container to be visible (indicates page has rendered)
	await page.locator('.main-content, main, [role="main"]').first().waitFor({
		state: 'visible',
		timeout: 10000
	}).catch(() => {
		// Main content selector might not exist on all pages
	});

	if (waitForNetworkIdle) {
		// Wait for network to settle (API calls to complete)
		await page.waitForLoadState('networkidle').catch(() => {
			// Network might not settle in timeout, continue anyway
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
		const currentValue = await selector.inputValue().catch(() => '');
		const options = await selector.locator('option').all();
		for (const option of options) {
			const text = await option.textContent();
			if (text && text.toLowerCase().includes(creds.tenantSlug)) {
				const value = await option.getAttribute('value');
				if (value) {
					if (currentValue === value) return;
					await selector.selectOption(value);
					await expect(selector).toHaveValue(value);
					await page.waitForLoadState('domcontentloaded');
					break;
				}
			}
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
 * Wait for page to be fully loaded and interactive.
 * More reliable than waitForLoadState('networkidle') alone.
 * @param page - Playwright page object
 * @param timeout - Maximum wait time in ms (default: 10000)
 */
export async function waitForPageReady(page: Page, timeout = 10000): Promise<void> {
	await page.waitForLoadState('domcontentloaded', { timeout });

	// Wait for any loading overlays to disappear
	await waitForLoadingIndicatorsToClear(page, timeout);
}

/**
 * Wait for a route-owned selector after the DOM is ready.
 * Use this instead of fixed sleeps after navigateTo.
 */
export async function waitForRouteReady(
	page: Page,
	readySelector: string,
	timeout = 10000
): Promise<void> {
	await page.waitForLoadState('domcontentloaded', { timeout });
	await expect(async () => {
		const visibleCount = await page.locator(readySelector).evaluateAll((elements) => {
			return elements.filter((element) => {
				const style = window.getComputedStyle(element);
				const rect = element.getBoundingClientRect();
				return (
					style.display !== 'none' &&
					style.visibility !== 'hidden' &&
					rect.width > 0 &&
					rect.height > 0
				);
			}).length;
		});

		expect(visibleCount).toBeGreaterThan(0);
	}).toPass({ timeout });
}

interface WaitForLoadingOptions {
	includePlainText?: boolean;
}

async function waitForLoadingIndicatorsToClear(
	page: Page,
	timeout: number,
	options: WaitForLoadingOptions = {}
): Promise<void> {
	const loadingIndicators = page.locator(
		'.loading, .loading-spinner, .loading-overlay, .spinner, .animate-spin, [data-loading="true"], .skeleton'
	);
	const loadingText = options.includePlainText
		? page.getByText(/^Loading\.\.\.$|^Laadimine\.\.\.$/i)
		: null;

	await expect(async () => {
		const visibleCssCount = await loadingIndicators.evaluateAll((elements) => {
			return elements.filter((element) => {
				const style = window.getComputedStyle(element);
				const rect = element.getBoundingClientRect();
				return (
					style.display !== 'none' &&
					style.visibility !== 'hidden' &&
					rect.width > 0 &&
					rect.height > 0
				);
			}).length;
		});
		const visibleTextCount = loadingText
			? await loadingText.evaluateAll((elements) => {
					return elements.filter((element) => {
						const style = window.getComputedStyle(element);
						const rect = element.getBoundingClientRect();
						return (
							style.display !== 'none' &&
							style.visibility !== 'hidden' &&
							rect.width > 0 &&
							rect.height > 0
						);
					}).length;
				})
			: 0;

		expect(visibleCssCount + visibleTextCount).toBe(0);
	}).toPass({ timeout });
}

/**
 * Wait for table to have data rows OR an empty state message.
 * Handles cases where demo data may not be seeded.
 * @param page - Playwright page object
 * @param timeout - Maximum wait time in ms (default: 15000)
 * @returns Object indicating whether data exists
 */
export async function waitForDataOrEmpty(
	page: Page,
	timeout = 15000
): Promise<{ hasData: boolean; isEmpty: boolean }> {
	const tableBody = page.locator('table tbody');
	const emptyIndicators = page.locator(
		'.empty-state, [data-testid="empty"], text=/no data|no records|empty|no results/i'
	);
	// Wait for loading to complete first
	try {
		await waitForLoadingIndicatorsToClear(page, Math.min(timeout, 10000), {
			includePlainText: true
		});
	} catch {
		// Loading might have already completed
	}

	// Race between table data and empty state
	try {
		await Promise.race([
			tableBody.locator('tr').first().waitFor({ state: 'visible', timeout }),
			emptyIndicators.first().waitFor({ state: 'visible', timeout })
		]);
	} catch {
		// Neither appeared within timeout
	}

	const rowCount = await tableBody.locator('tr').count();
	const hasEmpty = await emptyIndicators.first().isVisible().catch(() => false);

	return {
		hasData: rowCount > 0,
		isEmpty: hasEmpty || rowCount === 0
	};
}
