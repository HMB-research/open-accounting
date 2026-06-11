import { expect, type Locator, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, ensureDemoTenant, navigateTo, waitForRouteReady } from './utils';

const invoicesLoadedState = 'table tbody tr, .empty-state, .alert-error';
const contactsRequestPattern = /\/api\/v1\/tenants\/[^/]+\/contacts$/;

export interface ContactResponse {
	id: string;
	name: string;
	email?: string;
}

export async function setupInvoicesPage(page: Page, testInfo: TestInfo): Promise<void> {
	await ensureAuthenticated(page, testInfo);
	await ensureDemoTenant(page, testInfo);
	await openInvoicesPage(page, testInfo);
}

export async function openInvoicesPage(page: Page, testInfo: TestInfo): Promise<void> {
	await navigateTo(page, '/invoices', testInfo, { waitForNetworkIdle: false });
	await waitForInvoicesLoaded(page);
}

export async function waitForInvoicesLoaded(page: Page): Promise<void> {
	await waitForRouteReady(
		page,
		'main h1, .workflow-hero, .filters, table, .empty-state, .alert-error',
		15000
	);
	await page
		.getByText(/^Loading\.\.\.$|^Laadimine\.\.\.$/i)
		.waitFor({ state: 'hidden', timeout: 15000 })
		.catch(() => {});
	await waitForRouteReady(page, invoicesLoadedState, 15000);
}

export function newInvoiceButton(page: Page): Locator {
	return page.getByRole('button', { name: /new invoice|uus arve|\+/i }).first();
}

export function invoiceModal(page: Page): Locator {
	return page
		.locator('[role="dialog"], .modal')
		.filter({ has: page.locator('#create-invoice-title') })
		.first();
}

export function contactModal(page: Page): Locator {
	return page
		.locator('[role="dialog"], .modal')
		.filter({ has: page.locator('#create-contact-title') })
		.first();
}

export async function openNewInvoiceModal(page: Page): Promise<Locator> {
	const createButton = newInvoiceButton(page);
	await expect(createButton).toBeVisible({ timeout: 10000 });
	await createButton.click();

	const modal = invoiceModal(page);
	await expect(modal).toBeVisible({ timeout: 5000 });
	return modal;
}

export async function openInlineContactModal(page: Page): Promise<Locator> {
	const modal = await openNewInvoiceModal(page);
	const newContactButton = modal.locator('.btn-new-contact');
	await expect(newContactButton).toBeVisible({ timeout: 5000 });
	await newContactButton.click();

	const contact = contactModal(page);
	await expect(contact).toBeVisible({ timeout: 5000 });
	return contact;
}

export async function createInlineContact(
	page: Page,
	name = `Inline Contact ${Date.now()}`
): Promise<ContactResponse> {
	const modal = contactModal(page);
	await expect(modal).toBeVisible({ timeout: 5000 });

	const unique = Date.now();
	await modal.locator('#contact-name').fill(name);
	await modal.locator('#contact-email').fill(`inline-${unique}@example.com`);

	const createResponsePromise = page.waitForResponse((response) => {
		const url = new URL(response.url());
		return (
			response.request().method() === 'POST' &&
			contactsRequestPattern.test(url.pathname) &&
			response.ok()
		);
	});

	await modal.getByRole('button', { name: /create|loo/i }).click();
	const response = await createResponsePromise;
	const contact = (await response.json()) as ContactResponse;

	await expect(contactModal(page)).not.toBeVisible({ timeout: 10000 });
	await expect(invoiceModal(page).locator('#contact')).toHaveValue(contact.id, { timeout: 5000 });
	await expect(invoiceModal(page).locator('#contact')).toContainText(contact.name);
	return contact;
}
