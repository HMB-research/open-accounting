import { Page, expect, TestInfo } from '@playwright/test';

// Use environment variables for local testing, fall back to Railway for remote demo testing
export const DEMO_URL = process.env.BASE_URL || 'https://open-accounting.up.railway.app';
export const DEMO_API_URL = process.env.PUBLIC_API_URL || 'https://open-accounting-api.up.railway.app';

// Demo user reserved for end users (README documentation)
// This user should NOT be used by automated tests to avoid conflicts
export const END_USER_DEMO = { email: 'demo1@example.com', password: 'demo12345', tenantSlug: 'demo1', tenantName: 'Demo Company 1', tenantId: 'b0000000-0000-0000-0001-000000000001' };

// Demo credentials for E2E tests only (demo2, demo3, demo4)
// Tenant IDs follow the pattern: b0000000-0000-0000-000X-000000000001 where X is user number
// NOTE: demo1 is reserved for end users - tests use demo2, demo3, demo4 only
export const DEMO_CREDENTIALS = [
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

	await page.waitForLoadState('networkidle');
	console.log(`Login completed in ${Date.now() - startTime}ms for ${creds.email}`);
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
	await page.waitForLoadState('networkidle');
	await page.waitForTimeout(500);
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
