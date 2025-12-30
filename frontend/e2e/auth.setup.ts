import { test as setup, expect } from '@playwright/test';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const authFile = path.join(__dirname, '.auth/user.json');

const TEST_USER = {
	email: 'test@example.com',
	password: 'testpassword123',
	name: 'E2E Test User'
};

setup('authenticate', async ({ page, request }) => {
	const baseUrl = process.env.PUBLIC_API_URL || 'http://localhost:8080';

	// Step 1: Try to register test user via API (ignore if already exists)
	try {
		await request.post(`${baseUrl}/api/v1/auth/register`, {
			data: TEST_USER
		});
		// 201 = created, 409 = already exists - both are OK
	} catch {
		// Registration may fail if user already exists - that's fine
	}

	// Step 2: Login via UI to get proper localStorage state
	await page.goto('/login');
	await page.getByLabel(/email/i).fill(TEST_USER.email);
	await page.getByLabel(/password/i).fill(TEST_USER.password);
	await page.getByRole('button', { name: /login|sign in/i }).click();

	// Wait for successful navigation to dashboard
	await expect(page).toHaveURL(/dashboard/i, { timeout: 15000 });

	// Step 3: Save storage state for reuse by other tests
	await page.context().storageState({ path: authFile });
});
