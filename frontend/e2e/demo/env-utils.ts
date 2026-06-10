import { expect, type Page, type TestInfo } from '@playwright/test';
import {
	DEMO_CREDENTIALS,
	DEMO_URL,
	ensureAuthenticated,
	loginWithDemoCredentials,
	navigateTo,
	waitForPageReady
} from './utils';

// Use demo1 for environment smoke tests that intentionally exercise the full
// login workflow. Tests with TestInfo use the worker-assigned auth state.
export const DEMO_USER = DEMO_CREDENTIALS[0];
const DEMO_TENANT_ID = DEMO_USER.tenantId;

export async function waitForDemoShell(page: Page) {
	await waitForPageReady(page);
	await page.locator('nav.navbar, .mobile-menu-btn').first().waitFor({ state: 'visible', timeout: 10000 });
}

export async function waitForVisibleContent(page: Page) {
	await waitForPageReady(page);
	await page.getByText(/^Loading\.\.\.$/).waitFor({ state: 'hidden', timeout: 10000 }).catch(() => {});
	await expect(page.locator('main, .main-content, [class*="content"]').first()).toBeVisible({ timeout: 10000 });
}

export async function clickWizardButton(page: Page, name: RegExp) {
	const button = page.getByRole('button', { name }).first();
	await expect(button).toBeVisible({ timeout: 10000 });
	await expect(button).toBeEnabled();
	await button.click();
}

export async function advanceWizardStep(page: Page, buttonName: RegExp, nextHeading: RegExp) {
	await clickWizardButton(page, buttonName);
	await expect(page.getByRole('heading', { name: nextHeading })).toBeVisible({ timeout: 10000 });
}

export async function loginAsDemoEnv(page: Page, testInfo?: TestInfo) {
	if (testInfo) {
		await ensureAuthenticated(page, testInfo);
		await navigateTo(page, '/dashboard', testInfo, { waitForNetworkIdle: false });
		await waitForDemoShell(page);
		return;
	}

	await loginWithDemoCredentials(page, DEMO_USER, { logPrefix: 'Demo env login' });
	await waitForDemoShell(page);
}

export async function navigateToEnvPage(page: Page, path: string) {
	const separator = path.includes('?') ? '&' : '?';
	const url = `${DEMO_URL}${path}${separator}tenant=${DEMO_TENANT_ID}`;
	await page.goto(url);
	await waitForVisibleContent(page);
}
