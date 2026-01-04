import { defineConfig, devices } from '@playwright/test';

// Use environment variables for local testing, fall back to Railway for remote demo testing
const baseURL = process.env.BASE_URL || 'https://open-accounting.up.railway.app';
const isLocalTesting = baseURL.includes('localhost');

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
 * Or against local environment when BASE_URL is set to localhost:
 * - Frontend: http://localhost:5173
 * - API: http://localhost:8080
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
		baseURL,
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
	],

	// Start dev server when testing locally
	...(isLocalTesting && {
		webServer: {
			command: 'npm run dev',
			url: baseURL,
			reuseExistingServer: !process.env.CI,
			timeout: 120000
		}
	})
});
