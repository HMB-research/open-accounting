import { defineConfig, devices } from '@playwright/test';
import * as path from 'path';
import { fileURLToPath } from 'url';

// ESM-compatible __dirname
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Use environment variables for local testing, fall back to Railway for remote demo testing
const baseURL = process.env.BASE_URL || 'https://open-accounting.up.railway.app';
const isLocalTesting = baseURL.includes('localhost');

// Auth state directory - each worker creates its own file
export const AUTH_DIR = path.join(__dirname, '.auth');

/**
 * Playwright configuration for Demo Environment E2E tests
 *
 * PERFORMANCE OPTIMIZATIONS:
 * 1. Auth setup runs once per worker, saves session state to file
 * 2. ensureAuthenticated loads saved state instead of re-logging in
 * 3. 4 workers for 4 demo users (demo1-4)
 * 4. CI sharding supported via --shard flag
 */
export default defineConfig({
	testDir: './e2e',
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 0, // Increase retries in CI for stability
	workers: 4, // 4 workers for 4 demo users
	reporter: [
		['html', { outputFolder: 'playwright-report-demo' }],
		['list'],
		['json', { outputFile: 'demo-test-results.json' }]
	],
	timeout: 60000,

	use: {
		baseURL,
		trace: 'on-first-retry',
		screenshot: 'only-on-failure',
		video: 'retain-on-failure',
		actionTimeout: 15000,
		navigationTimeout: 30000
	},

	projects: [
		// Auth setup project - runs first, once per worker
		// Creates auth state files that tests can load
		{
			name: 'auth-setup',
			testMatch: '**/demo/auth.setup.ts',
			use: {
				...devices['Desktop Chrome'],
				storageState: { cookies: [], origins: [] }
			}
		},
		// Main test project - depends on auth-setup
		// Tests use ensureAuthenticated to load saved auth state
		{
			name: 'demo-chromium',
			testMatch: ['**/demo/*.spec.ts', 'demo-env.spec.ts', 'demo-all-views.spec.ts', 'auth.spec.ts'],
			testIgnore: '**/demo/auth.setup.ts',
			dependencies: ['auth-setup'],
			use: {
				...devices['Desktop Chrome'],
				// Start with clean state - ensureAuthenticated will load auth file
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
