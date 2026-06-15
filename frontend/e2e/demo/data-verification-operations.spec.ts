import { test } from '@playwright/test';
import {
	operationsRoutes,
	prepareDataVerificationPage,
	verifyDemoRoutes
} from './data-verification-utils';

test.describe('Demo Data Verification - Operations', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await prepareDataVerificationPage(page, testInfo);
	});

	test('loads recurring invoices, banking, and reports without data errors', async ({ page }, testInfo) => {
		await verifyDemoRoutes(page, testInfo, operationsRoutes);
	});
});
