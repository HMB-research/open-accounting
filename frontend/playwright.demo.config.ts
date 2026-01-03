import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for Demo Environment E2E tests
 *
 * Run with: npx playwright test --config=playwright.demo.config.ts
 * Or: npm run test:e2e:demo
 *
 * These tests run against the live demo environment:
 * - Frontend: https://open-accounting.up.railway.app
 * - API: https://open-accounting-api.up.railway.app
 *
 * Parallel testing is enabled with 3 workers, each using a dedicated demo user:
 * - Worker 0: demo1@example.com / tenant_demo1
 * - Worker 1: demo2@example.com / tenant_demo2
 * - Worker 2: demo3@example.com / tenant_demo3
 */
export default defineConfig({
	testDir: './e2e',
	testMatch: ['**/demo/*.spec.ts', 'demo-env.spec.ts', 'demo-all-views.spec.ts'],
	fullyParallel: true, // Enable parallel execution
	forbidOnly: !!process.env.CI,
	retries: 2, // Retry on network flakiness
	workers: 3, // 3 workers for 3 demo users
	reporter: [
		['html', { outputFolder: 'playwright-report-demo' }],
		['list'],
		['json', { outputFile: 'demo-test-results.json' }]
	],
	timeout: 60000, // Longer timeout for remote environment

	use: {
		baseURL: 'https://open-accounting.up.railway.app',
		trace: 'on-first-retry',
		screenshot: 'only-on-failure',
		video: 'retain-on-failure',
		// Extra time for network requests
		actionTimeout: 15000,
		navigationTimeout: 30000
	},

	projects: [
		{
			name: 'demo-chromium',
			use: {
				...devices['Desktop Chrome'],
				// Clear state for each test
				storageState: { cookies: [], origins: [] }
			}
		}
	]

	// No webServer - we're testing against live demo
});
