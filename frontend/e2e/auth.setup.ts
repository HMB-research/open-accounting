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

	// Step 1: Register test user via API
	console.log(`Registering test user at ${baseUrl}/api/v1/auth/register`);
	const registerResponse = await request.post(`${baseUrl}/api/v1/auth/register`, {
		data: TEST_USER,
		failOnStatusCode: false
	});
	console.log(`Registration response: ${registerResponse.status()}`);

	// 201 = created, 409 = already exists - both are OK
	if (registerResponse.status() !== 201 && registerResponse.status() !== 409) {
		const body = await registerResponse.text();
		console.log(`Registration response body: ${body}`);
	}

	// Step 2: Login via API to get tokens
	console.log(`Logging in at ${baseUrl}/api/v1/auth/login`);
	const loginResponse = await request.post(`${baseUrl}/api/v1/auth/login`, {
		data: {
			email: TEST_USER.email,
			password: TEST_USER.password
		},
		failOnStatusCode: false
	});
	console.log(`Login response: ${loginResponse.status()}`);

	if (!loginResponse.ok()) {
		const body = await loginResponse.text();
		throw new Error(`Login failed: ${loginResponse.status()} - ${body}`);
	}

	const tokens = await loginResponse.json();
	console.log(`Login successful, got access token`);

	// Step 3: Navigate to app and inject tokens into localStorage
	await page.goto('/login');
	await page.evaluate((tokenData) => {
		localStorage.setItem('access_token', tokenData.access_token);
		localStorage.setItem('refresh_token', tokenData.refresh_token);
	}, tokens);

	// Step 4: Navigate to dashboard (should be authenticated now)
	await page.goto('/dashboard');
	await expect(page).toHaveURL(/dashboard/i, { timeout: 10000 });

	// Step 5: Save storage state for reuse by other tests
	await page.context().storageState({ path: authFile });
});
