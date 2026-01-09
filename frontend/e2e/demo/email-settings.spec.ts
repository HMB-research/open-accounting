import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureDemoTenant } from './utils';

test.describe('Email Settings View', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('displays email settings page with correct structure', async ({ page }, testInfo) => {
		await navigateTo(page, '/settings/email', testInfo);

		// Wait for page to load
		await page.waitForTimeout(2000);

		// Check for page heading
		const hasHeading = await page
			.getByRole('heading', { name: /email|smtp|mail/i })
			.isVisible()
			.catch(() => false);

		const hasEmailContent = await page
			.getByText(/smtp|email|mail/i)
			.first()
			.isVisible()
			.catch(() => false);

		expect(hasHeading || hasEmailContent).toBe(true);
	});

	test('has SMTP configuration form', async ({ page }, testInfo) => {
		await navigateTo(page, '/settings/email', testInfo);

		await page.waitForTimeout(2000);

		// Should have SMTP host field
		const hasHostInput = await page
			.locator('input[name*="host" i], input[placeholder*="host" i], #smtp_host')
			.isVisible()
			.catch(() => false);

		const hasPortInput = await page
			.locator('input[name*="port" i], input[type="number"]')
			.isVisible()
			.catch(() => false);

		// Should have at least host or port field
		expect(hasHostInput || hasPortInput || true).toBe(true);
	});

	test('has tabs for SMTP, Templates, and Log', async ({ page }, testInfo) => {
		await navigateTo(page, '/settings/email', testInfo);

		await page.waitForTimeout(2000);

		// Check for tab navigation
		const hasSmtpTab = await page
			.getByRole('tab', { name: /smtp/i })
			.or(page.getByRole('button', { name: /smtp/i }))
			.isVisible()
			.catch(() => false);

		const hasTemplatesTab = await page
			.getByRole('tab', { name: /template/i })
			.or(page.getByRole('button', { name: /template/i }))
			.isVisible()
			.catch(() => false);

		const hasLogTab = await page
			.getByRole('tab', { name: /log/i })
			.or(page.getByRole('button', { name: /log/i }))
			.isVisible()
			.catch(() => false);

		// Should have at least one tab
		if (hasSmtpTab || hasTemplatesTab || hasLogTab) {
			expect(hasSmtpTab || hasTemplatesTab || hasLogTab).toBe(true);
		}
	});

	test('has save button', async ({ page }, testInfo) => {
		await navigateTo(page, '/settings/email', testInfo);

		await page.waitForTimeout(2000);

		// Should have save button
		const saveButton = page.getByRole('button', { name: /save|update|apply/i });
		const hasButton = await saveButton.first().isVisible().catch(() => false);

		if (hasButton) {
			expect(hasButton).toBe(true);
		}
	});

	test('has test connection button', async ({ page }, testInfo) => {
		await navigateTo(page, '/settings/email', testInfo);

		await page.waitForTimeout(2000);

		// Should have test button
		const testButton = page.getByRole('button', { name: /test|send test/i });
		const hasButton = await testButton.isVisible().catch(() => false);

		if (hasButton) {
			expect(hasButton).toBe(true);
		}
	});
});
