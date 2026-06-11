import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, navigateTo, ensureDemoTenant, waitForRouteReady } from './utils';

interface RecurringInvoiceResponse {
	id: string;
	name: string;
	reference?: string;
	frequency: string;
	payment_terms_days: number;
	is_active: boolean;
	lines?: Array<{
		description: string;
		quantity: number | string;
		unit_price: number | string;
		vat_rate: number | string;
	}>;
}

const seededRecurringTemplates = [
	{
		name: 'Monthly Support - TechStart',
		contact: 'TechStart OÜ',
		frequency: /monthly/i,
		generatedCount: '12'
	},
	{
		name: 'Quarterly Retainer - Nordic',
		contact: 'Nordic Solutions AS',
		frequency: /quarterly/i,
		generatedCount: '4'
	},
	{
		name: 'Annual License - GreenTech',
		contact: 'GreenTech Industries',
		frequency: /yearly/i,
		generatedCount: '1'
	}
];

async function waitForRecurringRows(page: Page) {
	await waitForRouteReady(page, 'table tbody tr', 15000);
	await expect(async () => {
		const rowCount = await page.locator('table tbody tr').count();
		expect(rowCount).toBeGreaterThanOrEqual(seededRecurringTemplates.length);
	}).toPass({ timeout: 15000 });
}

async function openRecurring(page: Page, testInfo: TestInfo) {
	const recurringLoaded = page.waitForResponse(
		(response) =>
			response.url().includes('/recurring-invoices') &&
			response.request().method() === 'GET' &&
			response.status() === 200
	);
	const contactsLoaded = page.waitForResponse(
		(response) =>
			response.url().includes('/contacts?active_only=true') &&
			response.request().method() === 'GET' &&
			response.status() === 200
	);
	await navigateTo(page, '/recurring', testInfo, { waitForNetworkIdle: false });
	await Promise.all([recurringLoaded, contactsLoaded]);
	await waitForRecurringRows(page);
}

function recurringRow(page: Page, name: string) {
	return page.locator('table tbody tr').filter({ hasText: name });
}

test.describe('Demo Recurring Invoices - Seed Data Verification', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
		await openRecurring(page, testInfo);
	});

	test('displays seeded recurring invoice details and manages a template', async ({ page }) => {
		await expect(page.getByRole('heading', { name: /recurring invoices/i })).toBeVisible();
		await expect(page.getByRole('button', { name: /new recurring invoice/i })).toBeVisible();
		await expect(page.locator('table tbody tr')).toHaveCount(seededRecurringTemplates.length);

		for (const template of seededRecurringTemplates) {
			const row = recurringRow(page, template.name);
			await expect(row).toBeVisible();
			await expect(row.locator('td').nth(1)).toHaveText(template.contact);
			await expect(row.locator('td').nth(2)).toContainText(template.frequency);
			await expect(row.locator('td').nth(4)).toHaveText(template.generatedCount);
			await expect(row.locator('td').nth(6)).toContainText(/active/i);
		}

		const suffix = `${Date.now()}`;
		const initialName = `E2E recurring ${suffix}`;
		const updatedName = `E2E recurring updated ${suffix}`;
		const initialReference = `REC-${suffix}`;
		const updatedReference = `REC-UPD-${suffix}`;
		const initialLine = `Managed service ${suffix}`;
		const updatedLine = `Managed service updated ${suffix}`;

		await page.getByRole('button', { name: /new recurring invoice/i }).click();
		let dialog = page.getByRole('dialog', { name: /create recurring invoice/i });
		await expect(dialog).toBeVisible();

		await dialog.locator('#name').fill(initialName);
		await dialog.locator('#contact').selectOption({ index: 1 });
		await dialog.locator('#frequency').selectOption('MONTHLY');
		await dialog.locator('#payment_terms').fill('14');
		await dialog.locator('#start_date').fill('2026-06-09');
		await dialog.locator('#end_date').fill('2026-12-31');
		await dialog.locator('#reference').fill(initialReference);

		let lineRow = dialog.locator('.line-row').first();
		await lineRow.locator('input').nth(0).fill(initialLine);
		await lineRow.locator('input').nth(1).fill('2');
		await lineRow.locator('input').nth(2).fill('150.00');
		await lineRow.locator('input').nth(3).fill('22');

		const createResponsePromise = page.waitForResponse((response) => {
			const path = new URL(response.url()).pathname;
			return (
				response.request().method() === 'POST' &&
				/\/api\/v1\/tenants\/[^/]+\/recurring-invoices$/.test(path)
			);
		});
		await dialog.getByRole('button', { name: /^create$/i }).click();
		const createResponse = await createResponsePromise;
		expect(createResponse.status()).toBe(201);
		const created = (await createResponse.json()) as RecurringInvoiceResponse;
		expect(created.name).toBe(initialName);
		expect(created.reference).toBe(initialReference);
		expect(created.frequency).toBe('MONTHLY');
		expect(created.lines?.[0]?.description).toBe(initialLine);

		let row = recurringRow(page, initialName);
		await expect(row).toBeVisible({ timeout: 10000 });
		await expect(row).toContainText(initialReference);
		await expect(row).toContainText(/monthly/i);
		await expect(row).toContainText(/active/i);

		const detailsResponsePromise = page.waitForResponse((response) => {
			const path = new URL(response.url()).pathname;
			return (
				response.request().method() === 'GET' &&
				path.endsWith(`/recurring-invoices/${created.id}`)
			);
		});
		await row.getByRole('button', { name: /^edit$/i }).click();
		const detailsResponse = await detailsResponsePromise;
		expect(detailsResponse.status()).toBe(200);

		dialog = page.getByRole('dialog', { name: /update recurring invoice/i });
		await expect(dialog).toBeVisible();
		await dialog.locator('#name').fill(updatedName);
		await dialog.locator('#frequency').selectOption('QUARTERLY');
		await dialog.locator('#payment_terms').fill('21');
		await dialog.locator('#end_date').fill('2027-03-31');
		await dialog.locator('#reference').fill(updatedReference);
		lineRow = dialog.locator('.line-row').first();
		await lineRow.locator('input').nth(0).fill(updatedLine);
		await lineRow.locator('input').nth(1).fill('3');
		await lineRow.locator('input').nth(2).fill('175.00');

		const updateResponsePromise = page.waitForResponse((response) => {
			const path = new URL(response.url()).pathname;
			return (
				response.request().method() === 'PUT' &&
				path.endsWith(`/recurring-invoices/${created.id}`)
			);
		});
		await dialog.getByRole('button', { name: /^save$/i }).click();
		const updateResponse = await updateResponsePromise;
		expect(updateResponse.status()).toBe(200);
		const updated = (await updateResponse.json()) as RecurringInvoiceResponse;
		expect(updated.name).toBe(updatedName);
		expect(updated.reference).toBe(updatedReference);
		expect(updated.frequency).toBe('QUARTERLY');
		expect(updated.payment_terms_days).toBe(21);
		expect(updated.lines?.[0]?.description).toBe(updatedLine);

		row = recurringRow(page, updatedName);
		await expect(row).toBeVisible({ timeout: 10000 });
		await expect(row).toContainText(updatedReference);
		await expect(row).toContainText(/quarterly/i);

		const pauseResponsePromise = page.waitForResponse((response) => {
			const path = new URL(response.url()).pathname;
			return (
				response.request().method() === 'POST' &&
				path.endsWith(`/recurring-invoices/${created.id}/pause`)
			);
		});
		await row.getByRole('button', { name: /^pause$/i }).click();
		const pauseResponse = await pauseResponsePromise;
		expect(pauseResponse.ok()).toBeTruthy();
		row = recurringRow(page, updatedName);
		await expect(row).toContainText(/paused/i, { timeout: 10000 });

		const resumeResponsePromise = page.waitForResponse((response) => {
			const path = new URL(response.url()).pathname;
			return (
				response.request().method() === 'POST' &&
				path.endsWith(`/recurring-invoices/${created.id}/resume`)
			);
		});
		await row.getByRole('button', { name: /^resume$/i }).click();
		const resumeResponse = await resumeResponsePromise;
		expect(resumeResponse.ok()).toBeTruthy();
		row = recurringRow(page, updatedName);
		await expect(row).toContainText(/active/i, { timeout: 10000 });

		page.once('dialog', (confirmDialog) => confirmDialog.accept());
		const deleteResponsePromise = page.waitForResponse((response) => {
			const path = new URL(response.url()).pathname;
			return (
				response.request().method() === 'DELETE' &&
				path.endsWith(`/recurring-invoices/${created.id}`)
			);
		});
		await row.getByRole('button', { name: /^delete$/i }).click();
		const deleteResponse = await deleteResponsePromise;
		expect(deleteResponse.ok()).toBeTruthy();
		await expect(recurringRow(page, updatedName)).toHaveCount(0, { timeout: 10000 });
	});
});
