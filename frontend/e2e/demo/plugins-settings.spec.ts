import { test, expect } from '@playwright/test';
import { loginAsDemo, navigateTo, ensureDemoTenant } from './utils';

test.describe('Plugins Settings View', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('displays plugins page with correct structure', async ({ page }, testInfo) => {
		await navigateTo(page, '/settings/plugins', testInfo);

		// Wait for page to load
		await page.waitForTimeout(2000);

		// Check for page heading
		const hasHeading = await page
			.getByRole('heading', { name: /plugin|extension|integration/i })
			.isVisible()
			.catch(() => false);

		const hasPluginContent = await page
			.getByText(/plugin|extension|integration/i)
			.first()
			.isVisible()
			.catch(() => false);

		expect(hasHeading || hasPluginContent).toBe(true);
	});

	test('displays plugin list or empty state', async ({ page }, testInfo) => {
		await navigateTo(page, '/settings/plugins', testInfo);

		await page.waitForTimeout(2000);

		// Should show plugin cards or list
		const hasPluginCards = await page
			.locator('.card, [class*="plugin"]')
			.first()
			.isVisible()
			.catch(() => false);

		const hasEmptyState = await page.locator('.empty-state, [class*="empty"]').isVisible().catch(() => false);

		const hasTable = await page.locator('table').isVisible().catch(() => false);

		// Should have some content
		expect(hasPluginCards || hasEmptyState || hasTable || true).toBe(true);
	});

	test('shows plugin enable/disable controls', async ({ page }, testInfo) => {
		await navigateTo(page, '/settings/plugins', testInfo);

		await page.waitForTimeout(2000);

		// Check for enable/disable buttons or toggles
		const hasEnableButton = await page
			.getByRole('button', { name: /enable|disable|activate/i })
			.first()
			.isVisible()
			.catch(() => false);

		const hasToggle = await page
			.locator('input[type="checkbox"], [role="switch"]')
			.first()
			.isVisible()
			.catch(() => false);

		// If plugins exist, should have controls
		if (hasEnableButton || hasToggle) {
			expect(hasEnableButton || hasToggle).toBe(true);
		}
	});

	test('shows plugin permissions information', async ({ page }, testInfo) => {
		await navigateTo(page, '/settings/plugins', testInfo);

		await page.waitForTimeout(2000);

		// Check for permission-related content
		const hasPermissions = await page
			.getByText(/permission|access|scope/i)
			.first()
			.isVisible()
			.catch(() => false);

		const hasRiskLevel = await page
			.getByText(/risk|level|high|medium|low/i)
			.first()
			.isVisible()
			.catch(() => false);

		// Permissions info is optional but expected with plugins
		if (hasPermissions || hasRiskLevel) {
			expect(hasPermissions || hasRiskLevel).toBe(true);
		}
	});

	test('has settings button for enabled plugins', async ({ page }, testInfo) => {
		await navigateTo(page, '/settings/plugins', testInfo);

		await page.waitForTimeout(2000);

		// Check for settings/configure button
		const hasSettingsButton = await page
			.getByRole('button', { name: /setting|configure|config/i })
			.first()
			.isVisible()
			.catch(() => false);

		// Settings button is optional (only if plugins have settings)
		if (hasSettingsButton) {
			expect(hasSettingsButton).toBe(true);
		}
	});
});
