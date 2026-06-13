import { describe, expect, it } from 'vitest';
import { getLastCompletedFiscalYearEnd } from '$lib/review/workspace';

describe('review workspace helpers', () => {
	it('returns the previous calendar year end for January fiscal years', () => {
		expect(getLastCompletedFiscalYearEnd(1, new Date('2026-06-13T12:00:00Z'))).toBe('2025-12-31');
	});

	it('returns the previous completed custom fiscal year end', () => {
		expect(getLastCompletedFiscalYearEnd(7, new Date('2026-08-15T12:00:00Z'))).toBe('2026-06-30');
		expect(getLastCompletedFiscalYearEnd(7, new Date('2026-06-13T12:00:00Z'))).toBe('2025-06-30');
	});
});
