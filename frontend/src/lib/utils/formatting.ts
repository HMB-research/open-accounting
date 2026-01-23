import Decimal from 'decimal.js';

/**
 * Formatting utility functions extracted from duplicated code across
 * invoices, quotes, and orders pages.
 */

export interface FormatCurrencyOptions {
	locale?: string;
	currency?: string;
}

export interface LineItem {
	description: string;
	quantity: string;
	unit_price: string;
	vat_rate: string;
	discount_percent: string;
}

/**
 * Format a value as currency using Intl.NumberFormat.
 * Handles Decimal.js instances, numbers, and string values.
 *
 * @param value - The value to format (Decimal, number, or string)
 * @param options - Formatting options (locale, currency)
 * @returns Formatted currency string
 */
export function formatCurrency(
	value: Decimal | number | string,
	options: FormatCurrencyOptions = {}
): string {
	const { locale = 'et-EE', currency = 'EUR' } = options;

	// Handle Decimal.js instances
	const num = value instanceof Decimal ? value.toNumber() : Number(value);

	return new Intl.NumberFormat(locale, {
		style: 'currency',
		currency
	}).format(num);
}

/**
 * Format a date string for display.
 *
 * @param dateStr - ISO date string
 * @param locale - Locale for formatting (default: 'et-EE')
 * @returns Formatted date string
 */
export function formatDate(dateStr: string, locale: string = 'et-EE'): string {
	return new Date(dateStr).toLocaleDateString(locale);
}

/**
 * Calculate the total for a single line item including VAT and discount.
 *
 * Formula: ((qty * price) - discount) + VAT
 * Where discount = (qty * price) * discount_percent / 100
 * And VAT = subtotal * vat_rate / 100
 *
 * @param line - Line item with quantity, unit_price, vat_rate, and discount_percent
 * @returns Total as Decimal
 */
export function calculateLineTotal(line: LineItem): Decimal {
	const qty = new Decimal(line.quantity || 0);
	const price = new Decimal(line.unit_price || 0);
	const discount = new Decimal(line.discount_percent || 0);
	const vat = new Decimal(line.vat_rate || 0);

	const gross = qty.mul(price);
	const discountAmt = gross.mul(discount).div(100);
	const subtotal = gross.minus(discountAmt);
	const vatAmt = subtotal.mul(vat).div(100);
	return subtotal.plus(vatAmt);
}

/**
 * Calculate the total for an array of line items.
 *
 * @param lines - Array of line items
 * @returns Total as Decimal
 */
export function calculateLinesTotal(lines: LineItem[]): Decimal {
	return lines.reduce((sum, line) => sum.plus(calculateLineTotal(line)), new Decimal(0));
}

/**
 * Create a default empty line item.
 * Used when initializing forms with a first line item.
 *
 * @param defaultVatRate - Default VAT rate (default: '22')
 * @returns A new line item with default values
 */
export function createEmptyLine(defaultVatRate: string = '22'): LineItem {
	return {
		description: '',
		quantity: '1',
		unit_price: '0',
		vat_rate: defaultVatRate,
		discount_percent: '0'
	};
}
