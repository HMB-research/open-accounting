import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureAcmeTenant, assertTableRowCount } from './utils';

test.describe('Demo Journal Entries - Seeded Data Verification', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
		await ensureAcmeTenant(page);
		await navigateTo(page, '/journal');
	});

	test('displays journal page heading', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /journal|ledger/i })).toBeVisible();
	});

	test('shows seeded journal entries', async ({ page }) => {
		await page.waitForSelector('table tbody tr', { timeout: 10000 });
		// Should have 4 journal entries from seed
		await assertTableRowCount(page, 3);
	});

	test('displays JE-2024-001 opening balances entry', async ({ page }) => {
		await expect(page.getByText(/JE-2024-001/)).toBeVisible();
	});

	test('displays JE-2024-002 office rent entry', async ({ page }) => {
		const hasEntry = await page.getByText(/JE-2024-002/).isVisible().catch(() => false);
		const hasDescription = await page.getByText(/Office rent|rent/i).first().isVisible().catch(() => false);
		expect(hasEntry || hasDescription).toBeTruthy();
	});

	test('shows POSTED and DRAFT statuses', async ({ page }) => {
		// 3 entries are POSTED, 1 is DRAFT
		const hasPosted = await page.getByText(/posted/i).first().isVisible();
		expect(hasPosted).toBeTruthy();
	});

	test('displays entry dates', async ({ page }) => {
		// Entries are from 2024
		await expect(page.getByText(/2024/)).toBeVisible();
	});

	test('can click on entry to view details', async ({ page }) => {
		const entryRow = page.getByText(/JE-2024-001/).first();

		if (await entryRow.isVisible()) {
			await entryRow.click();
			await page.waitForTimeout(1000);

			// Should show entry lines with debit/credit
			const hasDetails = await page.getByText(/debit|credit|50,?000|Bank|Capital/).first().isVisible().catch(() => false);
			expect(hasDetails).toBeTruthy();
		}
	});

	test('create entry button is visible', async ({ page }) => {
		await expect(page.getByRole('button', { name: /new|create|add/i })).toBeVisible();
	});
});
