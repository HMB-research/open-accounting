import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { DEMO_API_URL, ensureAuthenticated, ensureDemoTenant, getDemoCredentials, navigateTo } from './utils';

interface PaymentResponse {
	id: string;
	payment_number: string;
	reference?: string;
}

interface DocumentResponse {
	id: string;
	file_name: string;
	document_type: 'supporting_document' | 'receipt';
	review_status: 'PENDING' | 'REVIEWED' | 'APPROVED' | 'REJECTED';
	retention_until?: string;
}

async function demoApiRequest<T>(
	page: Page,
	path: string,
	options: {
		method?: string;
		body?: unknown;
		form?: Record<string, string>;
		file?: { field: string; name: string; content: string; type: string };
	} = {}
): Promise<{ status: number; body: T }> {
	return page.evaluate(
		async ({ apiUrl, requestPath, requestOptions }) => {
			const token = localStorage.getItem('access_token') || sessionStorage.getItem('access_token');
			if (!token) {
				throw new Error('Missing demo access token');
			}

			const headers: Record<string, string> = {
				Authorization: `Bearer ${token}`
			};
			let body: BodyInit | undefined;

			if (requestOptions.form || requestOptions.file) {
				const formData = new FormData();
				for (const [key, value] of Object.entries(requestOptions.form || {})) {
					formData.set(key, value);
				}
				if (requestOptions.file) {
					formData.set(
						requestOptions.file.field,
						new File([requestOptions.file.content], requestOptions.file.name, {
							type: requestOptions.file.type
						})
					);
				}
				body = formData;
			} else if (requestOptions.body !== undefined) {
				headers['Content-Type'] = 'application/json';
				body = JSON.stringify(requestOptions.body);
			}

			const response = await fetch(`${apiUrl}${requestPath}`, {
				method: requestOptions.method || 'GET',
				headers,
				body
			});
			const text = await response.text();
			let parsed: unknown = {};
			if (text) {
				try {
					parsed = JSON.parse(text);
				} catch {
					parsed = { raw: text };
				}
			}
			return { status: response.status, body: parsed };
		},
		{ apiUrl: DEMO_API_URL, requestPath: path, requestOptions: options }
	) as Promise<{ status: number; body: T }>;
}

async function createPaymentWithDocuments(page: Page, testInfo: TestInfo) {
	const tenantId = getDemoCredentials(testInfo).tenantId;
	const suffix = Date.now();
	const paymentResponse = await demoApiRequest<PaymentResponse>(
		page,
		`/api/v1/tenants/${tenantId}/payments`,
		{
			method: 'POST',
			body: {
				payment_type: 'RECEIVED',
				payment_date: '2026-06-09T00:00:00Z',
				amount: '87.65',
				currency: 'EUR',
				payment_method: 'BANK_TRANSFER',
				reference: `E2E-DOC-PAY-${suffix}`,
				notes: `Document review queue workflow ${suffix}`
			}
		}
	);
	expect(paymentResponse.status).toBe(201);

	const reviewFileName = `e2e-review-${suffix}.txt`;
	const reviewUpload = await demoApiRequest<DocumentResponse>(
		page,
		`/api/v1/tenants/${tenantId}/documents`,
		{
			method: 'POST',
			form: {
				entity_type: 'payment',
				entity_id: paymentResponse.body.id,
				document_type: 'supporting_document',
				notes: `Pending review evidence ${suffix}`
			},
			file: {
				field: 'file',
				name: reviewFileName,
				content: `Review evidence for ${paymentResponse.body.payment_number}`,
				type: 'text/plain'
			}
		}
	);
	expect(reviewUpload.status).toBe(201);
	expect(reviewUpload.body.review_status).toBe('PENDING');

	const retentionFileName = `e2e-retention-${suffix}.txt`;
	const retentionUpload = await demoApiRequest<DocumentResponse>(
		page,
		`/api/v1/tenants/${tenantId}/documents`,
		{
			method: 'POST',
			form: {
				entity_type: 'payment',
				entity_id: paymentResponse.body.id,
				document_type: 'receipt',
				retention_until: '2026-06-30',
				notes: `Retention evidence ${suffix}`
			},
			file: {
				field: 'file',
				name: retentionFileName,
				content: `Retention evidence for ${paymentResponse.body.payment_number}`,
				type: 'text/plain'
			}
		}
	);
	expect(retentionUpload.status).toBe(201);
	expect(retentionUpload.body.retention_until).toContain('2026-06-30');

	return {
		tenantId,
		payment: paymentResponse.body,
		reviewDocument: reviewUpload.body,
		retentionDocument: retentionUpload.body,
		reviewFileName,
		retentionFileName
	};
}

async function waitForDocumentsLoaded(page: Page) {
	await expect(async () => {
		const isLoading = await page
			.getByText(/^Loading\.\.\.$/i)
			.first()
			.isVisible()
			.catch(() => false);
		const hasTable = await page
			.locator('table tbody tr')
			.first()
			.isVisible()
			.catch(() => false);
		const hasEmpty = await page
			.locator('.empty-state')
			.isVisible()
			.catch(() => false);
		expect(isLoading === false && (hasTable || hasEmpty)).toBeTruthy();
	}).toPass({ timeout: 15000 });
}

function documentRow(page: Page, fileName: string) {
	return page.locator('table tbody tr').filter({ hasText: fileName });
}

test.describe('Document Review Queue', () => {
	test.beforeEach(async ({ page }, testInfo) => {
		await ensureAuthenticated(page, testInfo);
		await ensureDemoTenant(page, testInfo);
	});

	test('reviews documents and updates retention metadata from the tenant queue', async ({
		page
	}, testInfo) => {
		const setup = await createPaymentWithDocuments(page, testInfo);

		const reviewQueueLoaded = page.waitForResponse((response) => {
			const url = new URL(response.url());
			return (
				response.request().method() === 'GET' &&
				url.pathname.match(/\/api\/v1\/tenants\/[^/]+\/documents\/review-queue$/) !== null &&
				url.searchParams.get('entity_type') === 'payment' &&
				url.searchParams.get('document_type') === 'supporting_document' &&
				url.searchParams.get('review_status') === 'PENDING' &&
				response.status() === 200
			);
		});

		await navigateTo(
			page,
			'/documents?entity_type=payment&document_type=supporting_document&review_status=PENDING',
			testInfo
		);
		await reviewQueueLoaded;
		await waitForDocumentsLoaded(page);

		await expect(page.getByRole('heading', { name: /document review/i })).toBeVisible();
		const reviewRow = documentRow(page, setup.reviewFileName);
		await expect(reviewRow).toBeVisible({ timeout: 10000 });
		await expect(reviewRow).toContainText(setup.payment.id);
		await expect(reviewRow).toContainText(/pending review/i);
		await expect(documentRow(page, setup.retentionFileName)).toHaveCount(0);

		const approvalResponse = page.waitForResponse((response) => {
			const path = new URL(response.url()).pathname;
			return (
				response.request().method() === 'POST' &&
				path.endsWith(`/documents/${setup.reviewDocument.id}/review`)
			);
		});
		await reviewRow.locator('textarea').fill('Approved by document review demo E2E');
		await reviewRow.getByRole('button', { name: /^approve$/i }).click();
		const approval = await approvalResponse;
		expect(approval.status()).toBe(200);
		const approvedDocument = (await approval.json()) as DocumentResponse;
		expect(approvedDocument.review_status).toBe('APPROVED');
		await expect(page.getByText(`${setup.reviewFileName} review status updated.`)).toBeVisible({
			timeout: 10000
		});
		await expect(documentRow(page, setup.reviewFileName)).toHaveCount(0);

		const retentionQueueLoaded = page.waitForResponse((response) => {
			const url = new URL(response.url());
			return (
				response.request().method() === 'GET' &&
				url.pathname.match(/\/api\/v1\/tenants\/[^/]+\/documents\/retention$/) !== null &&
				url.searchParams.get('as_of') === '2026-06-09' &&
				url.searchParams.get('horizon_days') === '45' &&
				url.searchParams.get('include_missing') === 'true' &&
				response.status() === 200
			);
		});
		await page.getByRole('tab', { name: /retention queue/i }).click();
		await page.locator('#retention-as-of').fill('2026-06-09');
		await page.locator('#retention-horizon').fill('45');
		await page.locator('#include-missing-retention').check();
		await page.getByRole('button', { name: /apply filters/i }).click();
		await retentionQueueLoaded;
		await waitForDocumentsLoaded(page);

		const retentionRow = documentRow(page, setup.retentionFileName);
		await expect(retentionRow).toBeVisible({ timeout: 10000 });
		await expect(retentionRow).toContainText(setup.payment.id);
		await expect(retentionRow).toContainText(/Jun 30, 2026|30\.6\.2026|6\/30\/2026|2026-06-30/);

		const retentionResponse = page.waitForResponse((response) => {
			const path = new URL(response.url()).pathname;
			return (
				response.request().method() === 'PATCH' &&
				path.endsWith(`/documents/${setup.retentionDocument.id}/retention`)
			);
		});
		await retentionRow.locator(`#retention-date-${setup.retentionDocument.id}`).fill('2028-03-31');
		await retentionRow.getByRole('button', { name: /^save retention$/i }).click();
		const retentionUpdate = await retentionResponse;
		expect(retentionUpdate.status()).toBe(200);
		const updatedDocument = (await retentionUpdate.json()) as DocumentResponse;
		expect(updatedDocument.retention_until).toContain('2028-03-31');
		await expect(page.getByText(`${setup.retentionFileName} retention metadata updated.`)).toBeVisible({
			timeout: 10000
		});
	});
});
