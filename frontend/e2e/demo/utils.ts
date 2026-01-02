import { Page, expect } from '@playwright/test';

export const DEMO_URL = 'https://open-accounting.up.railway.app';
export const DEMO_API_URL = 'https://open-accounting-api.up.railway.app';
export const DEMO_EMAIL = 'demo@example.com';
export const DEMO_PASSWORD = 'demo123';

export async function loginAsDemo(page: Page): Promise<void> {
	await page.goto(`${DEMO_URL}/login`);
	await page.waitForLoadState('networkidle');
	await page.getByLabel(/email/i).fill(DEMO_EMAIL);
	await page.getByLabel(/password/i).fill(DEMO_PASSWORD);
	await page.getByRole('button', { name: /sign in|login/i }).click();
	await page.waitForURL(/dashboard/, { timeout: 30000 });
	await page.waitForLoadState('networkidle');
}

export async function navigateTo(page: Page, path: string): Promise<void> {
	await page.goto(`${DEMO_URL}${path}`);
	await page.waitForLoadState('networkidle');
	await page.waitForTimeout(500);
}

export async function ensureAcmeTenant(page: Page): Promise<void> {
	const selector = page.locator('select').first();
	if (await selector.isVisible()) {
		const currentValue = await selector.inputValue();
		if (!currentValue.includes('acme') && !currentValue.includes('Acme')) {
			// Find the option containing "Acme" and select by its value
			const options = await selector.locator('option').all();
			for (const option of options) {
				const text = await option.textContent();
				if (text && /Acme/i.test(text)) {
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
