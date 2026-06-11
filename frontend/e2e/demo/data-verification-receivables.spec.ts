import { test } from '@playwright/test';
import {
	prepareDataVerificationPage,
	receivablesRoutes,
	verifyDemoRoutes
} from './data-verification-utils';

test.describe('Demo Data Verification - Receivables And Payments', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await prepareDataVerificationPage(page, testInfo);
	});

	test('loads contacts, invoices, and payments without data errors', async ({ page }, testInfo) => {
		await verifyDemoRoutes(page, testInfo, receivablesRoutes);
	});
});
