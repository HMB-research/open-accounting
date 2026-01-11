import { describe, it, expect } from 'vitest';
import {
	getTodayISO,
	toISODate,
	getStartOfMonth,
	getEndOfMonth,
	getStartOfQuarter,
	getEndOfQuarter,
	getStartOfYear,
	getEndOfYear,
	getDaysAgo,
	calculateDateRange,
	formatDateET,
	formatDate,
	type DatePreset
} from '$lib/utils/dates';

describe('Date Utilities', () => {
	describe('getTodayISO', () => {
		it('returns today in YYYY-MM-DD format', () => {
			const result = getTodayISO();
			expect(result).toMatch(/^\d{4}-\d{2}-\d{2}$/);
		});

		it('returns correct date', () => {
			const result = getTodayISO();
			const expected = new Date().toISOString().slice(0, 10);
			expect(result).toBe(expected);
		});
	});

	describe('toISODate', () => {
		it('converts Date to YYYY-MM-DD string', () => {
			const date = new Date('2024-06-15T12:00:00Z');
			expect(toISODate(date)).toBe('2024-06-15');
		});

		it('handles beginning of year', () => {
			const date = new Date('2024-01-01T12:00:00Z');
			expect(toISODate(date)).toBe('2024-01-01');
		});

		it('handles end of year', () => {
			const date = new Date('2024-12-31T12:00:00Z');
			expect(toISODate(date)).toBe('2024-12-31');
		});
	});

	describe('getStartOfMonth', () => {
		it('returns a valid ISO date for current month start', () => {
			const result = getStartOfMonth();
			// Should be a valid ISO date
			expect(result).toMatch(/^\d{4}-\d{2}-\d{2}$/);
		});

		it('returns a valid ISO date when given explicit date', () => {
			const result = getStartOfMonth(new Date());
			expect(result).toMatch(/^\d{4}-\d{2}-\d{2}$/);
		});
	});

	describe('getEndOfMonth', () => {
		it('returns last day for current month when no arg provided', () => {
			const result = getEndOfMonth();
			// Last day should be between 28-31
			const day = parseInt(result.slice(-2));
			expect(day).toBeGreaterThanOrEqual(28);
			expect(day).toBeLessThanOrEqual(31);
		});

		it('returns a valid ISO date', () => {
			const result = getEndOfMonth(new Date());
			expect(result).toMatch(/^\d{4}-\d{2}-\d{2}$/);
		});
	});

	describe('getStartOfQuarter', () => {
		it('returns a valid ISO date representing quarter start', () => {
			const result = getStartOfQuarter();
			// Should be a valid ISO date
			expect(result).toMatch(/^\d{4}-\d{2}-\d{2}$/);
		});
	});

	describe('getEndOfQuarter', () => {
		it('returns a valid ISO date representing quarter end', () => {
			const result = getEndOfQuarter();
			// Should be a valid ISO date
			expect(result).toMatch(/^\d{4}-\d{2}-\d{2}$/);
		});
	});

	describe('getStartOfYear', () => {
		it('returns a valid ISO date representing year start', () => {
			const result = getStartOfYear();
			// Should return a valid ISO date
			expect(result).toMatch(/^\d{4}-\d{2}-\d{2}$/);
			// Year should be current or previous (timezone edge)
			const resultYear = parseInt(result.slice(0, 4));
			const currentYear = new Date().getFullYear();
			expect(resultYear).toBeGreaterThanOrEqual(currentYear - 1);
			expect(resultYear).toBeLessThanOrEqual(currentYear);
		});
	});

	describe('getEndOfYear', () => {
		it('returns a valid ISO date representing year end', () => {
			const result = getEndOfYear();
			// Should return a valid ISO date
			expect(result).toMatch(/^\d{4}-\d{2}-\d{2}$/);
			// Year should be current or next (timezone edge)
			const resultYear = parseInt(result.slice(0, 4));
			const currentYear = new Date().getFullYear();
			expect(resultYear).toBeGreaterThanOrEqual(currentYear);
			expect(resultYear).toBeLessThanOrEqual(currentYear + 1);
		});
	});

	describe('getDaysAgo', () => {
		it('returns date 7 days ago', () => {
			const from = new Date('2024-06-15T12:00:00Z');
			expect(getDaysAgo(7, from)).toBe('2024-06-08');
		});

		it('returns date 30 days ago', () => {
			const from = new Date('2024-06-15T12:00:00Z');
			expect(getDaysAgo(30, from)).toBe('2024-05-16');
		});

		it('handles month boundary', () => {
			const from = new Date('2024-03-05T12:00:00Z');
			expect(getDaysAgo(10, from)).toBe('2024-02-24');
		});

		it('handles year boundary', () => {
			const from = new Date('2024-01-05T12:00:00Z');
			expect(getDaysAgo(10, from)).toBe('2023-12-26');
		});

		it('returns same date for 0 days', () => {
			const from = new Date('2024-06-15T12:00:00Z');
			expect(getDaysAgo(0, from)).toBe('2024-06-15');
		});

		it('handles February leap year boundary', () => {
			const from = new Date('2024-03-01T12:00:00Z');
			expect(getDaysAgo(1, from)).toBe('2024-02-29');
		});

		it('handles February non-leap year boundary', () => {
			const from = new Date('2023-03-01T12:00:00Z');
			expect(getDaysAgo(1, from)).toBe('2023-02-28');
		});
	});

	describe('calculateDateRange', () => {
		it('returns correct range for TODAY', () => {
			const result = calculateDateRange('TODAY');
			const today = getTodayISO();
			expect(result.from).toBe(today);
			expect(result.to).toBe(today);
		});

		it('returns correct range for LAST_7_DAYS', () => {
			const result = calculateDateRange('LAST_7_DAYS');
			const today = getTodayISO();
			expect(result.to).toBe(today);
			expect(result.from).toBe(getDaysAgo(7));
		});

		it('returns correct range for LAST_30_DAYS', () => {
			const result = calculateDateRange('LAST_30_DAYS');
			const today = getTodayISO();
			expect(result.to).toBe(today);
			expect(result.from).toBe(getDaysAgo(30));
		});

		it('returns correct range for THIS_MONTH', () => {
			const result = calculateDateRange('THIS_MONTH');
			const now = new Date();
			expect(result.from).toBe(getStartOfMonth(now));
			expect(result.to).toBe(getEndOfMonth(now));
		});

		it('returns correct range for LAST_MONTH', () => {
			const result = calculateDateRange('LAST_MONTH');
			const now = new Date();
			const lastMonth = new Date(now.getFullYear(), now.getMonth() - 1, 1);
			expect(result.from).toBe(getStartOfMonth(lastMonth));
			expect(result.to).toBe(getEndOfMonth(lastMonth));
		});

		it('returns correct range for THIS_QUARTER', () => {
			const result = calculateDateRange('THIS_QUARTER');
			const now = new Date();
			expect(result.from).toBe(getStartOfQuarter(now));
			expect(result.to).toBe(getEndOfQuarter(now));
		});

		it('returns correct range for THIS_YEAR', () => {
			const result = calculateDateRange('THIS_YEAR');
			const now = new Date();
			expect(result.from).toBe(getStartOfYear(now));
			expect(result.to).toBe(getEndOfYear(now));
		});

		it('returns empty strings for ALL_TIME', () => {
			const result = calculateDateRange('ALL_TIME');
			expect(result.from).toBe('');
			expect(result.to).toBe('');
		});

		it('returns empty strings for unknown preset', () => {
			const result = calculateDateRange('UNKNOWN' as DatePreset);
			expect(result.from).toBe('');
			expect(result.to).toBe('');
		});
	});

	describe('formatDateET', () => {
		it('returns empty string for empty input', () => {
			expect(formatDateET('')).toBe('');
		});

		it('formats date with Estonian locale', () => {
			const result = formatDateET('2024-06-15');
			// Estonian format is DD.MM.YYYY
			expect(result).toContain('15');
			expect(result).toContain('2024');
		});

		it('handles different dates', () => {
			const result = formatDateET('2024-01-01');
			expect(result).toContain('2024');
			expect(result).toContain('1');
		});
	});

	describe('formatDate', () => {
		it('returns empty string for empty input', () => {
			expect(formatDate('')).toBe('');
		});

		it('formats date with provided locale', () => {
			const result = formatDate('2024-06-15', 'en-US');
			expect(result).toContain('2024');
		});

		it('formats date without locale', () => {
			const result = formatDate('2024-06-15');
			expect(result).toContain('2024');
		});

		it('formats date with Estonian locale', () => {
			const result = formatDate('2024-06-15', 'et-EE');
			expect(result).toContain('15');
		});
	});
});
