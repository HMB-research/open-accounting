import { expect, type Locator, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, ensureDemoTenant, navigateTo, waitForRouteReady } from './utils';

const absencesLoadedState = 'table.table, .empty-state, .alert-error';

export async function setupAbsencesPage(page: Page, testInfo: TestInfo): Promise<void> {
	await ensureAuthenticated(page, testInfo);
	await ensureDemoTenant(page, testInfo);
	await openAbsencesPage(page, testInfo);
}

export async function openAbsencesPage(page: Page, testInfo: TestInfo): Promise<void> {
	await navigateTo(page, '/employees/absences', testInfo, { waitForNetworkIdle: false });
	await waitForAbsencesPageReady(page);
}

export async function waitForAbsencesPageReady(page: Page): Promise<void> {
	await waitForRouteReady(page, 'main h1, #yearFilter, #employeeFilter, .tabs, .alert-error', 15000);
	await page
		.getByText(/^Loading\.\.\.$|^Laadimine\.\.\.$/i)
		.waitFor({ state: 'hidden', timeout: 15000 })
		.catch(() => {});
	await waitForRouteReady(page, absencesLoadedState, 15000);
}

export function absenceTabs(page: Page): Locator {
	return page.locator('.tabs .tab, .tabs button');
}

export function requestLeaveButton(page: Page): Locator {
	return page.getByRole('button', { name: /request|taotl|\+/i }).first();
}

export async function switchAbsenceTab(page: Page, tabIndex: number): Promise<void> {
	const tabs = absenceTabs(page);
	await expect(tabs.nth(tabIndex)).toBeVisible({ timeout: 10000 });
	await tabs.nth(tabIndex).click();
	await expect(tabs.nth(tabIndex)).toHaveClass(/active/);
	await expect(page.locator('table, .empty-state, .alert-error').first()).toBeVisible({
		timeout: 5000
	});
}

export async function openRequestLeaveModal(page: Page): Promise<Locator> {
	const requestButton = requestLeaveButton(page);
	await expect(requestButton).toBeVisible({ timeout: 10000 });
	await requestButton.click();

	const modal = page.locator('[role="dialog"], .modal').first();
	await expect(modal).toBeVisible({ timeout: 5000 });
	return modal;
}
