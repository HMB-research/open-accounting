import { Page, expect, TestInfo } from '@playwright/test';

// Use environment variables for local testing, fall back to Railway for remote demo testing
export const DEMO_URL = process.env.BASE_URL || 'https://open-accounting.up.railway.app';
export const DEMO_API_URL = process.env.PUBLIC_API_URL || 'https://open-accounting-api.up.railway.app';

// Demo credentials mapped by worker index (0-2)
export const DEMO_CREDENTIALS = [
	{ email: 'demo1@example.com', password: 'demo12345', tenantSlug: 'demo1', tenantName: 'Demo Company 1' },
	{ email: 'demo2@example.com', password: 'demo12345', tenantSlug: 'demo2', tenantName: 'Demo Company 2' },
	{ email: 'demo3@example.com', password: 'demo12345', tenantSlug: 'demo3', tenantName: 'Demo Company 3' }
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

	await page.goto(`${DEMO_URL}/login`);
	await page.waitForLoadState('networkidle');
	await page.getByLabel(/email/i).fill(creds.email);
	await page.getByLabel(/password/i).fill(creds.password);
	await page.getByRole('button', { name: /sign in|login/i }).click();
	await page.waitForURL(/dashboard/, { timeout: 30000 });
	await page.waitForLoadState('networkidle');
}

export async function navigateTo(page: Page, path: string): Promise<void> {
	await page.goto(`${DEMO_URL}${path}`);
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
export const DEMO_EMAIL = 'demo1@example.com';
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
