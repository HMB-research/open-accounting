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
 */
export default defineConfig({
	testDir: './e2e',
	testMatch: ['**/demo/*.spec.ts', 'demo-env.spec.ts', 'demo-all-views.spec.ts'],
	fullyParallel: false, // Run sequentially to avoid rate limiting
	forbidOnly: !!process.env.CI,
	retries: 2, // Retry on network flakiness
	workers: 1, // Single worker for demo environment
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
		},
		{
			name: 'demo-firefox',
			use: {
				...devices['Desktop Firefox'],
				storageState: { cookies: [], origins: [] }
			}
		},
		{
			name: 'demo-mobile',
			use: {
				...devices['Pixel 5'],
				storageState: { cookies: [], origins: [] }
			}
		}
	]

	// No webServer - we're testing against live demo
});
