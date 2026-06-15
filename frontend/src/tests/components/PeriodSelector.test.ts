import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { baseLocale, setLocale } from '$lib/paraglide/runtime.js';
import PeriodSelector from '$lib/components/PeriodSelector.svelte';

describe('PeriodSelector', () => {
	beforeEach(() => {
		setLocale(baseLocale, { reload: false });
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2026-06-10T12:00:00Z'));
	});

	afterEach(() => {
		cleanup();
		vi.useRealTimers();
	});

	it('uses shared date ranges for predefined periods', async () => {
		const onchange = vi.fn();

		render(PeriodSelector, { onchange });

		await fireEvent.change(screen.getByTestId('period-select'), {
			target: { value: 'THIS_QUARTER' }
		});

		expect(onchange).toHaveBeenLastCalledWith('THIS_QUARTER', '2026-04-01', '2026-06-30');

		await fireEvent.change(screen.getByTestId('period-select'), {
			target: { value: 'LAST_MONTH' }
		});

		expect(onchange).toHaveBeenLastCalledWith('LAST_MONTH', '2026-05-01', '2026-05-31');
	});

	it('renders custom date inputs and reports manual custom dates', async () => {
		const onchange = vi.fn();

		render(PeriodSelector, {
			value: 'CUSTOM',
			startDate: '2026-02-01',
			endDate: '2026-02-28',
			onchange
		});

		expect(screen.getByTestId<HTMLInputElement>('date-start').value).toBe('2026-02-01');
		expect(screen.getByTestId<HTMLInputElement>('date-end').value).toBe('2026-02-28');

		await fireEvent.change(screen.getByTestId('date-start'), {
			target: { value: '2026-03-01' }
		});
		expect(onchange).toHaveBeenLastCalledWith('CUSTOM', '2026-03-01', '2026-02-28');

		await fireEvent.change(screen.getByTestId('date-end'), {
			target: { value: '2026-03-31' }
		});
		expect(onchange).toHaveBeenLastCalledWith('CUSTOM', '2026-03-01', '2026-03-31');
	});
});
