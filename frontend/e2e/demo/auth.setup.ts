import { test as setup, expect } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';
import { DEMO_CREDENTIALS, DEMO_URL } from './utils';

// ESM-compatible __dirname
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Auth state directory - must match utils.ts and playwright.demo.config.ts
const AUTH_DIR = path.join(__dirname, '..', '..', '.auth');

// Authenticate all 4 demo users and save their auth state
// This runs once before any test workers start
setup('authenticate all demo users', async ({ browser }) => {
	// Ensure auth directory exists
	if (!fs.existsSync(AUTH_DIR)) {
		fs.mkdirSync(AUTH_DIR, { recursive: true });
	}

	console.log(`[Auth Setup] Authenticating all ${DEMO_CREDENTIALS.length} demo users...`);

	// Authenticate each demo user in sequence
	for (let workerIndex = 0; workerIndex < DEMO_CREDENTIALS.length; workerIndex++) {
		const creds = DEMO_CREDENTIALS[workerIndex];
		const authFile = path.join(AUTH_DIR, `worker-${workerIndex}.json`);

		console.log(`[Auth Setup] Authenticating demo user ${workerIndex + 1}/${DEMO_CREDENTIALS.length}: ${creds.email}...`);

		// Create a new context for each user
		const context = await browser.newContext();
		const page = await context.newPage();

		try {
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

			console.log(`[Auth Setup] Login successful for ${creds.email}, saving state to ${authFile}`);

			// Save authentication state
			await context.storageState({ path: authFile });
		} catch (error) {
			console.error(`[Auth Setup] Failed to authenticate ${creds.email}:`, error);
			throw error;
		} finally {
			await context.close();
		}
	}

	console.log(`[Auth Setup] All ${DEMO_CREDENTIALS.length} demo users authenticated successfully`);
});
