import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { baseLocale, setLocale } from '$lib/paraglide/runtime.js';
import Decimal from 'decimal.js';
import LineItemsEditor from '$lib/components/LineItemsEditor.svelte';
import type { LineItem } from '$lib/utils/formatting';

function line(overrides: Partial<LineItem> = {}): LineItem {
	return {
		description: 'Consulting',
		quantity: '2',
		unit_price: '100',
		vat_rate: '22',
		discount_percent: '10',
		...overrides
	};
}

function formatCurrency(value: Decimal): string {
	return `EUR ${value.toFixed(2)}`;
}

describe('LineItemsEditor', () => {
	beforeEach(() => {
		setLocale(baseLocale, { reload: false });
	});

	afterEach(() => {
		cleanup();
	});

	it('renders editable lines with calculated row and grand totals', () => {
		render(LineItemsEditor, {
			lines: [
				line(),
				line({
					description: 'Hosting',
					quantity: '1',
					unit_price: '50',
					vat_rate: '0',
					discount_percent: '0'
				})
			],
			formatCurrency,
			sectionLabel: 'Quote lines'
		});

		expect(screen.getByRole('heading', { name: 'Quote lines' })).toBeInTheDocument();
		expect(screen.getByDisplayValue('Consulting')).toBeInTheDocument();
		expect(screen.getByDisplayValue('Hosting')).toBeInTheDocument();
		expect(screen.getByText('EUR 219.60')).toBeInTheDocument();
		expect(screen.getByText('EUR 50.00')).toBeInTheDocument();
		expect(screen.getByText('EUR 269.60')).toBeInTheDocument();
	});

	it('adds a line using the first configured VAT rate', async () => {
		const { container } = render(LineItemsEditor, {
			lines: [line()],
			vatRates: ['24', '9', '0'],
			formatCurrency,
			addLabel: 'Add quote line'
		});

		await fireEvent.click(screen.getByRole('button', { name: '+ Add quote line' }));

		const rows = container.querySelectorAll('tbody tr');
		expect(rows).toHaveLength(2);
		const vatSelects = container.querySelectorAll<HTMLSelectElement>('tbody select');
		expect(vatSelects[1]?.value).toBe('24');
		expect(screen.getAllByDisplayValue('')).toHaveLength(1);
	});

	it('recalculates totals when line fields change', async () => {
		render(LineItemsEditor, {
			lines: [line({ quantity: '1', unit_price: '100', vat_rate: '0', discount_percent: '0' })],
			formatCurrency
		});

		await fireEvent.input(screen.getByDisplayValue('1'), {
			target: { value: '3' }
		});

		expect(screen.getAllByText('EUR 300.00')).toHaveLength(2);
	});

	it('removes lines while keeping the final required line', async () => {
		const { container } = render(LineItemsEditor, {
			lines: [line({ description: 'First line' }), line({ description: 'Second line' })],
			formatCurrency
		});

		await fireEvent.click(screen.getAllByRole('button', { name: '×' })[0]);

		expect(container.querySelectorAll('tbody tr')).toHaveLength(1);
		expect(screen.queryByDisplayValue('First line')).not.toBeInTheDocument();
		expect(screen.getByDisplayValue('Second line')).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: '×' })).not.toBeInTheDocument();
	});

	it('can hide discount controls for document types that do not support discounts', () => {
		const { container } = render(LineItemsEditor, {
			lines: [line()],
			showDiscount: false,
			formatCurrency
		});

		expect(screen.queryByText('Discount %')).not.toBeInTheDocument();
		expect(container.querySelectorAll('tbody input[type="number"]')).toHaveLength(2);
		expect(screen.getAllByText('EUR 219.60')).toHaveLength(2);
	});
});
