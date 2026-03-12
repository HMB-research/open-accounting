import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { baseLocale, setLocale } from '$lib/paraglide/runtime.js';
import type { DocumentAttachment } from '$lib/api';

const { apiMock } = vi.hoisted(() => ({
	apiMock: {
		listDocuments: vi.fn(),
		uploadDocument: vi.fn(),
		downloadDocument: vi.fn(),
		deleteDocument: vi.fn()
	}
}));

vi.mock('$lib/api', async () => {
	const actual = await vi.importActual<typeof import('$lib/api')>('$lib/api');
	return {
		...actual,
		api: apiMock
	};
});

import DocumentManager from '$lib/components/DocumentManager.svelte';

function createDocument(overrides: Partial<DocumentAttachment> = {}): DocumentAttachment {
	return {
		id: 'doc-1',
		tenant_id: 'tenant-1',
		entity_type: 'invoice',
		entity_id: 'inv-1',
		file_name: 'invoice.pdf',
		content_type: 'application/pdf',
		file_size: 2048,
		uploaded_by: 'user-1',
		created_at: '2026-03-01T10:00:00Z',
		...overrides
	};
}

describe('DocumentManager', () => {
	afterEach(() => {
		cleanup();
		vi.restoreAllMocks();
		vi.unstubAllGlobals();
	});

	beforeEach(() => {
		setLocale(baseLocale, { reload: false });
		vi.clearAllMocks();
		vi.stubGlobal('confirm', vi.fn(() => true));
		apiMock.listDocuments.mockResolvedValue([]);
		apiMock.uploadDocument.mockResolvedValue(createDocument());
		apiMock.downloadDocument.mockResolvedValue(undefined);
		apiMock.deleteDocument.mockResolvedValue({ status: 'deleted' });
	});

	it('loads, downloads, and deletes existing documents', async () => {
		apiMock.listDocuments.mockResolvedValueOnce([
			createDocument({ file_name: 'invoice-1001.pdf' })
		]);

		render(DocumentManager, {
			open: true,
			tenantId: 'tenant-1',
			entityType: 'invoice',
			entityId: 'inv-1',
			title: 'Documents for invoice INV-1001'
		});

		await waitFor(() => {
			expect(apiMock.listDocuments).toHaveBeenCalledWith('tenant-1', 'invoice', 'inv-1');
		});

		expect(screen.getByText('invoice-1001.pdf')).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Download' }));
		expect(apiMock.downloadDocument).toHaveBeenCalledWith('tenant-1', 'doc-1', 'invoice-1001.pdf');

		await fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
		expect(apiMock.deleteDocument).toHaveBeenCalledWith('tenant-1', 'doc-1');
		await waitFor(() => {
			expect(screen.queryByText('invoice-1001.pdf')).not.toBeInTheDocument();
		});
	});

	it('uploads selected files and refreshes the list', async () => {
		apiMock.listDocuments
			.mockResolvedValueOnce([])
			.mockResolvedValueOnce([
				createDocument({
					id: 'doc-2',
					entity_type: 'payment',
					entity_id: 'pay-1',
					file_name: 'receipt.pdf'
				})
			]);

		const { container } = render(DocumentManager, {
			open: true,
			tenantId: 'tenant-1',
			entityType: 'payment',
			entityId: 'pay-1',
			title: 'Documents for payment PMT-001'
		});

		await waitFor(() => {
			expect(apiMock.listDocuments).toHaveBeenCalledWith('tenant-1', 'payment', 'pay-1');
		});

		const fileInput = container.querySelector('input[type="file"]') as HTMLInputElement | null;
		expect(fileInput).not.toBeNull();

		const file = new File(['receipt'], 'receipt.pdf', { type: 'application/pdf' });
		Object.defineProperty(fileInput, 'files', {
			configurable: true,
			value: [file]
		});
		await fireEvent.change(fileInput as HTMLInputElement);
		await fireEvent.click(screen.getByRole('button', { name: 'Upload selected files' }));

		await waitFor(() => {
			expect(apiMock.uploadDocument).toHaveBeenCalledWith('tenant-1', 'payment', 'pay-1', file);
		});
		await waitFor(() => {
			expect(apiMock.listDocuments).toHaveBeenCalledTimes(2);
		});
		expect(screen.getByText('receipt.pdf')).toBeInTheDocument();
	});
});
