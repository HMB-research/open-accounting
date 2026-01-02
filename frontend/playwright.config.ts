import { defineConfig, devices } from '@playwright/test';

const authFile = './e2e/.auth/user.json';

/**
 * Playwright configuration for Open Accounting E2E tests
 * See https://playwright.dev/docs/test-configuration
 */
export default defineConfig({
	testDir: './e2e',
	// Exclude demo environment tests - run them with: npm run test:e2e:demo
	testIgnore: ['**/demo-env.spec.ts'],
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 0,
	workers: process.env.CI ? 4 : undefined, // Maximize parallelization in CI
	reporter: [['html', { outputFolder: 'playwright-report' }], ['list']],
	timeout: 30000,

	use: {
		baseURL: process.env.BASE_URL || 'http://localhost:5173',
		trace: 'on-first-retry',
		screenshot: 'only-on-failure',
		video: 'retain-on-failure'
	},

	projects: [
		// Setup project - runs first to authenticate
		{
			name: 'setup',
			testMatch: /auth\.setup\.ts/
		},
		// Desktop browsers
		{
			name: 'chromium',
			use: {
				...devices['Desktop Chrome'],
				storageState: authFile
			},
			dependencies: ['setup']
		},
		{
			name: 'firefox',
			use: {
				...devices['Desktop Firefox'],
				storageState: authFile
			},
			dependencies: ['setup']
		},
		{
			name: 'webkit',
			use: {
				...devices['Desktop Safari'],
				storageState: authFile
			},
			dependencies: ['setup']
		},
		// Mobile viewports
		{
			name: 'Mobile Chrome',
			use: {
				...devices['Pixel 5'],
				storageState: authFile
			},
			dependencies: ['setup']
		},
		{
			name: 'Mobile Safari',
			use: {
				...devices['iPhone 12'],
				storageState: authFile
			},
			dependencies: ['setup']
		}
	],

	webServer: {
		command: 'npm run dev',
		url: 'http://localhost:5173',
		reuseExistingServer: !process.env.CI,
		timeout: 120000
	}
});
