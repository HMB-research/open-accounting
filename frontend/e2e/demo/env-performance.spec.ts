import { test, expect } from '@playwright/test';
import { loginWithDemoCredentials } from './utils';
import { DEMO_USER, loginAsDemoEnv, waitForVisibleContent } from './env-utils';

test.describe('Demo Environment - Performance', () => {
	test('Login flow completes successfully', async ({ page }) => {
		const startTime = Date.now();

		await loginWithDemoCredentials(page, DEMO_USER, { logPrefix: 'Performance login' });

		const elapsed = Date.now() - startTime;
		console.log(`Login completed in ${elapsed}ms`);

		expect(elapsed).toBeLessThan(45000);
	});

	test('Dashboard reload is responsive', async ({ page }, testInfo) => {
		await loginAsDemoEnv(page, testInfo);

		const startTime = Date.now();
		await page.reload();
		await waitForVisibleContent(page);

		const elapsed = Date.now() - startTime;
		console.log(`Dashboard reload completed in ${elapsed}ms`);

		expect(elapsed).toBeLessThan(20000);
	});
});
