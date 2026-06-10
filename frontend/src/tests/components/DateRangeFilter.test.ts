import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { baseLocale, setLocale } from '$lib/paraglide/runtime.js';
import DateRangeFilter from '$lib/components/DateRangeFilter.svelte';

describe('DateRangeFilter', () => {
	beforeEach(() => {
		setLocale(baseLocale, { reload: false });
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2026-06-10T12:00:00Z'));
	});

	afterEach(() => {
		cleanup();
		vi.useRealTimers();
	});

	it('applies preset ranges and reports the selected dates', async () => {
		const onchange = vi.fn();

		render(DateRangeFilter, { onchange });

		await fireEvent.change(screen.getByTestId('preset-select'), {
			target: { value: 'LAST_7_DAYS' }
		});

		expect(onchange).toHaveBeenLastCalledWith('2026-06-03', '2026-06-10');
		expect(screen.getByTestId<HTMLSelectElement>('preset-select').value).toBe('LAST_7_DAYS');
	});

	it('shows custom date inputs without presets and emits manual changes', async () => {
		const onchange = vi.fn();

		render(DateRangeFilter, {
			fromDate: '2026-01-01',
			toDate: '2026-01-31',
			showPresets: false,
			onchange
		});

		expect(screen.queryByTestId('preset-select')).not.toBeInTheDocument();
		expect(screen.getByTestId<HTMLInputElement>('from-date').value).toBe('2026-01-01');
		expect(screen.getByTestId<HTMLInputElement>('to-date').value).toBe('2026-01-31');

		await fireEvent.change(screen.getByTestId('from-date'), {
			target: { value: '2026-02-01' }
		});
		expect(onchange).toHaveBeenLastCalledWith('2026-02-01', '2026-01-31');

		await fireEvent.change(screen.getByTestId('to-date'), {
			target: { value: '2026-02-28' }
		});
		expect(onchange).toHaveBeenLastCalledWith('2026-02-01', '2026-02-28');
	});

	it('clears manual dates and returns to all time', async () => {
		const onchange = vi.fn();

		render(DateRangeFilter, {
			fromDate: '2026-03-01',
			toDate: '2026-03-31',
			showPresets: false,
			onchange
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Clear Dates' }));

		expect(screen.getByTestId<HTMLInputElement>('from-date').value).toBe('');
		expect(screen.getByTestId<HTMLInputElement>('to-date').value).toBe('');
		expect(onchange).toHaveBeenLastCalledWith('', '');
	});
});
