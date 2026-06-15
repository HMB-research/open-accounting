import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { baseLocale, setLocale } from '$lib/paraglide/runtime.js';
import ExportButton from '$lib/components/ExportButton.svelte';

const originalCreateObjectURL = URL.createObjectURL;
const originalRevokeObjectURL = URL.revokeObjectURL;

function readBlobText(blob: Blob): Promise<string> {
	return new Promise((resolve, reject) => {
		const reader = new FileReader();
		reader.onerror = () => reject(reader.error);
		reader.onload = () => resolve(String(reader.result));
		reader.readAsText(blob);
	});
}

function stubObjectUrls() {
	const createObjectURL = vi.fn((_blob: Blob) => 'blob:open-accounting-export');
	const revokeObjectURL = vi.fn();
	Object.defineProperty(URL, 'createObjectURL', {
		configurable: true,
		value: createObjectURL
	});
	Object.defineProperty(URL, 'revokeObjectURL', {
		configurable: true,
		value: revokeObjectURL
	});
	return { createObjectURL, revokeObjectURL };
}

describe('ExportButton', () => {
	beforeEach(() => {
		setLocale(baseLocale, { reload: false });
	});

	afterEach(() => {
		cleanup();
		vi.restoreAllMocks();

		if (originalCreateObjectURL) {
			Object.defineProperty(URL, 'createObjectURL', {
				configurable: true,
				value: originalCreateObjectURL
			});
		} else {
			delete (URL as unknown as { createObjectURL?: typeof URL.createObjectURL }).createObjectURL;
		}

		if (originalRevokeObjectURL) {
			Object.defineProperty(URL, 'revokeObjectURL', {
				configurable: true,
				value: originalRevokeObjectURL
			});
		} else {
			delete (URL as unknown as { revokeObjectURL?: typeof URL.revokeObjectURL }).revokeObjectURL;
		}
	});

	it('opens and closes the export menu', async () => {
		render(ExportButton, {
			data: [[{ name: 'Baltic Commerce', amount: '42.35' }]],
			headers: [['Name', 'Amount']],
			filename: 'cash-payments'
		});

		await fireEvent.click(screen.getByRole('button', { name: /^Export$/ }));

		expect(screen.getByRole('button', { name: 'Export to Excel' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Export to CSV' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Export to PDF' })).toBeInTheDocument();

		await fireEvent.click(document.body);
		expect(screen.queryByRole('button', { name: 'Export to CSV' })).not.toBeInTheDocument();
	});

	it('downloads escaped CSV for the first sheet', async () => {
		const { createObjectURL, revokeObjectURL } = stubObjectUrls();
		const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

		render(ExportButton, {
			data: [[{ name: 'Acme, Inc', amount: '42.35' }]],
			headers: [['Name', 'Amount']],
			filename: 'cash-payments'
		});

		await fireEvent.click(screen.getByRole('button', { name: /^Export$/ }));
		await fireEvent.click(screen.getByRole('button', { name: 'Export to CSV' }));

		expect(click).toHaveBeenCalledTimes(1);
		expect(createObjectURL).toHaveBeenCalledTimes(1);
		const blob = createObjectURL.mock.calls[0]?.[0];
		if (!blob) throw new Error('Expected CSV export to create a Blob URL');
		await expect(readBlobText(blob)).resolves.toBe('Name,Amount\n"Acme, Inc",42.35');
		expect(revokeObjectURL).toHaveBeenCalledWith('blob:open-accounting-export');
		expect(screen.queryByRole('button', { name: 'Export to CSV' })).not.toBeInTheDocument();
	});

	it('prints the current page for PDF exports', async () => {
		const print = vi.spyOn(window, 'print').mockImplementation(() => {});

		render(ExportButton, {
			data: [[{ name: 'Baltic Commerce', amount: '42.35' }]],
			headers: [['Name', 'Amount']],
			filename: 'cash-payments'
		});

		await fireEvent.click(screen.getByRole('button', { name: /^Export$/ }));
		await fireEvent.click(screen.getByRole('button', { name: 'Export to PDF' }));

		expect(print).toHaveBeenCalledTimes(1);
		expect(screen.queryByRole('button', { name: 'Export to PDF' })).not.toBeInTheDocument();
	});
});
