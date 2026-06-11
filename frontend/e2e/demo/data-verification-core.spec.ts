import { test } from '@playwright/test';
import {
	coreAccountingRoutes,
	prepareDataVerificationPage,
	verifyDemoRoutes
} from './data-verification-utils';

test.describe('Demo Data Verification - Core Accounting', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await prepareDataVerificationPage(page, testInfo);
	});

	test('loads dashboard, accounts, and journal without data errors', async ({ page }, testInfo) => {
		await verifyDemoRoutes(page, testInfo, coreAccountingRoutes);
	});
});
