import { expect, test, type Page } from '@playwright/test';

test.describe('Authentication - Login Page', () => {
	// Clear storageState because this suite verifies unauthenticated login/register behavior.
	test.use({ storageState: { cookies: [], origins: [] } });

	const openLoginPage = async (page: Page) => {
		await page.goto('/login');
		await page.waitForLoadState('networkidle').catch(() => {
			// Vite and background requests can keep the page busy in local runs.
		});
		await expect(page.getByRole('heading', { name: /welcome|login|sign in|tere tulemast|logi/i })).toBeVisible();
	};

	const emailInput = (page: Page) => page.getByLabel(/email|e-post/i);
	const passwordInput = (page: Page) => page.locator('#password');
	const nameInput = (page: Page) => page.getByLabel(/name|nimi/i);
	const submitButton = (page: Page) => page.locator('form button[type="submit"]');
	const registerToggleButton = (page: Page) =>
		page.locator('.toggle-mode').getByRole('button', { name: /create account|register|loo konto/i });

	test('validates login form controls and register mode requirements', async ({ page }) => {
		await openLoginPage(page);

		await expect(emailInput(page)).toBeVisible();
		await expect(passwordInput(page)).toBeVisible();
		await expect(submitButton(page)).toBeVisible();

		await emailInput(page).fill('test@example.com');
		await passwordInput(page).fill('password123');

		await expect(emailInput(page)).toHaveValue('test@example.com');
		await expect(passwordInput(page)).toHaveValue('password123');
		await expect(passwordInput(page)).not.toHaveAttribute('minlength', /.+/);
		await expect(submitButton(page)).toBeEnabled();

		await registerToggleButton(page).click();

		await expect(page.getByRole('heading', { name: /create account|register|loo konto/i })).toBeVisible();
		await expect(nameInput(page)).toBeVisible();
		await expect(passwordInput(page)).toHaveAttribute('minlength', '8');
		await expect(submitButton(page)).toContainText(/create account|register|loo konto/i);
	});

	test('shows invalid credential errors without leaving login', async ({ page }) => {
		await openLoginPage(page);

		await emailInput(page).fill('invalid@example.com');
		await passwordInput(page).fill('wrongpassword');
		await expect(emailInput(page)).toHaveValue('invalid@example.com');
		await expect(passwordInput(page)).toHaveValue('wrongpassword');

		const [response] = await Promise.all([
			page.waitForResponse(
				(loginResponse) =>
					new URL(loginResponse.url()).pathname.endsWith('/api/v1/auth/login') &&
					loginResponse.request().method() === 'POST'
			),
			submitButton(page).click()
		]);
		expect(response.status()).toBe(401);

		await expect(page.locator('.alert-error, [role="alert"]')).toContainText(/.+/);
		await expect(page).toHaveURL(/login/i);
	});

	test('accepts demo password length in login mode', async ({ page }) => {
		await openLoginPage(page);

		await emailInput(page).fill('demo1@example.com');
		await passwordInput(page).fill('demo12345');

		await expect(passwordInput(page)).not.toHaveAttribute('minlength', /.+/);
		await expect(submitButton(page)).toBeEnabled();

		const passwordValidity = await passwordInput(page).evaluate((input: HTMLInputElement) => ({
			valid: input.checkValidity(),
			validationMessage: input.validationMessage
		}));

		expect(passwordValidity).toEqual({ valid: true, validationMessage: '' });
	});
});
