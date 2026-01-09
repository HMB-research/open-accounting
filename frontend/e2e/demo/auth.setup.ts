import { test as setup, expect } from '@playwright/test';
import { DEMO_CREDENTIALS, DEMO_URL } from './utils';

// This runs once per worker before any tests
// Each worker gets its own demo user based on parallelIndex
setup('authenticate demo user', async ({ page }, testInfo) => {
	const workerIndex = testInfo.parallelIndex % DEMO_CREDENTIALS.length;
	const creds = DEMO_CREDENTIALS[workerIndex];
	const authFile = `frontend/.auth/worker-${workerIndex}.json`;

	console.log(`[Worker ${workerIndex}] Authenticating as ${creds.email}...`);

	// Navigate to login page
	await page.goto(`${DEMO_URL}/login`);
	await page.waitForLoadState('networkidle');

	// Fill credentials
	await page.waitForSelector('input[type="email"], input[name="email"]', { timeout: 10000 });
	const emailInput = page.locator('input[type="email"], input[name="email"]').first();
	const passwordInput = page.locator('input[type="password"]').first();
	await emailInput.fill(creds.email);
	await passwordInput.fill(creds.password);

	// Submit and wait for dashboard
	const signInButton = page.getByRole('button', { name: /sign in|login|logi sisse/i });
	await signInButton.click();
	await page.waitForURL(/dashboard/, { timeout: 30000 });

	// Wait for dashboard to be fully loaded
	await page.waitForLoadState('domcontentloaded');
	await expect(page.getByText(/dashboard|cash flow|revenue/i).first()).toBeVisible({ timeout: 10000 });

	console.log(`[Worker ${workerIndex}] Login successful, saving state to ${authFile}`);

	// Save authentication state for this worker
	await page.context().storageState({ path: authFile });
});
