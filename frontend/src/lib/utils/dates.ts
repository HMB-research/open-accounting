/**
 * Date utility functions for consistent date handling across the application.
 * All dates use ISO 8601 format (YYYY-MM-DD).
 */

export type DatePreset =
	| 'TODAY'
	| 'LAST_7_DAYS'
	| 'LAST_30_DAYS'
	| 'THIS_MONTH'
	| 'LAST_MONTH'
	| 'THIS_QUARTER'
	| 'THIS_YEAR'
	| 'ALL_TIME';

/**
 * Get today's date in ISO format (YYYY-MM-DD)
 */
export function getTodayISO(): string {
	return new Date().toISOString().slice(0, 10);
}

/**
 * Format a Date to ISO date string (YYYY-MM-DD)
 */
export function toISODate(date: Date): string {
	return date.toISOString().slice(0, 10);
}

/**
 * Get the first day of the month for a given date
 */
export function getStartOfMonth(date: Date = new Date()): string {
	return toISODate(new Date(date.getFullYear(), date.getMonth(), 1));
}

/**
 * Get the last day of the month for a given date
 */
export function getEndOfMonth(date: Date = new Date()): string {
	return toISODate(new Date(date.getFullYear(), date.getMonth() + 1, 0));
}

/**
 * Get the first day of the quarter for a given date
 */
export function getStartOfQuarter(date: Date = new Date()): string {
	const quarterStart = Math.floor(date.getMonth() / 3) * 3;
	return toISODate(new Date(date.getFullYear(), quarterStart, 1));
}

/**
 * Get the last day of the quarter for a given date
 */
export function getEndOfQuarter(date: Date = new Date()): string {
	const quarterStart = Math.floor(date.getMonth() / 3) * 3;
	return toISODate(new Date(date.getFullYear(), quarterStart + 3, 0));
}

/**
 * Get the first day of the year for a given date
 */
export function getStartOfYear(date: Date = new Date()): string {
	return toISODate(new Date(date.getFullYear(), 0, 1));
}

/**
 * Get the last day of the year for a given date
 */
export function getEndOfYear(date: Date = new Date()): string {
	return toISODate(new Date(date.getFullYear(), 11, 31));
}

/**
 * Get a date X days ago
 */
export function getDaysAgo(days: number, from: Date = new Date()): string {
	const date = new Date(from);
	date.setDate(date.getDate() - days);
	return toISODate(date);
}

/**
 * Calculate date range for a preset period
 */
export function calculateDateRange(preset: DatePreset): { from: string; to: string } {
	const now = new Date();
	const today = getTodayISO();

	switch (preset) {
		case 'TODAY':
			return { from: today, to: today };

		case 'LAST_7_DAYS':
			return { from: getDaysAgo(7), to: today };

		case 'LAST_30_DAYS':
			return { from: getDaysAgo(30), to: today };

		case 'THIS_MONTH':
			return { from: getStartOfMonth(now), to: getEndOfMonth(now) };

		case 'LAST_MONTH': {
			const lastMonth = new Date(now.getFullYear(), now.getMonth() - 1, 1);
			return { from: getStartOfMonth(lastMonth), to: getEndOfMonth(lastMonth) };
		}

		case 'THIS_QUARTER':
			return { from: getStartOfQuarter(now), to: getEndOfQuarter(now) };

		case 'THIS_YEAR':
			return { from: getStartOfYear(now), to: getEndOfYear(now) };

		case 'ALL_TIME':
			return { from: '', to: '' };

		default:
			return { from: '', to: '' };
	}
}

/**
 * Format a date string for display using Estonian locale
 */
export function formatDateET(dateStr: string): string {
	if (!dateStr) return '';
	return new Date(dateStr).toLocaleDateString('et-EE');
}

/**
 * Format a date string for display using user's locale
 */
export function formatDate(dateStr: string, locale?: string): string {
	if (!dateStr) return '';
	return new Date(dateStr).toLocaleDateString(locale);
}
