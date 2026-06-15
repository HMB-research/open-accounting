import { test, expect } from '@playwright/test';
import {
	createInlineContact,
	invoiceModal,
	openInlineContactModal,
	setupInvoicesPage
} from './invoices-utils';

test.describe('Demo Invoices - Inline Contact', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await setupInvoicesPage(page, testInfo);
	});

	test('opens contact modal, creates a contact, and selects it in the invoice form', async ({
		page
	}) => {
		const contactModal = await openInlineContactModal(page);
		await expect(contactModal.locator('#contact-name')).toBeVisible({ timeout: 5000 });
		await expect(contactModal.locator('#contact-email')).toBeVisible();

		const contact = await createInlineContact(page);
		await expect(invoiceModal(page).locator('#contact')).toContainText(contact.name);
	});
});
