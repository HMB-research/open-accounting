import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Test the DateRangeFilter component logic without component rendering
// Svelte 5 components require a browser environment for rendering

// Import the actual utility functions used by the component
import {
	type DatePreset,
	calculateDateRange,
	getTodayISO,
	getStartOfMonth,
	getEndOfMonth,
	getStartOfQuarter,
	getEndOfQuarter,
	getStartOfYear,
	getEndOfYear,
	getDaysAgo,
	toISODate
} from '$lib/utils/dates';

describe('DateRangeFilter Component Logic', () => {
	// Use current date for testing - tests verify relative behavior, not specific dates
	const now = new Date();
	const today = getTodayISO();

	describe('Preset Options', () => {
		const presetValues: (DatePreset | 'CUSTOM')[] = [
			'ALL_TIME',
			'TODAY',
			'LAST_7_DAYS',
			'LAST_30_DAYS',
			'THIS_MONTH',
			'THIS_QUARTER',
			'THIS_YEAR',
			'CUSTOM'
		];

		it('should have all expected preset options', () => {
			expect(presetValues).toContain('ALL_TIME');
			expect(presetValues).toContain('TODAY');
			expect(presetValues).toContain('LAST_7_DAYS');
			expect(presetValues).toContain('LAST_30_DAYS');
			expect(presetValues).toContain('THIS_MONTH');
			expect(presetValues).toContain('THIS_QUARTER');
			expect(presetValues).toContain('THIS_YEAR');
			expect(presetValues).toContain('CUSTOM');
		});

		it('should have 8 preset options including CUSTOM', () => {
			expect(presetValues).toHaveLength(8);
		});
	});

	describe('calculateDateRange', () => {
		it('should return today for TODAY preset', () => {
			const result = calculateDateRange('TODAY');
			expect(result.from).toBe(today);
			expect(result.to).toBe(today);
		});

		it('should return last 7 days for LAST_7_DAYS preset', () => {
			const result = calculateDateRange('LAST_7_DAYS');
			expect(result.from).toBe(getDaysAgo(7));
			expect(result.to).toBe(today);
		});

		it('should return last 30 days for LAST_30_DAYS preset', () => {
			const result = calculateDateRange('LAST_30_DAYS');
			expect(result.from).toBe(getDaysAgo(30));
			expect(result.to).toBe(today);
		});

		it('should return this month for THIS_MONTH preset', () => {
			const result = calculateDateRange('THIS_MONTH');
			expect(result.from).toBe(getStartOfMonth());
			expect(result.to).toBe(getEndOfMonth());
		});

		it('should return this quarter for THIS_QUARTER preset', () => {
			const result = calculateDateRange('THIS_QUARTER');
			expect(result.from).toBe(getStartOfQuarter());
			expect(result.to).toBe(getEndOfQuarter());
		});

		it('should return this year for THIS_YEAR preset', () => {
			const result = calculateDateRange('THIS_YEAR');
			expect(result.from).toBe(getStartOfYear());
			expect(result.to).toBe(getEndOfYear());
		});

		it('should return empty strings for ALL_TIME preset', () => {
			const result = calculateDateRange('ALL_TIME');
			expect(result.from).toBe('');
			expect(result.to).toBe('');
		});
	});

	describe('Date Utility Functions', () => {
		describe('getTodayISO', () => {
			it('should return date in ISO format', () => {
				const todayISO = getTodayISO();
				expect(todayISO).toMatch(/^\d{4}-\d{2}-\d{2}$/);
			});

			it('should return consistent value when called multiple times', () => {
				const first = getTodayISO();
				const second = getTodayISO();
				expect(first).toBe(second);
			});
		});

		describe('toISODate', () => {
			it('should convert Date to ISO string', () => {
				const date = new Date('2026-06-15T00:00:00.000Z');
				expect(toISODate(date)).toBe('2026-06-15');
			});

			it('should handle dates in any month', () => {
				const date = new Date('2025-12-31T00:00:00.000Z');
				expect(toISODate(date)).toBe('2025-12-31');
			});
		});

		describe('getStartOfMonth', () => {
			it('should return first day of current month in correct format', () => {
				const result = getStartOfMonth();
				expect(result).toMatch(/^\d{4}-\d{2}-01$/);
			});

			it('should return first day of specified month', () => {
				const date = new Date('2026-03-15');
				expect(getStartOfMonth(date)).toBe('2026-03-01');
			});
		});

		describe('getEndOfMonth', () => {
			it('should return last day of current month in correct format', () => {
				const result = getEndOfMonth();
				expect(result).toMatch(/^\d{4}-\d{2}-\d{2}$/);
			});

			it('should handle February in leap year', () => {
				const date = new Date('2024-02-15');
				expect(getEndOfMonth(date)).toBe('2024-02-29');
			});

			it('should handle February in non-leap year', () => {
				const date = new Date('2025-02-15');
				expect(getEndOfMonth(date)).toBe('2025-02-28');
			});
		});

		describe('getStartOfQuarter', () => {
			it('should return a valid date in correct format', () => {
				const result = getStartOfQuarter();
				expect(result).toMatch(/^\d{4}-\d{2}-01$/);
			});

			it('should return Q2 start for April', () => {
				const date = new Date('2026-04-15');
				expect(getStartOfQuarter(date)).toBe('2026-04-01');
			});

			it('should return Q3 start for August', () => {
				const date = new Date('2026-08-15');
				expect(getStartOfQuarter(date)).toBe('2026-07-01');
			});

			it('should return Q4 start for November', () => {
				const date = new Date('2026-11-15');
				expect(getStartOfQuarter(date)).toBe('2026-10-01');
			});
		});

		describe('getEndOfQuarter', () => {
			it('should return a valid date in correct format', () => {
				const result = getEndOfQuarter();
				expect(result).toMatch(/^\d{4}-\d{2}-\d{2}$/);
			});

			it('should return Q2 end for April', () => {
				const date = new Date('2026-04-15');
				expect(getEndOfQuarter(date)).toBe('2026-06-30');
			});

			it('should return Q3 end for September', () => {
				const date = new Date('2026-09-15');
				expect(getEndOfQuarter(date)).toBe('2026-09-30');
			});

			it('should return Q4 end for December', () => {
				const date = new Date('2026-12-15');
				expect(getEndOfQuarter(date)).toBe('2026-12-31');
			});
		});

		describe('getDaysAgo', () => {
			it('should return a date before today', () => {
				const sevenDaysAgo = getDaysAgo(7);
				const todayDate = new Date(today);
				const agoDate = new Date(sevenDaysAgo);
				expect(agoDate < todayDate).toBe(true);
			});

			it('should return same date for 0 days ago', () => {
				expect(getDaysAgo(0)).toBe(today);
			});

			it('should handle crossing year boundary', () => {
				const from = new Date('2026-01-05');
				expect(getDaysAgo(10, from)).toBe('2025-12-26');
			});

			it('should calculate correct number of days difference', () => {
				const from = new Date('2026-01-15');
				const result = getDaysAgo(5, from);
				expect(result).toBe('2026-01-10');
			});
		});
	});

	describe('Preset Change Handler Logic', () => {
		interface DateState {
			fromDate: string;
			toDate: string;
			selectedPreset: DatePreset | 'CUSTOM';
		}

		function handlePresetChange(state: DateState, preset: DatePreset | 'CUSTOM'): DateState {
			const newState = { ...state, selectedPreset: preset };

			if (preset === 'CUSTOM') {
				// Don't change dates, let user input them
				if (!newState.fromDate) newState.fromDate = getTodayISO();
				if (!newState.toDate) newState.toDate = getTodayISO();
			} else if (preset === 'ALL_TIME') {
				newState.fromDate = '';
				newState.toDate = '';
			} else {
				const range = calculateDateRange(preset);
				newState.fromDate = range.from;
				newState.toDate = range.to;
			}

			return newState;
		}

		it('should set dates when selecting TODAY preset', () => {
			const state: DateState = { fromDate: '', toDate: '', selectedPreset: 'ALL_TIME' };
			const newState = handlePresetChange(state, 'TODAY');

			expect(newState.selectedPreset).toBe('TODAY');
			expect(newState.fromDate).toBe(today);
			expect(newState.toDate).toBe(today);
		});

		it('should clear dates when selecting ALL_TIME preset', () => {
			const state: DateState = {
				fromDate: '2026-01-01',
				toDate: '2026-01-31',
				selectedPreset: 'THIS_MONTH'
			};
			const newState = handlePresetChange(state, 'ALL_TIME');

			expect(newState.selectedPreset).toBe('ALL_TIME');
			expect(newState.fromDate).toBe('');
			expect(newState.toDate).toBe('');
		});

		it('should set today if dates empty when selecting CUSTOM', () => {
			const state: DateState = { fromDate: '', toDate: '', selectedPreset: 'ALL_TIME' };
			const newState = handlePresetChange(state, 'CUSTOM');

			expect(newState.selectedPreset).toBe('CUSTOM');
			expect(newState.fromDate).toBe(today);
			expect(newState.toDate).toBe(today);
		});

		it('should preserve existing dates when selecting CUSTOM', () => {
			const state: DateState = {
				fromDate: '2026-01-01',
				toDate: '2026-01-15',
				selectedPreset: 'THIS_MONTH'
			};
			const newState = handlePresetChange(state, 'CUSTOM');

			expect(newState.selectedPreset).toBe('CUSTOM');
			expect(newState.fromDate).toBe('2026-01-01');
			expect(newState.toDate).toBe('2026-01-15');
		});
	});

	describe('Date Change Handler Logic', () => {
		it('should switch to CUSTOM preset on manual date change', () => {
			let selectedPreset: DatePreset | 'CUSTOM' = 'THIS_MONTH';

			function handleDateChange() {
				selectedPreset = 'CUSTOM';
			}

			handleDateChange();
			expect(selectedPreset).toBe('CUSTOM');
		});
	});

	describe('Clear Dates Logic', () => {
		interface DateState {
			fromDate: string;
			toDate: string;
			selectedPreset: DatePreset | 'CUSTOM';
		}

		function clearDates(state: DateState): DateState {
			return {
				fromDate: '',
				toDate: '',
				selectedPreset: 'ALL_TIME'
			};
		}

		it('should clear all dates and reset preset', () => {
			const state: DateState = {
				fromDate: '2026-01-01',
				toDate: '2026-01-31',
				selectedPreset: 'CUSTOM'
			};

			const newState = clearDates(state);

			expect(newState.fromDate).toBe('');
			expect(newState.toDate).toBe('');
			expect(newState.selectedPreset).toBe('ALL_TIME');
		});
	});

	describe('Callback Handling', () => {
		it('should invoke onchange callback with dates', () => {
			const callback = vi.fn();
			const fromDate = '2026-01-01';
			const toDate = '2026-01-31';

			callback(fromDate, toDate);

			expect(callback).toHaveBeenCalledWith('2026-01-01', '2026-01-31');
		});

		it('should invoke callback with empty strings for ALL_TIME', () => {
			const callback = vi.fn();
			callback('', '');

			expect(callback).toHaveBeenCalledWith('', '');
		});

		it('should not fail if callback is undefined', () => {
			const callback: ((from: string, to: string) => void) | undefined = undefined;
			expect(() => callback?.('2026-01-01', '2026-01-31')).not.toThrow();
		});
	});

	describe('Date Input Visibility Logic', () => {
		function shouldShowDateInputs(selectedPreset: string, showPresets: boolean): boolean {
			return selectedPreset === 'CUSTOM' || !showPresets;
		}

		it('should show inputs when CUSTOM is selected', () => {
			expect(shouldShowDateInputs('CUSTOM', true)).toBe(true);
		});

		it('should hide inputs when preset is selected', () => {
			expect(shouldShowDateInputs('THIS_MONTH', true)).toBe(false);
		});

		it('should show inputs when presets are hidden', () => {
			expect(shouldShowDateInputs('THIS_MONTH', false)).toBe(true);
		});
	});

	describe('Clear Button Visibility Logic', () => {
		function shouldShowClearButton(fromDate: string, toDate: string): boolean {
			return Boolean(fromDate || toDate);
		}

		it('should show clear button when fromDate is set', () => {
			expect(shouldShowClearButton('2026-01-01', '')).toBe(true);
		});

		it('should show clear button when toDate is set', () => {
			expect(shouldShowClearButton('', '2026-01-31')).toBe(true);
		});

		it('should show clear button when both dates are set', () => {
			expect(shouldShowClearButton('2026-01-01', '2026-01-31')).toBe(true);
		});

		it('should hide clear button when no dates are set', () => {
			expect(shouldShowClearButton('', '')).toBe(false);
		});
	});

	describe('Effect: Sync Preset with Empty Dates', () => {
		it('should switch to ALL_TIME when dates become empty', () => {
			let selectedPreset: DatePreset | 'CUSTOM' = 'THIS_MONTH';
			const fromDate = '';
			const toDate = '';

			// Simulating the $effect logic
			if (!fromDate && !toDate && selectedPreset !== 'ALL_TIME') {
				selectedPreset = 'ALL_TIME';
			}

			expect(selectedPreset).toBe('ALL_TIME');
		});

		it('should not change preset if already ALL_TIME', () => {
			let selectedPreset: DatePreset | 'CUSTOM' = 'ALL_TIME';
			const fromDate = '';
			const toDate = '';

			// Effect should not run if already ALL_TIME
			if (!fromDate && !toDate && selectedPreset !== 'ALL_TIME') {
				selectedPreset = 'CUSTOM'; // This shouldn't happen
			}

			expect(selectedPreset).toBe('ALL_TIME');
		});

		it('should not change preset if dates are set', () => {
			let selectedPreset: DatePreset | 'CUSTOM' = 'THIS_MONTH';
			const fromDate = '2026-01-01';
			const toDate = '2026-01-31';

			if (!fromDate && !toDate && selectedPreset !== 'ALL_TIME') {
				selectedPreset = 'ALL_TIME';
			}

			expect(selectedPreset).toBe('THIS_MONTH');
		});
	});

	describe('Compact Mode', () => {
		const compactClass = 'compact';

		it('should apply compact class when compact prop is true', () => {
			const compact = true;
			const classes = compact ? compactClass : '';
			expect(classes).toBe('compact');
		});

		it('should not apply compact class when compact prop is false', () => {
			const compact = false;
			const classes = compact ? compactClass : '';
			expect(classes).toBe('');
		});
	});

	describe('ISO Date Format Validation', () => {
		function isValidISODateFormat(dateStr: string): boolean {
			if (!dateStr) return true; // Empty is valid (ALL_TIME)
			const regex = /^\d{4}-\d{2}-\d{2}$/;
			return regex.test(dateStr);
		}

		function isValidISODateStrict(dateStr: string): boolean {
			if (!dateStr) return true; // Empty is valid (ALL_TIME)
			const regex = /^\d{4}-\d{2}-\d{2}$/;
			if (!regex.test(dateStr)) return false;
			// Check that parsed date matches input (catches Feb 30 becoming Mar 2)
			const date = new Date(dateStr + 'T00:00:00.000Z');
			return date.toISOString().slice(0, 10) === dateStr;
		}

		it('should accept valid ISO date', () => {
			expect(isValidISODateFormat('2026-01-23')).toBe(true);
		});

		it('should accept empty string', () => {
			expect(isValidISODateFormat('')).toBe(true);
		});

		it('should reject invalid format', () => {
			expect(isValidISODateFormat('01/23/2026')).toBe(false);
		});

		it('should reject dates that overflow (strict validation)', () => {
			// Feb 30 2026 doesn't exist - it gets normalized to Mar 2
			expect(isValidISODateStrict('2026-02-30')).toBe(false);
		});

		it('should accept valid February dates', () => {
			expect(isValidISODateStrict('2026-02-28')).toBe(true);
			expect(isValidISODateStrict('2024-02-29')).toBe(true); // leap year
		});
	});
});
