import { expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, ensureDemoTenant, navigateTo, waitForRouteReady } from './utils';

const reminderLoadedState = '.summary-card, .empty-state, .alert-error';

export async function openPaymentRemindersPage(page: Page, testInfo: TestInfo): Promise<void> {
	await navigateTo(page, '/invoices/reminders', testInfo, { waitForNetworkIdle: false });
	await waitForRouteReady(page, 'main h1, .summary-card, .empty-state, .alert-error');
}

export async function setupPaymentRemindersPage(page: Page, testInfo: TestInfo): Promise<void> {
	await ensureAuthenticated(page, testInfo);
	await ensureDemoTenant(page, testInfo);
	await openPaymentRemindersPage(page, testInfo);
}

export async function waitForReminderDataState(page: Page): Promise<void> {
	await waitForRouteReady(page, reminderLoadedState, 15000);
}

export async function hasReminderTable(page: Page): Promise<boolean> {
	await waitForReminderDataState(page);
	return page.locator('table.table').isVisible().catch(() => false);
}

export async function expectTerminalReminderState(page: Page): Promise<void> {
	await expect(page.locator('.empty-state, .alert-error').first()).toBeVisible();
}

export async function openFirstSendModalIfAvailable(page: Page): Promise<boolean> {
	if (!(await hasReminderTable(page))) {
		await expectTerminalReminderState(page);
		return false;
	}

	const sendButton = page.locator('tbody button.btn-sm:not([disabled])').first();
	if (!(await sendButton.isVisible().catch(() => false))) {
		await expect(page.locator('tbody button.btn-sm').first()).toBeVisible();
		return false;
	}

	await sendButton.click();
	await expect(page.locator('.modal')).toBeVisible({ timeout: 5000 });
	return true;
}
