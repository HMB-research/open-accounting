import { expect, type Locator, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, ensureDemoTenant, navigateTo, waitForRouteReady } from './utils';

const costCentersTerminalState = 'table.table, .card-body h5, .alert-danger';

export async function setupCostCentersPage(page: Page, testInfo: TestInfo): Promise<void> {
	await ensureAuthenticated(page, testInfo);
	await ensureDemoTenant(page, testInfo);
	await openCostCentersPage(page, testInfo);
}

export async function openCostCentersPage(page: Page, testInfo: TestInfo): Promise<void> {
	await navigateTo(page, '/settings/cost-centers', testInfo, { waitForNetworkIdle: false });
	await waitForCostCentersPageReady(page);
}

export async function waitForCostCentersPageReady(page: Page): Promise<void> {
	await waitForRouteReady(
		page,
		'main h1, h1, button.btn-primary, .allocation-form, table.table, .card-body h5, .alert-danger',
		15000
	);
	await page.locator('.spinner-border').first().waitFor({ state: 'hidden', timeout: 15000 }).catch(() => {});
	await waitForRouteReady(page, costCentersTerminalState, 15000);
	await waitForRouteReady(page, '.allocation-form, .alert-danger', 15000);
}

export function addCostCenterButton(page: Page): Locator {
	return page.getByRole('button', { name: /add cost center|lisa kulukoht|add|lisa|\+/i }).first();
}

export function costCenterModal(page: Page): Locator {
	return page.locator('.modal.show, .modal').filter({ has: page.locator('#code') }).first();
}

export async function openCostCenterModal(page: Page): Promise<Locator> {
	const addButton = addCostCenterButton(page);
	await expect(addButton).toBeVisible({ timeout: 10000 });
	await addButton.click();

	const modal = costCenterModal(page);
	await expect(modal).toBeVisible({ timeout: 5000 });
	return modal;
}
