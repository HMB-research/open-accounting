import { test } from '@playwright/test';
import {
	payrollTaxRoutes,
	prepareDataVerificationPage,
	verifyDemoRoutes
} from './data-verification-utils';

test.describe('Demo Data Verification - Payroll And Tax', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await prepareDataVerificationPage(page, testInfo);
	});

	test('loads employees, payroll, and TSD without data errors', async ({ page }, testInfo) => {
		await verifyDemoRoutes(page, testInfo, payrollTaxRoutes);
	});
});
