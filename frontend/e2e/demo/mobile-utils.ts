import { expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, ensureDemoTenant, navigateTo, waitForRouteReady } from './utils';

const routeReadySelectors: Record<string, string> = {
	'/dashboard': 'main h1, .dashboard-header, [data-testid="dashboard"], .summary-grid, .tenant-selector',
	'/invoices': 'main h1, .workflow-hero, table, .empty-state',
	'/contacts': 'main h1, table, .empty-state',
	'/reports': 'main h1, .report-controls, .reports-grid'
};

export async function prepareMobileDemo(page: Page, testInfo: TestInfo): Promise<void> {
	await ensureAuthenticated(page, testInfo);
	await ensureDemoTenant(page, testInfo);
}

export async function openMobileRoute(page: Page, path: string, testInfo: TestInfo): Promise<void> {
	await navigateTo(page, path, testInfo, { waitForNetworkIdle: false });
	await waitForRouteReady(page, routeReadySelectors[path] ?? 'main h1, main, [role="main"]');
}

export async function openMobileDrawer(page: Page) {
	const mobileMenuButton = page.getByRole('button', {
		name: /toggle menu/i
	});

	await expect(mobileMenuButton).toBeVisible();
	await mobileMenuButton.click();

	const drawer = page.locator('.mobile-nav');
	await expect(drawer).toBeVisible();
	return drawer;
}

export async function expectNoHorizontalOverflow(page: Page): Promise<void> {
	const { scrollWidth, viewportWidth } = await page.evaluate(() => ({
		scrollWidth: Math.max(document.body.scrollWidth, document.documentElement.scrollWidth),
		viewportWidth: window.innerWidth
	}));

	expect(scrollWidth).toBeLessThanOrEqual(viewportWidth);
}
