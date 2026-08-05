import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { ensureAuthenticated, ensureDemoTenant, getDemoCredentials, navigateTo, waitForRouteReady } from './utils';

const routeLoadTimeout = 30_000;

async function openAccountantWorkspace(page: Page, testInfo: TestInfo): Promise<void> {
	await navigateTo(page, '/dashboard', testInfo, { waitForNetworkIdle: false });
	await waitForRouteReady(page, '#assignment-queue', routeLoadTimeout);
	await expect(page.locator('#assignment-queue .review-figure strong')).toBeVisible();
}

test.describe('Demo Accountant Workspace', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('renders the assignment queue and tenant-scoped review destinations', async ({ page }, testInfo) => {
		const { tenantId } = getDemoCredentials(testInfo);
		await openAccountantWorkspace(page, testInfo);

		const assignmentQueue = page.locator('#assignment-queue');
		await expect(assignmentQueue).toBeVisible();
		await expect(assignmentQueue.locator('.review-card-kicker')).toBeVisible();

		const reviewLinks = assignmentQueue.locator('a.review-action');
		await expect(reviewLinks.first()).toHaveAttribute(
			'href',
			`/documents?tenant=${tenantId}&review_status=PENDING`
		);

		const hrefs = await reviewLinks.evaluateAll((links) =>
			links.map((link) => link.getAttribute('href') ?? '')
		);
		for (const href of hrefs) {
			const url = new URL(href, page.url());
			expect(url.searchParams.get('tenant')).toBe(tenantId);
		expect(url.pathname).not.toMatch(/^\/expenses(?:\/|$)/);
		expect(url.pathname).not.toMatch(/^\/tax(?:\/|$)/);
		}
	});
});
