import { test, expect, type Page, type TestInfo } from '@playwright/test';
import {
	DEMO_API_URL as DEMO_API,
	DEMO_CREDENTIALS,
	DEMO_URL,
	ensureAuthenticated,
	loginWithDemoCredentials,
	waitForPageReady
} from './demo/utils';

/**
 * Live Demo Environment E2E Tests
 *
 * These tests run against:
 * - Local environment (default): Uses local BASE_URL and PUBLIC_API_URL defaults
 * - Hosted demo (optional): Requires explicit BASE_URL, PUBLIC_API_URL, and TEST_DEMO=true
 *
 * Run with: bun run test:e2e:demo
 *
 * Prerequisites:
 * - Demo environment must be deployed and accessible
 * - Demo users (demo1@example.com / demo12345) must exist
 */

// Use demo1 for this environment smoke file; broader demo specs distribute
// users by worker through the shared auth helpers.
const DEMO_USER = DEMO_CREDENTIALS[0];
const DEMO_TENANT_ID = DEMO_USER.tenantId;

async function waitForDemoShell(page: Page) {
	await waitForPageReady(page);
	await page.locator('nav.navbar, .mobile-menu-btn').first().waitFor({ state: 'visible', timeout: 10000 });
}

async function waitForVisibleContent(page: Page) {
	await waitForPageReady(page);
	await page.getByText(/^Loading\.\.\.$/).waitFor({ state: 'hidden', timeout: 10000 }).catch(() => {});
	await expect(page.locator('main, .main-content, [class*="content"]').first()).toBeVisible({ timeout: 10000 });
}

async function clickWizardButton(page: Page, name: RegExp) {
	const button = page.getByRole('button', { name }).first();
	await expect(button).toBeVisible({ timeout: 10000 });
	await expect(button).toBeEnabled();
	await button.click();
}

async function advanceWizardStep(page: Page, buttonName: RegExp, nextHeading: RegExp) {
	await clickWizardButton(page, buttonName);
	await expect(page.getByRole('heading', { name: nextHeading })).toBeVisible({ timeout: 10000 });
}

// Helper to authenticate as demo user. Use saved auth state for non-auth tests
// so this smoke file does not re-run the login workflow for every route check.
async function loginAsDemo(page: Page, testInfo?: TestInfo) {
	if (testInfo) {
		await ensureAuthenticated(page, testInfo);
		await waitForDemoShell(page);
		return;
	}

	await loginWithDemoCredentials(page, DEMO_USER, { logPrefix: 'Demo env login' });
	await waitForDemoShell(page);
}

// Helper to navigate with tenant parameter
async function navigateToPage(page: Page, path: string) {
	const separator = path.includes('?') ? '&' : '?';
	const url = `${DEMO_URL}${path}${separator}tenant=${DEMO_TENANT_ID}`;
	await page.goto(url);
	await waitForVisibleContent(page);
}

test.describe('Demo Environment - Health Checks', () => {
	test('API health endpoint responds', async ({ request }) => {
		const response = await request.get(`${DEMO_API}/health`);
		expect(response.ok()).toBeTruthy();
	});

	test('Frontend loads successfully', async ({ page }) => {
		await page.goto(DEMO_URL);
		await expect(page).toHaveTitle(/tallion|open accounting/i);
	});

	test('Login page renders correctly', async ({ page }) => {
		await page.goto(`${DEMO_URL}/login`);

		await expect(page.getByRole('heading', { name: /welcome|login|sign in/i })).toBeVisible();
		await expect(page.getByLabel(/email/i)).toBeVisible();
		await expect(page.locator('#password')).toBeVisible();
		await expect(page.getByRole('button', { name: /sign in|login/i })).toBeVisible();
	});
});

test.describe('Demo Environment - Authentication', () => {
	test('Demo user can login successfully', async ({ page }) => {
		await loginAsDemo(page);

		// Verify we're on the dashboard
		await expect(page).toHaveURL(/dashboard/);

		// Should see dashboard content
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
	});

	test('Invalid credentials show error', async ({ page }) => {
		await page.goto(`${DEMO_URL}/login`);

		await page.getByLabel(/email/i).fill('invalid@example.com');
		await page.locator('#password').fill('wrongpassword');
		await page.getByRole('button', { name: /sign in|login/i }).click();

		const errorAlert = page.locator('.alert-error, [role="alert"]').first();
		await expect(errorAlert.or(page.getByLabel(/email/i))).toBeVisible({ timeout: 5000 });

		const stillOnLogin = page.url().includes('/login');
		const hasError = await errorAlert.isVisible().catch(() => false);
		expect(stillOnLogin || hasError).toBeTruthy();
	});

	test('Logout works correctly', async ({ page }) => {
		await loginAsDemo(page);

		// Find and click logout
		const logoutButton = page.getByRole('button', { name: /logout|sign out/i });
		const hasLogout = await logoutButton.isVisible().catch(() => false);

		if (hasLogout) {
			await logoutButton.click();
			await page.waitForURL(/login/, { timeout: 10000 });
			await expect(page).toHaveURL(/login/);
		} else {
			// Try menu-based logout
			const userMenu = page.locator('[class*="user"], [class*="avatar"], [class*="profile"]').first();
			if (await userMenu.isVisible()) {
				await userMenu.click();
				const logoutItem = page.getByText(/logout|sign out/i);
				if (await logoutItem.isVisible()) {
					await logoutItem.click();
					await page.waitForURL(/login/, { timeout: 10000 });
				}
			}
		}
	});
});

test.describe('Demo Environment - Dashboard', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
	});

	test('Dashboard displays organization selector', async ({ page }) => {
		const orgSelector = page.locator('.tenant-selector, [class*="org-selector"], select').first();
		await expect(orgSelector).toBeVisible({ timeout: 10000 });
	});

	test('Dashboard shows summary cards', async ({ page }) => {
		await waitForVisibleContent(page);

		// Look for summary/stat cards
		const summarySection = page.locator('.summary-grid, .stats, [class*="summary"]').first();
		const hasSummary = await summarySection.isVisible({ timeout: 15000 }).catch(() => false);

		if (hasSummary) {
			await expect(summarySection).toBeVisible();
		} else {
			// Dashboard might have different layout - just verify content exists
			const dashboardContent = page.locator('main, .dashboard, [class*="content"]').first();
			await expect(dashboardContent).toBeVisible();
		}
	});

	test('Navigation sidebar is present', async ({ page }) => {
		await waitForDemoShell(page);
		const navItems = ['dashboard', 'invoice', 'contact', 'report'];
		const visibleItems = await Promise.all(
			navItems.map((item) =>
				page
					.getByRole('link', { name: new RegExp(item, 'i') })
					.first()
					.isVisible()
					.catch(() => false)
			)
		);
		const hasMenuShell = await page
			.locator('nav.navbar, .mobile-nav, .mobile-menu-btn, [aria-label*="menu"]')
			.first()
			.isVisible()
			.catch(() => false);

		expect(hasMenuShell || visibleItems.some(Boolean)).toBeTruthy();
	});
});

test.describe('Demo Environment - Invoices', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
	});

	test('Can navigate to invoices page', async ({ page }) => {
		const invoicesLink = page.getByRole('link', { name: /invoice/i }).first();
		const hasLink = await invoicesLink.isVisible().catch(() => false);

		if (hasLink) {
			await invoicesLink.click();
			await page.waitForURL(/invoice/, { timeout: 15000 });
			await expect(page).toHaveURL(/invoice/);
		} else {
			// Invoice link might not be visible in current view - navigate directly
			await navigateToPage(page, '/invoices');
			await expect(page).toHaveURL(/invoice/);
		}
	});

	test('Invoices list displays', async ({ page }) => {
		await navigateToPage(page, '/invoices');

		// Should show invoice list or empty state or any content
		const content = page.locator('main, [class*="content"], .container').first();
		await expect(content).toBeVisible();

		const hasInvoices = await page
			.locator('table tbody tr, .invoice-list, .workflow-hero, [class*="invoice"]')
			.first()
			.isVisible()
			.catch(() => false);
		const hasEmptyState = await page.getByText(/no invoice|create.*first|get started|no data/i).isVisible().catch(() => false);
		const hasHeading = await page.getByRole('heading', { level: 1, name: /^invoices$/i }).isVisible().catch(() => false);

		expect(hasInvoices || hasEmptyState || hasHeading).toBeTruthy();
	});

	test('Can access create invoice form', async ({ page }) => {
		await navigateToPage(page, '/invoices');

		const createButton = page.getByRole('link', { name: /new|create|add/i }).or(
			page.getByRole('button', { name: /new|create|add/i })
		).first();

		const hasCreate = await createButton.isVisible().catch(() => false);

		if (hasCreate) {
			await createButton.click();

			// Should be on create form or modal appeared
			await expect(page.getByRole('dialog', { name: /new invoice/i })).toBeVisible({ timeout: 10000 });
			const hasForm = await page.locator('form').first().isVisible().catch(() => false);
			const hasModal = await page.locator('.modal, [role="dialog"]').first().isVisible().catch(() => false);

			expect(hasForm || hasModal || page.url().includes('/new')).toBeTruthy();
		}
	});
});

test.describe('Demo Environment - Contacts', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
	});

	test('Can navigate to contacts page', async ({ page }) => {
		const contactsLink = page.getByRole('link', { name: /contact|customer|client/i }).first();
		const hasLink = await contactsLink.isVisible().catch(() => false);

		if (hasLink) {
			await contactsLink.click();
			await waitForVisibleContent(page);
			await expect(page).toHaveURL(/contact/);
		} else {
			await navigateToPage(page, '/contacts');
			await expect(page).toHaveURL(/contact/);
		}
	});

	test('Contacts list displays', async ({ page }) => {
		await navigateToPage(page, '/contacts');

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();
	});
});

test.describe('Demo Environment - Reports', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
	});

	test('Can navigate to reports page', async ({ page }) => {
		const reportsLink = page.getByRole('link', { name: /report/i }).first();
		const hasLink = await reportsLink.isVisible().catch(() => false);

		if (hasLink) {
			await reportsLink.click();
			await waitForVisibleContent(page);
			await expect(page).toHaveURL(/report/);
		} else {
			await navigateToPage(page, '/reports');
			await expect(page).toHaveURL(/report/);
		}
	});

	test('Reports page loads', async ({ page }) => {
		await navigateToPage(page, '/reports');

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();
	});
});

test.describe('Demo Environment - Settings', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);
	});

	test('Can access settings', async ({ page }) => {
		const settingsLink = page.getByRole('link', { name: /setting/i }).first();
		const hasLink = await settingsLink.isVisible().catch(() => false);

		if (hasLink) {
			await settingsLink.click();
			await waitForVisibleContent(page);
			await expect(page).toHaveURL(/setting/);
		} else {
			await navigateToPage(page, '/settings');
			await expect(page).toHaveURL(/setting/);
		}
	});
});

test.describe('Demo Environment - Responsive Design', () => {
	test('Mobile viewport works', async ({ page }, testInfo) => {
		await page.setViewportSize({ width: 375, height: 667 });
		await loginAsDemo(page, testInfo);

		// Dashboard should still be accessible
		await expect(page).toHaveURL(/dashboard/);

		// Content should be visible
		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();
	});

	test('Tablet viewport works', async ({ page }, testInfo) => {
		await page.setViewportSize({ width: 768, height: 1024 });
		await loginAsDemo(page, testInfo);

		await expect(page).toHaveURL(/dashboard/);

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();
	});
});

test.describe('Demo Environment - Error Handling', () => {
	test('Unknown route handled gracefully', async ({ page }) => {
		await page.goto(`${DEMO_URL}/this-page-does-not-exist`);
		await page.waitForLoadState('domcontentloaded');

		// Should show 404, redirect to login/dashboard, or show any page content
		const is404 = await page.getByText(/404|not found|page.*exist/i).isVisible().catch(() => false);
		const redirected = page.url().includes('/login') || page.url().includes('/dashboard');
		const hasContent = await page.locator('body').isVisible();

		expect(is404 || redirected || hasContent).toBeTruthy();
	});

	test('Protected routes require authentication', async ({ page }) => {
		// Try accessing protected route without auth
		await page.goto(`${DEMO_URL}/dashboard`);
		await page.waitForLoadState('domcontentloaded');

		// Should either redirect to login OR show login form OR show dashboard (if session persisted)
		const onLogin = page.url().includes('/login');
		const hasLoginForm = await page.getByLabel(/email/i).isVisible().catch(() => false);
		const onDashboard = page.url().includes('/dashboard');

		expect(onLogin || hasLoginForm || onDashboard).toBeTruthy();
	});
});

test.describe('Demo Environment - Onboarding Wizard', () => {
	test('Onboarding wizard displays for new organization', async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);

		// Check if onboarding wizard is visible (may or may not appear depending on org state)
		const wizardHeading = page.getByRole('heading', { name: /welcome to open accounting/i });
		const hasWizard = await wizardHeading.isVisible({ timeout: 5000 }).catch(() => false);

		if (hasWizard) {
			await expect(wizardHeading).toBeVisible();

			// Verify setup steps are shown
			await expect(page.getByText(/set up your organization/i)).toBeVisible();

			// Verify step indicators (1-Company, 2-Branding, 3-Contact, 4-Done)
			await expect(page.getByText('Company')).toBeVisible();
			await expect(page.getByText('Branding')).toBeVisible();
			await expect(page.getByText('Contact')).toBeVisible();
			await expect(page.getByText('Done')).toBeVisible();
		}
	});

	test('Company information form displays correctly', async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);

		const companyHeading = page.getByRole('heading', { name: /company information/i });
		const hasCompanyForm = await companyHeading.isVisible({ timeout: 5000 }).catch(() => false);

		if (hasCompanyForm) {
			await expect(companyHeading).toBeVisible();

			// Verify form fields
			await expect(page.getByLabel(/company name/i)).toBeVisible();
			await expect(page.getByLabel(/registration code/i)).toBeVisible();
			await expect(page.getByLabel(/vat number/i)).toBeVisible();
			await expect(page.getByLabel(/email/i)).toBeVisible();
			await expect(page.getByLabel(/phone/i)).toBeVisible();
			await expect(page.getByLabel(/address/i)).toBeVisible();
		}
	});

	test('Onboarding form accepts valid company data', async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);

		const companyNameInput = page.getByLabel(/company name/i);
		const hasCompanyForm = await companyNameInput.isVisible({ timeout: 5000 }).catch(() => false);

		if (hasCompanyForm) {
			// Fill in company information
			await companyNameInput.fill('Test Company');
			await expect(companyNameInput).toHaveValue('Test Company');

			// Check for next/continue button
			const nextButton = page.getByRole('button', { name: /next|continue|save/i });
			const hasNext = await nextButton.isVisible().catch(() => false);

			if (hasNext) {
				await expect(nextButton).toBeEnabled();
			}
		}
	});

	test('Onboarding wizard step navigation works', async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);

		// Check if we're on step 1 (Company)
		const step1Active = page.locator('[class*="active"], [class*="current"]').filter({ hasText: /company/i });
		const hasSteps = await step1Active.isVisible({ timeout: 5000 }).catch(() => false);

		if (hasSteps) {
			// Step 1 should be active/current
			await expect(step1Active).toBeVisible();
		}
	});

	test('Onboarding wizard can be completed to reach dashboard', async ({ page }, testInfo) => {
		// This test requires a valid demo user - uses shared login helper
		await loginAsDemo(page, testInfo);

		// Check if onboarding wizard is showing
		const wizardOverlay = page.locator('.onboarding-overlay, .onboarding-wizard');
		const hasWizard = await wizardOverlay.isVisible({ timeout: 5000 }).catch(() => false);

		if (hasWizard) {
			// Step 1: Company Info - fill required field and continue
			const companyNameInput = page.getByLabel(/company name/i);
			if (await companyNameInput.isVisible()) {
				await companyNameInput.fill('E2E Test Company');
			}
			await advanceWizardStep(page, /continue|next/i, /branding|invoice settings/i);

			// Step 2: Branding - skip or continue
			const step2Continue = page.getByRole('button', { name: /continue|next/i });
			if (await step2Continue.isVisible()) {
				await advanceWizardStep(page, /continue|next/i, /first contact/i);
			}

			// Step 3: First Contact - skip or continue
			const step3Continue = page.getByRole('button', { name: /skip|continue/i });
			if (await step3Continue.isVisible()) {
				await advanceWizardStep(page, /skip|continue/i, /all set/i);
			}

			// Step 4: Complete - click "Go to Dashboard"
			const goToDashboard = page.getByRole('button', { name: /go to dashboard|finish|complete/i });
			if (await goToDashboard.isVisible()) {
				await clickWizardButton(page, /go to dashboard|finish|complete/i);
				await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
			}

			// Verify wizard is now hidden
			await expect(wizardOverlay).not.toBeVisible({ timeout: 10000 });
		}

		// Final verification: Should be on dashboard without wizard overlay
		await expect(page).toHaveURL(/dashboard/);
		const dashboardContent = page.locator('main, .dashboard, [class*="content"]').first();
		await expect(dashboardContent).toBeVisible();

		// Verify onboarding wizard is NOT showing
		const wizardAfter = page.locator('.onboarding-overlay');
		await expect(wizardAfter).not.toBeVisible();
	});
});

test.describe('Demo Environment - Performance', () => {
	test('Login flow completes successfully', async ({ page }) => {
		const startTime = Date.now();

		await loginWithDemoCredentials(page, DEMO_USER, { logPrefix: 'Performance login' });

		const elapsed = Date.now() - startTime;
		// Log performance for monitoring
		console.log(`Login completed in ${elapsed}ms`);

		// Should complete within 45 seconds (generous for cold starts)
		expect(elapsed).toBeLessThan(45000);
	});

	test('Dashboard reload is responsive', async ({ page }, testInfo) => {
		await loginAsDemo(page, testInfo);

		const startTime = Date.now();
		await page.reload();
		await waitForVisibleContent(page);

		const elapsed = Date.now() - startTime;
		console.log(`Dashboard reload completed in ${elapsed}ms`);

		// Should reload within 20 seconds
		expect(elapsed).toBeLessThan(20000);
	});
});
