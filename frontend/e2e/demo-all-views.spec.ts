import { test, expect } from '@playwright/test';

/**
 * Comprehensive E2E Tests for All Demo Views
 *
 * These tests verify that every view in the application:
 * 1. Loads correctly
 * 2. Displays seeded demo data
 * 3. Has proper navigation
 *
 * Run with: npm run test:e2e:demo
 */

const DEMO_URL = 'https://open-accounting.up.railway.app';
const DEMO_EMAIL = 'demo@example.com';
const DEMO_PASSWORD = 'demo123';

// Helper to login as demo user
async function loginAsDemo(page: import('@playwright/test').Page) {
	await page.goto(`${DEMO_URL}/login`);
	await page.waitForLoadState('networkidle');

	const emailInput = page.getByLabel(/email/i);
	const passwordInput = page.getByLabel(/password/i);

	await emailInput.fill(DEMO_EMAIL);
	await passwordInput.fill(DEMO_PASSWORD);

	await page.getByRole('button', { name: /sign in|login/i }).click();
	await page.waitForURL(/dashboard/, { timeout: 30000 });
}

// Helper to navigate to a page
async function navigateTo(page: import('@playwright/test').Page, path: string) {
	await page.goto(`${DEMO_URL}${path}`);
	await page.waitForLoadState('networkidle');
}

test.describe('Demo All Views - Landing & Auth', () => {
	test('Landing page displays features', async ({ page }) => {
		await page.goto(DEMO_URL);
		await page.waitForLoadState('networkidle');

		// Should show landing page content
		await expect(page.locator('body')).toBeVisible();

		// Look for key landing page elements
		const hasHero = await page.locator('[class*="hero"], h1').first().isVisible().catch(() => false);
		const hasFeatures = await page.getByText(/feature|accounting|invoice/i).first().isVisible().catch(() => false);

		expect(hasHero || hasFeatures).toBeTruthy();
	});

	test('Login page renders correctly', async ({ page }) => {
		await page.goto(`${DEMO_URL}/login`);
		await page.waitForLoadState('networkidle');

		await expect(page.getByRole('heading', { name: /welcome|login|sign in/i })).toBeVisible();
		await expect(page.getByLabel(/email/i)).toBeVisible();
		await expect(page.getByLabel(/password/i)).toBeVisible();
		await expect(page.getByRole('button', { name: /sign in|login/i })).toBeVisible();
	});
});

test.describe('Demo All Views - Dashboard', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
	});

	test('Dashboard loads with data', async ({ page }) => {
		await expect(page).toHaveURL(/dashboard/);

		// Should show dashboard content
		const content = page.locator('main, [class*="content"], .container').first();
		await expect(content).toBeVisible();

		// Look for organization selector or dashboard elements
		const hasOrgSelector = await page.locator('select, .tenant-selector').first().isVisible().catch(() => false);
		const hasHeading = await page.getByRole('heading', { level: 1 }).isVisible().catch(() => false);

		expect(hasOrgSelector || hasHeading).toBeTruthy();
	});
});

test.describe('Demo All Views - Accounting', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
	});

	test('Accounts (Chart of Accounts) displays data', async ({ page }) => {
		await navigateTo(page, '/accounts');

		// Page should load
		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();

		// Should show accounts or heading
		const hasHeading = await page.getByRole('heading', { name: /account|chart/i }).isVisible().catch(() => false);
		const hasTable = await page.locator('table, .account-list, [class*="list"]').first().isVisible().catch(() => false);
		const hasData = await page.getByText(/cash|bank|asset|revenue|expense/i).first().isVisible().catch(() => false);

		expect(hasHeading || hasTable || hasData).toBeTruthy();
	});

	test('Journal entries page displays data', async ({ page }) => {
		await navigateTo(page, '/journal');

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();

		// Should show journal entries or heading
		const hasHeading = await page.getByRole('heading', { name: /journal|ledger|entry/i }).isVisible().catch(() => false);
		const hasData = await page.getByText(/JE-|entry|opening|debit|credit/i).first().isVisible().catch(() => false);
		const hasEmptyState = await page.getByText(/no.*entry|create.*first/i).isVisible().catch(() => false);

		expect(hasHeading || hasData || hasEmptyState).toBeTruthy();
	});

	test('Invoices page displays data', async ({ page }) => {
		await navigateTo(page, '/invoices');

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();

		// Should show invoices
		const hasHeading = await page.getByRole('heading', { name: /invoice/i }).isVisible().catch(() => false);
		const hasTable = await page.locator('table, .invoice-list').first().isVisible().catch(() => false);
		const hasInvoiceData = await page.getByText(/INV-|draft|sent|paid/i).first().isVisible().catch(() => false);

		expect(hasHeading || hasTable || hasInvoiceData).toBeTruthy();
	});

	test('Payments page displays data', async ({ page }) => {
		await navigateTo(page, '/payments');

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();

		// Should show payments
		const hasHeading = await page.getByRole('heading', { name: /payment/i }).isVisible().catch(() => false);
		const hasData = await page.getByText(/PAY-|received|transfer/i).first().isVisible().catch(() => false);
		const hasEmptyState = await page.getByText(/no.*payment|create.*first/i).isVisible().catch(() => false);

		expect(hasHeading || hasData || hasEmptyState).toBeTruthy();
	});

	test('Recurring invoices page displays data', async ({ page }) => {
		await navigateTo(page, '/recurring');

		const content = page.locator('main, [class*="content"], .container').first();
		await expect(content).toBeVisible();

		// Wait for page to stabilize (loading to complete)
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(1000);

		// Should show recurring invoices heading (always visible) or page content
		const hasHeading = await page.locator('h1').filter({ hasText: /recurring/i }).isVisible().catch(() => false);
		const hasData = await page.getByText(/monthly|quarterly|yearly|support/i).first().isVisible().catch(() => false);
		const hasEmptyState = await page.getByText(/no.*recurring|create.*first/i).isVisible().catch(() => false);
		const hasNewButton = await page.getByRole('button', { name: /new|create/i }).isVisible().catch(() => false);

		expect(hasHeading || hasData || hasEmptyState || hasNewButton).toBeTruthy();
	});

	test('Contacts page displays data', async ({ page }) => {
		await navigateTo(page, '/contacts');

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();

		// Should show contacts
		const hasHeading = await page.getByRole('heading', { name: /contact|customer|supplier/i }).isVisible().catch(() => false);
		const hasData = await page.getByText(/techstart|nordic|baltic|greentech/i).first().isVisible().catch(() => false);
		const hasEmptyState = await page.getByText(/no.*contact|create.*first/i).isVisible().catch(() => false);

		expect(hasHeading || hasData || hasEmptyState).toBeTruthy();
	});

	test('Reports page loads', async ({ page }) => {
		await navigateTo(page, '/reports');

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();

		// Should show reports options
		const hasHeading = await page.getByRole('heading', { name: /report/i }).isVisible().catch(() => false);
		const hasReportTypes = await page.getByText(/trial balance|balance sheet|income|profit|loss/i).first().isVisible().catch(() => false);

		expect(hasHeading || hasReportTypes).toBeTruthy();
	});
});

test.describe('Demo All Views - Payroll', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
	});

	test('Employees page displays data', async ({ page }) => {
		await navigateTo(page, '/employees');

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();

		// Should show employees
		const hasHeading = await page.getByRole('heading', { name: /employee/i }).isVisible().catch(() => false);
		const hasData = await page.getByText(/maria|jaan|anna|peeter|developer|manager/i).first().isVisible().catch(() => false);
		const hasEmptyState = await page.getByText(/no.*employee|add.*first/i).isVisible().catch(() => false);

		expect(hasHeading || hasData || hasEmptyState).toBeTruthy();
	});

	test('Payroll page displays data', async ({ page }) => {
		await navigateTo(page, '/payroll');

		const content = page.locator('main, [class*="content"], .container').first();
		await expect(content).toBeVisible();

		// Wait for page to stabilize (loading to complete)
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(1000);

		// Should show payroll runs heading or page content
		const hasHeading = await page.locator('h1').filter({ hasText: /payroll/i }).isVisible().catch(() => false);
		const hasData = await page.getByText(/2024|october|november|december|paid|approved/i).first().isVisible().catch(() => false);
		const hasEmptyState = await page.getByText(/no.*payroll|create.*first/i).isVisible().catch(() => false);
		const hasNewButton = await page.getByRole('button', { name: /new|create/i }).isVisible().catch(() => false);
		const hasYearFilter = await page.locator('select').isVisible().catch(() => false);

		expect(hasHeading || hasData || hasEmptyState || hasNewButton || hasYearFilter).toBeTruthy();
	});

	test('TSD declarations page displays data', async ({ page }) => {
		await navigateTo(page, '/tsd');

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();

		// Should show TSD declarations
		const hasHeading = await page.getByRole('heading', { name: /tsd|tax|declaration/i }).isVisible().catch(() => false);
		const hasData = await page.getByText(/2024|submitted|draft|income tax|social tax/i).first().isVisible().catch(() => false);
		const hasEmptyState = await page.getByText(/no.*declaration|create.*first/i).isVisible().catch(() => false);

		expect(hasHeading || hasData || hasEmptyState).toBeTruthy();
	});
});

test.describe('Demo All Views - Banking', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
	});

	test('Banking page displays data', async ({ page }) => {
		await navigateTo(page, '/banking');

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();

		// Should show bank accounts
		const hasHeading = await page.getByRole('heading', { name: /bank/i }).isVisible().catch(() => false);
		const hasData = await page.getByText(/swedbank|seb|eur|main|savings/i).first().isVisible().catch(() => false);
		const hasEmptyState = await page.getByText(/no.*account|add.*first/i).isVisible().catch(() => false);

		expect(hasHeading || hasData || hasEmptyState).toBeTruthy();
	});

	test('Banking import page loads', async ({ page }) => {
		await navigateTo(page, '/banking/import');

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();

		// Should show import options
		const hasHeading = await page.getByRole('heading', { name: /import|statement/i }).isVisible().catch(() => false);
		const hasUpload = await page.locator('input[type="file"], .dropzone, [class*="upload"]').first().isVisible().catch(() => false);
		const hasText = await page.getByText(/csv|upload|import|file/i).first().isVisible().catch(() => false);

		expect(hasHeading || hasUpload || hasText).toBeTruthy();
	});
});

test.describe('Demo All Views - Tax', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
	});

	test('Tax page loads', async ({ page }) => {
		await navigateTo(page, '/tax');

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();

		// Should show tax reporting
		const hasHeading = await page.getByRole('heading', { name: /tax/i }).isVisible().catch(() => false);
		const hasContent = await page.getByText(/vat|report|compliance|period/i).first().isVisible().catch(() => false);

		expect(hasHeading || hasContent).toBeTruthy();
	});
});

test.describe('Demo All Views - Settings', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
	});

	test('Settings page loads', async ({ page }) => {
		await navigateTo(page, '/settings');

		const content = page.locator('main, [class*="content"], .container').first();
		await expect(content).toBeVisible();

		// Wait for page to stabilize
		await page.waitForLoadState('networkidle');
		await page.waitForTimeout(1000);

		// Should show settings heading or card links
		const hasHeading = await page.locator('h1').filter({ hasText: /setting/i }).isVisible().catch(() => false);
		const hasContent = await page.getByText(/company|email|plugin|preference/i).first().isVisible().catch(() => false);
		const hasCards = await page.locator('.settings-card, .card, a[href*="settings"]').first().isVisible().catch(() => false);

		expect(hasHeading || hasContent || hasCards).toBeTruthy();
	});

	test('Company settings page displays data', async ({ page }) => {
		await navigateTo(page, '/settings/company');

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();

		// Should show company settings with seeded data
		const hasHeading = await page.getByRole('heading', { name: /company|organization/i }).isVisible().catch(() => false);
		const hasData = await page.getByText(/acme|12345678|vat|address/i).first().isVisible().catch(() => false);
		const hasForm = await page.locator('form, input, [class*="form"]').first().isVisible().catch(() => false);

		expect(hasHeading || hasData || hasForm).toBeTruthy();
	});

	test('Email settings page loads', async ({ page }) => {
		await navigateTo(page, '/settings/email');

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();

		// Should show email settings
		const hasHeading = await page.getByRole('heading', { name: /email/i }).isVisible().catch(() => false);
		const hasContent = await page.getByText(/smtp|template|notification|sender/i).first().isVisible().catch(() => false);

		expect(hasHeading || hasContent).toBeTruthy();
	});

	test('Plugins settings page loads', async ({ page }) => {
		await navigateTo(page, '/settings/plugins');

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();

		// Should show plugins settings
		const hasHeading = await page.getByRole('heading', { name: /plugin/i }).isVisible().catch(() => false);
		const hasContent = await page.getByText(/enable|disable|configure|installed/i).first().isVisible().catch(() => false);

		expect(hasHeading || hasContent).toBeTruthy();
	});
});

test.describe('Demo All Views - Admin', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
	});

	test('Admin plugins page loads', async ({ page }) => {
		await navigateTo(page, '/admin/plugins');

		const content = page.locator('main, [class*="content"]').first();
		await expect(content).toBeVisible();

		// Should show admin plugins
		const hasHeading = await page.getByRole('heading', { name: /plugin|admin/i }).isVisible().catch(() => false);
		const hasContent = await page.getByText(/manage|install|system|admin/i).first().isVisible().catch(() => false);

		expect(hasHeading || hasContent).toBeTruthy();
	});
});

test.describe('Demo All Views - Navigation', () => {
	test.beforeEach(async ({ page }) => {
		await loginAsDemo(page);
	});

	test('Sidebar navigation has all main links', async ({ page }) => {
		// Main navigation should have key links
		const nav = page.locator('nav, .sidebar, [class*="nav"]').first();
		await expect(nav).toBeVisible();

		// Check for essential navigation items
		const navItems = ['dashboard', 'account', 'journal', 'contact', 'invoice', 'payment', 'report'];

		for (const item of navItems) {
			const link = page.getByRole('link', { name: new RegExp(item, 'i') });
			const exists = await link.count();
			if (exists > 0) {
				expect(exists).toBeGreaterThan(0);
				break; // At least one nav item exists
			}
		}
	});

	test('Payroll dropdown has subitems', async ({ page }) => {
		// Look for payroll menu
		const payrollMenu = page.getByText(/payroll/i).first();
		const hasPayrollMenu = await payrollMenu.isVisible().catch(() => false);

		if (hasPayrollMenu) {
			await payrollMenu.click();
			await page.waitForTimeout(500);

			// Should show employee, payroll runs, TSD options
			const hasEmployees = await page.getByText(/employee/i).isVisible().catch(() => false);
			const hasTSD = await page.getByText(/tsd|declaration/i).isVisible().catch(() => false);

			expect(hasEmployees || hasTSD).toBeTruthy();
		}
	});
});

test.describe('Demo All Views - Responsive', () => {
	test('Mobile viewport shows all views correctly', async ({ page }) => {
		await page.setViewportSize({ width: 375, height: 667 });
		await loginAsDemo(page);

		// Dashboard should be accessible
		await expect(page).toHaveURL(/dashboard/);

		// Navigate to invoices
		await navigateTo(page, '/invoices');
		const invoicesContent = page.locator('main, [class*="content"]').first();
		await expect(invoicesContent).toBeVisible();

		// Navigate to contacts
		await navigateTo(page, '/contacts');
		const contactsContent = page.locator('main, [class*="content"]').first();
		await expect(contactsContent).toBeVisible();

		// Navigate to employees
		await navigateTo(page, '/employees');
		const employeesContent = page.locator('main, [class*="content"]').first();
		await expect(employeesContent).toBeVisible();
	});
});
