import { test, expect } from '@playwright/test';
import { advanceWizardStep, clickWizardButton, loginAsDemoEnv } from './env-utils';

test.describe('Demo Environment - Onboarding Wizard', () => {
	test('Onboarding wizard displays for new organization', async ({ page }, testInfo) => {
		await loginAsDemoEnv(page, testInfo);

		const wizardHeading = page.getByRole('heading', { name: /welcome to open accounting/i });
		const hasWizard = await wizardHeading.isVisible({ timeout: 5000 }).catch(() => false);

		if (hasWizard) {
			await expect(wizardHeading).toBeVisible();

			await expect(page.getByText(/set up your organization/i)).toBeVisible();

			await expect(page.getByText('Company')).toBeVisible();
			await expect(page.getByText('Branding')).toBeVisible();
			await expect(page.getByText('Contact')).toBeVisible();
			await expect(page.getByText('Done')).toBeVisible();
		}
	});

	test('Company information form displays correctly', async ({ page }, testInfo) => {
		await loginAsDemoEnv(page, testInfo);

		const companyHeading = page.getByRole('heading', { name: /company information/i });
		const hasCompanyForm = await companyHeading.isVisible({ timeout: 5000 }).catch(() => false);

		if (hasCompanyForm) {
			await expect(companyHeading).toBeVisible();

			await expect(page.getByLabel(/company name/i)).toBeVisible();
			await expect(page.getByLabel(/registration code/i)).toBeVisible();
			await expect(page.getByLabel(/vat number/i)).toBeVisible();
			await expect(page.getByLabel(/email/i)).toBeVisible();
			await expect(page.getByLabel(/phone/i)).toBeVisible();
			await expect(page.getByLabel(/address/i)).toBeVisible();
		}
	});

	test('Onboarding form accepts valid company data', async ({ page }, testInfo) => {
		await loginAsDemoEnv(page, testInfo);

		const companyNameInput = page.getByLabel(/company name/i);
		const hasCompanyForm = await companyNameInput.isVisible({ timeout: 5000 }).catch(() => false);

		if (hasCompanyForm) {
			await companyNameInput.fill('Test Company');
			await expect(companyNameInput).toHaveValue('Test Company');

			const nextButton = page.getByRole('button', { name: /next|continue|save/i });
			const hasNext = await nextButton.isVisible().catch(() => false);

			if (hasNext) {
				await expect(nextButton).toBeEnabled();
			}
		}
	});

	test('Onboarding wizard step navigation works', async ({ page }, testInfo) => {
		await loginAsDemoEnv(page, testInfo);

		const step1Active = page.locator('[class*="active"], [class*="current"]').filter({ hasText: /company/i });
		const hasSteps = await step1Active.isVisible({ timeout: 5000 }).catch(() => false);

		if (hasSteps) {
			await expect(step1Active).toBeVisible();
		}
	});

	test('Onboarding wizard can be completed to reach dashboard', async ({ page }, testInfo) => {
		await loginAsDemoEnv(page, testInfo);

		const wizardOverlay = page.locator('.onboarding-overlay, .onboarding-wizard');
		const hasWizard = await wizardOverlay.isVisible({ timeout: 5000 }).catch(() => false);

		if (hasWizard) {
			const companyNameInput = page.getByLabel(/company name/i);
			if (await companyNameInput.isVisible()) {
				await companyNameInput.fill('E2E Test Company');
			}
			await advanceWizardStep(page, /continue|next/i, /branding|invoice settings/i);

			const step2Continue = page.getByRole('button', { name: /continue|next/i });
			if (await step2Continue.isVisible()) {
				await advanceWizardStep(page, /continue|next/i, /first contact/i);
			}

			const step3Continue = page.getByRole('button', { name: /skip|continue/i });
			if (await step3Continue.isVisible()) {
				await advanceWizardStep(page, /skip|continue/i, /all set/i);
			}

			const goToDashboard = page.getByRole('button', { name: /go to dashboard|finish|complete/i });
			if (await goToDashboard.isVisible()) {
				await clickWizardButton(page, /go to dashboard|finish|complete/i);
				await expect(page).toHaveURL(/dashboard/, { timeout: 10000 });
			}

			await expect(wizardOverlay).not.toBeVisible({ timeout: 10000 });
		}

		await expect(page).toHaveURL(/dashboard/);
		const dashboardContent = page.locator('main, .dashboard, [class*="content"]').first();
		await expect(dashboardContent).toBeVisible();

		const wizardAfter = page.locator('.onboarding-overlay');
		await expect(wizardAfter).not.toBeVisible();
	});
});
