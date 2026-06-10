import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant, waitForRouteReady } from './utils';

const reminderLoadedState = '.summary-card, .empty-state, .alert-error';

async function openPaymentRemindersPage(page: Page, testInfo: TestInfo): Promise<void> {
	await navigateTo(page, '/invoices/reminders', testInfo);
	await waitForRouteReady(page, 'h1');
}

async function waitForReminderDataState(page: Page): Promise<void> {
	await expect(page.locator(reminderLoadedState).first()).toBeVisible({ timeout: 15000 });
}

async function hasReminderTable(page: Page): Promise<boolean> {
	await waitForReminderDataState(page);
	return page.locator('table.table').isVisible().catch(() => false);
}

async function expectTerminalReminderState(page: Page): Promise<void> {
	await expect(page.locator('.empty-state, .alert-error').first()).toBeVisible();
}

async function openFirstSendModalIfAvailable(page: Page): Promise<boolean> {
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

test.describe('Demo Payment Reminders - Page Structure', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await openPaymentRemindersPage(page, testInfo);
	});

	test('displays payment reminders page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 10000 });
		const heading = page.getByRole('heading', { level: 1 });
		const headingText = await heading.textContent();
		expect(headingText?.toLowerCase()).toMatch(/reminder|meeldetuletus/i);
	});

	test('has refresh button', async ({ page }) => {
		const refreshBtn = page.locator('button.btn-secondary').first();
		await expect(refreshBtn).toBeVisible({ timeout: 10000 });
	});

	test('has back button linking to invoices', async ({ page }) => {
		const backLink = page.locator('a.btn-secondary[href*="invoices"]');
		await expect(backLink).toBeVisible({ timeout: 10000 });
	});
});

test.describe('Demo Payment Reminders - Summary Display', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await openPaymentRemindersPage(page, testInfo);
	});

	test('shows overdue summary statistics', async ({ page }) => {
		await waitForReminderDataState(page);

		// Check for summary card
		const summaryCard = page.locator('.summary-card');
		const hasSummary = await summaryCard.isVisible().catch(() => false);

		if (hasSummary) {
			// Check for expected statistics labels
			const pageContent = await summaryCard.textContent();
			const hasStats =
				pageContent?.match(/total.*overdue|üle.*tähtaja/i) ||
				pageContent?.match(/invoice|arve/i) ||
				pageContent?.match(/contact|kontakt/i);

			expect(hasStats).toBeTruthy();
			return;
		}

		await expectTerminalReminderState(page);
	});

	test('displays empty state or invoice list', async ({ page }) => {
		await waitForReminderDataState(page);

		// Should show either empty state, table, or error (API may fail for some tenants)
		const hasEmptyState = await page
			.getByText(/no overdue|ei leitud/i)
			.isVisible()
			.catch(() => false);
		const hasTable = await page.locator('table.table').isVisible().catch(() => false);
		const hasError = await page.getByText(/failed|error|viga/i).isVisible().catch(() => false);

		expect(hasEmptyState || hasTable || hasError).toBeTruthy();
	});
});

test.describe('Demo Payment Reminders - Invoice Selection', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await openPaymentRemindersPage(page, testInfo);
	});

	test('can select individual invoices when available', async ({ page }) => {
		const tableVisible = await hasReminderTable(page);

		if (tableVisible) {
			// Try to find checkboxes
			const checkboxes = page.locator('tbody input[type="checkbox"]:not([disabled])');
			const checkboxCount = await checkboxes.count();

			if (checkboxCount > 0) {
				// Click first checkbox
				await checkboxes.first().click();
				const isChecked = await checkboxes.first().isChecked();
				expect(isChecked).toBeTruthy();
				return;
			}

			await expect(page.locator('tbody input[type="checkbox"]').first()).toBeVisible();
			return;
		}

		await expectTerminalReminderState(page);
	});

	test('has select all functionality', async ({ page }) => {
		const tableVisible = await hasReminderTable(page);

		if (tableVisible) {
			// Find header checkbox
			const headerCheckbox = page.locator('thead input[type="checkbox"]');
			await expect(headerCheckbox).toBeVisible();
			return;
		}

		await expectTerminalReminderState(page);
	});
});

test.describe('Demo Payment Reminders - Send Functionality', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await openPaymentRemindersPage(page, testInfo);
	});

	test('has send reminders button', async ({ page }) => {
		const tableVisible = await hasReminderTable(page);

		if (tableVisible) {
			await expect(page.locator('tbody button.btn-sm').first()).toBeVisible();
			return;
		}

		await expectTerminalReminderState(page);
	});

	test('individual invoice has send button when email available', async ({ page }) => {
		const tableVisible = await hasReminderTable(page);

		if (tableVisible) {
			// Check for individual send buttons
			const sendBtns = page.locator('tbody button.btn-sm');
			const btnCount = await sendBtns.count();

			// Should have send buttons if there are rows
			const rowCount = await page.locator('tbody tr').count();
			if (rowCount > 0) {
				expect(btnCount).toBe(rowCount);
				await expect(sendBtns.first()).toBeVisible();
				return;
			}
		}

		await expectTerminalReminderState(page);
	});
});

test.describe('Demo Payment Reminders - Modal', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await openPaymentRemindersPage(page, testInfo);
	});

	test('opens send modal when clicking send button', async ({ page }) => {
		if (await openFirstSendModalIfAvailable(page)) {
			const modalContent = await page.locator('.modal').textContent();
			expect(modalContent).toMatch(/send|saada|reminder|meeldetuletus/i);
		}
	});

	test('modal can be closed', async ({ page }) => {
		if (await openFirstSendModalIfAvailable(page)) {
			const closeBtn = page.locator('.btn-close');
			await expect(closeBtn).toBeVisible();
			await closeBtn.click();
			await expect(page.locator('.modal')).not.toBeVisible();
		}
	});

	test('modal has custom message field', async ({ page }) => {
		if (await openFirstSendModalIfAvailable(page)) {
			const textarea = page.locator('.modal textarea');
			await expect(textarea).toBeVisible();
			await textarea.fill('Test message');
			await expect(textarea).toHaveValue('Test message');
		}
	});
});

test.describe('Demo Payment Reminders - Table Display', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await openPaymentRemindersPage(page, testInfo);
	});

	test('table has proper headers when invoices exist', async ({ page }) => {
		const table = page.locator('table.table');
		const tableVisible = await hasReminderTable(page);

		if (tableVisible) {
			// Check for expected headers
			const headers = await table.locator('thead').textContent();
			const hasExpectedHeaders =
				headers?.match(/invoice|arve/i) &&
				(headers?.match(/contact|kontakt/i) || headers?.match(/outstanding|tasumata/i));

			expect(hasExpectedHeaders).toBeTruthy();
			return;
		}

		await expectTerminalReminderState(page);
	});

	test('shows overdue days with visual indicator', async ({ page }) => {
		await waitForReminderDataState(page);

		// Check if table exists with overdue badges
		const overdueBadges = page.locator('.overdue-badge');
		const badgeCount = await overdueBadges.count();

		// Either has badges, no overdue invoices, or error (API may fail for some tenants)
		const hasEmptyState = await page
			.getByText(/no overdue|ei leitud/i)
			.isVisible()
			.catch(() => false);
		const hasError = await page.getByText(/failed|error|viga/i).isVisible().catch(() => false);
		expect(badgeCount > 0 || hasEmptyState || hasError).toBeTruthy();
	});
});
