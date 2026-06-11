import { expect, test, type Page } from '@playwright/test';

test.describe('Authentication - Login Page', () => {
	// Clear storageState because this suite verifies unauthenticated login/register behavior.
	test.use({ storageState: { cookies: [], origins: [] } });

	const openLoginPage = async (page: Page) => {
		await page.goto('/login');
		await page.waitForLoadState('networkidle').catch(() => {
			// Vite and background requests can keep the page busy in local runs.
		});
		await expect(page.getByRole('heading', { name: /welcome|login|sign in/i })).toBeVisible();
	};

	const emailInput = (page: Page) => page.getByLabel(/email/i);
	const passwordInput = (page: Page) => page.locator('#password');
	const submitButton = (page: Page) => page.getByRole('button', { name: /sign in|login/i });

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

		await page.evaluate(() => {
			const buttons = document.querySelectorAll('button.link-btn');
			for (const button of buttons) {
				if (button.textContent?.toLowerCase().includes('create account')) {
					(button as HTMLButtonElement).click();
					break;
				}
			}
		});

		await expect(page.getByRole('heading', { name: /create account|register/i })).toBeVisible();
		await expect(page.getByLabel(/name/i)).toBeVisible();
		await expect(passwordInput(page)).toHaveAttribute('minlength', '8');
		await expect(submitButton(page)).toBeVisible();
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
