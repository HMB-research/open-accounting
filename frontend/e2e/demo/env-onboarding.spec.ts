import { test, expect, type Locator } from '@playwright/test';
import { loginAsDemoEnv } from './env-utils';

type TenantResponse = {
	id: string;
	name: string;
	slug: string;
	onboarding_completed: boolean;
};

function responsePath(responseUrl: string) {
	return new URL(responseUrl).pathname;
}

async function clickWizardAction(wizard: Locator, name: RegExp) {
	const button = wizard.getByRole('button', { name }).first();
	await expect(button).toBeVisible({ timeout: 10000 });
	await expect(button).toBeEnabled();
	await button.click();
}

test.describe('Demo Environment - Onboarding Wizard', () => {
	test.describe.configure({ mode: 'serial' });

	test('creates a new organization and completes onboarding', async ({ page }, testInfo) => {
		await loginAsDemoEnv(page, testInfo);

		const suffix = `${testInfo.parallelIndex}-${testInfo.repeatEachIndex}-${testInfo.retry}-${Date.now().toString(36)}`;
		const orgName = `E2E Onboarding ${suffix}`;
		const slug = `e2e-onboarding-${suffix}`;
		const updatedName = `${orgName} Updated`;

		await page.getByRole('button', { name: /new organization/i }).click();

		const createDialog = page.getByRole('dialog', { name: /create organization/i });
		await expect(createDialog).toBeVisible();
		await createDialog.locator('#name').fill(orgName);
		await expect(createDialog.locator('#slug')).toHaveValue(slug);

		const createResponsePromise = page.waitForResponse(
			(response) =>
				response.request().method() === 'POST' &&
				responsePath(response.url()) === '/api/v1/tenants'
		);
		await createDialog.getByRole('button', { name: /^create$/i }).click();
		const createResponse = await createResponsePromise;
		const createPayload = await createResponse.json();
		expect(createResponse.status(), JSON.stringify(createPayload)).toBe(201);

		const createdTenant = createPayload as TenantResponse;
		expect(createdTenant.name).toBe(orgName);
		expect(createdTenant.slug).toBe(slug);
		expect(createdTenant.onboarding_completed).toBe(false);

		const workspaceHero = page.locator('.workspace-hero');
		await expect(workspaceHero.getByRole('heading', { name: orgName })).toBeVisible({
			timeout: 10000
		});
		await expect(workspaceHero.getByText('/' + slug)).toBeVisible();

		await workspaceHero.getByRole('button', { name: /continue guided setup/i }).click();

		const wizard = page.locator('.onboarding-wizard');
		await expect(wizard).toBeVisible({ timeout: 10000 });
		await expect(page.getByRole('heading', { name: /welcome to open accounting/i })).toBeVisible();
		await expect(wizard.getByText('Company', { exact: true }).first()).toBeVisible();
		await expect(wizard.getByText('Branding', { exact: true }).first()).toBeVisible();
		await expect(wizard.getByText('Contact', { exact: true }).first()).toBeVisible();
		await expect(wizard.getByText('Done', { exact: true }).first()).toBeVisible();

		await expect(page.getByRole('heading', { name: /company information/i })).toBeVisible();
		await page.locator('#companyName').fill(updatedName);
		await page.locator('#regCode').fill('12345678');
		await page.locator('#vatNumber').fill('EE123456789');
		await page.locator('#email').fill(`onboarding-${suffix}@example.com`);
		await page.locator('#phone').fill('+372 5555 5555');
		await page.locator('#address').fill('Tartu 1, Tallinn, Estonia');

		const tenantPath = `/api/v1/tenants/${createdTenant.id}`;
		const companyUpdatePromise = page.waitForResponse(
			(response) =>
				response.request().method() === 'PUT' && responsePath(response.url()) === tenantPath
		);
		await clickWizardAction(wizard, /continue/i);
		const companyUpdate = await companyUpdatePromise;
		expect(companyUpdate.ok()).toBe(true);
		const savedCompany = (await companyUpdate.json()) as TenantResponse;
		expect(savedCompany.name).toBe(updatedName);
		await expect(page.getByRole('heading', { name: /branding & invoice settings/i })).toBeVisible();

		await page.locator('#bankDetails').fill('LHV Pank EE121212121212121212');
		await page.locator('#invoiceTerms').fill('Payment due within 14 days');

		const brandingUpdatePromise = page.waitForResponse(
			(response) =>
				response.request().method() === 'PUT' && responsePath(response.url()) === tenantPath
		);
		await clickWizardAction(wizard, /continue/i);
		const brandingUpdate = await brandingUpdatePromise;
		expect(brandingUpdate.ok()).toBe(true);
		await expect(page.getByRole('heading', { name: /add your first contact/i })).toBeVisible();

		await page.locator('#contactName').fill(`Onboarding Contact ${suffix}`);
		await page.locator('#contactEmail').fill(`contact-${suffix}@example.com`);

		const contactCreatePromise = page.waitForResponse(
			(response) =>
				response.request().method() === 'POST' &&
				responsePath(response.url()) === `${tenantPath}/contacts`
		);
		await clickWizardAction(wizard, /add.*continue|continue/i);
		const contactCreate = await contactCreatePromise;
		expect(contactCreate.status()).toBe(201);
		await expect(page.getByRole('heading', { name: /you're all set/i })).toBeVisible();

		const completePromise = page.waitForResponse(
			(response) =>
				response.request().method() === 'POST' &&
				responsePath(response.url()) === `${tenantPath}/complete-onboarding`
		);
		const tenantReloadPromise = page.waitForResponse(
			(response) =>
				response.request().method() === 'GET' && responsePath(response.url()) === tenantPath
		);
		await clickWizardAction(wizard, /go to dashboard/i);
		const completeResponse = await completePromise;
		expect(completeResponse.ok()).toBe(true);
		const tenantReload = await tenantReloadPromise;
		expect(tenantReload.ok()).toBe(true);
		const completedTenant = (await tenantReload.json()) as TenantResponse;
		expect(completedTenant.onboarding_completed).toBe(true);

		await expect(wizard).not.toBeVisible({ timeout: 10000 });
		await expect(page.getByRole('heading', { name: updatedName })).toBeVisible();
		await expect(page.getByText(/workspace ready/i)).toBeVisible();
		await expect(page.getByRole('link', { name: /new invoice/i })).toBeVisible();
	});
});
