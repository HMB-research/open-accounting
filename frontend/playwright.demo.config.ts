import { defineConfig, devices } from '@playwright/test';
import * as path from 'path';
import { fileURLToPath } from 'url';

// ESM-compatible __dirname
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const isRemoteDemoTesting = process.env.TEST_DEMO === 'true';
const baseURL = process.env.BASE_URL || 'http://localhost:5173';
const localBaseURL = new URL(baseURL);
const localWebServerHost = localBaseURL.hostname;
const localWebServerPort = localBaseURL.port || (localBaseURL.protocol === 'https:' ? '443' : '80');

if (isRemoteDemoTesting && !process.env.BASE_URL) {
	throw new Error('BASE_URL is required when TEST_DEMO=true');
}

const isLocalTesting = !isRemoteDemoTesting && (baseURL.includes('localhost') || baseURL.includes('127.0.0.1'));

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
		},
		// Blocking smoke suite for core accountant flow.
		// Keep this intentionally small and stable so it can gate CI.
		{
			name: 'smoke-chromium',
			testMatch: ['**/smoke/*.spec.ts'],
			dependencies: ['auth-setup'],
			use: {
				...devices['Desktop Chrome'],
				storageState: { cookies: [], origins: [] }
			}
		}
	],

	// Start dev server when testing locally
	...(isLocalTesting && {
		webServer: {
			command: `bun run paraglide && bunx vite dev --host ${localWebServerHost} --port ${localWebServerPort}`,
			url: baseURL,
			reuseExistingServer: !process.env.CI,
			timeout: 120000
		}
	})
});
