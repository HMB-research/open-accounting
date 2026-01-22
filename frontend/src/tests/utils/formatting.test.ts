import { describe, it, expect } from 'vitest';
import Decimal from 'decimal.js';
import {
	formatCurrency,
	formatDate,
	calculateLineTotal,
	calculateLinesTotal,
	createEmptyLine,
	type LineItem
} from '$lib/utils/formatting';

describe('Formatting Utilities', () => {
	describe('formatCurrency', () => {
		it('formats number values correctly', () => {
			const result = formatCurrency(100);
			expect(result).toContain('100');
			// Currency symbol can be EUR or the euro sign depending on locale implementation
			expect(result.includes('EUR') || result.includes('\u20ac')).toBe(true);
		});

		it('formats string values correctly', () => {
			const result = formatCurrency('250.50');
			expect(result).toContain('250');
		});

		it('formats Decimal values correctly', () => {
			const decimal = new Decimal('1234.56');
			const result = formatCurrency(decimal);
			expect(result).toContain('1');
			expect(result).toContain('234');
		});

		it('uses custom locale when provided', () => {
			const result = formatCurrency(1000, { locale: 'en-US', currency: 'USD' });
			expect(result).toContain('$');
			expect(result).toContain('1,000');
		});

		it('defaults to Estonian locale and EUR', () => {
			const result = formatCurrency(1000);
			// Estonian locale uses space as thousand separator
			expect(result).toMatch(/1[\s\u00a0]?000/);
		});

		it('handles zero correctly', () => {
			const result = formatCurrency(0);
			expect(result).toContain('0');
		});

		it('handles negative values correctly', () => {
			const result = formatCurrency(-100);
			expect(result).toContain('100');
		});
	});

	describe('formatDate', () => {
		it('formats date string correctly with default locale', () => {
			const result = formatDate('2024-06-15');
			expect(result).toContain('15');
			expect(result).toContain('2024');
		});

		it('uses custom locale when provided', () => {
			const result = formatDate('2024-06-15', 'en-US');
			expect(result).toContain('2024');
		});

		it('handles different date formats', () => {
			const result = formatDate('2024-01-01');
			expect(result).toContain('1');
			expect(result).toContain('2024');
		});

		it('handles end of year dates', () => {
			const result = formatDate('2024-12-31');
			expect(result).toContain('31');
			expect(result).toContain('12');
			expect(result).toContain('2024');
		});
	});

	describe('calculateLineTotal', () => {
		it('calculates simple line total without discount', () => {
			const line: LineItem = {
				description: 'Test',
				quantity: '2',
				unit_price: '100',
				vat_rate: '0',
				discount_percent: '0'
			};
			const result = calculateLineTotal(line);
			expect(result.toNumber()).toBe(200);
		});

		it('calculates line total with VAT', () => {
			const line: LineItem = {
				description: 'Test',
				quantity: '1',
				unit_price: '100',
				vat_rate: '22',
				discount_percent: '0'
			};
			const result = calculateLineTotal(line);
			expect(result.toNumber()).toBe(122);
		});

		it('calculates line total with discount', () => {
			const line: LineItem = {
				description: 'Test',
				quantity: '1',
				unit_price: '100',
				vat_rate: '0',
				discount_percent: '10'
			};
			const result = calculateLineTotal(line);
			expect(result.toNumber()).toBe(90);
		});

		it('calculates line total with both VAT and discount', () => {
			const line: LineItem = {
				description: 'Test',
				quantity: '1',
				unit_price: '100',
				vat_rate: '22',
				discount_percent: '10'
			};
			const result = calculateLineTotal(line);
			// (100 - 10% discount) = 90, then + 22% VAT = 109.80
			expect(result.toNumber()).toBe(109.8);
		});

		it('handles decimal quantities', () => {
			const line: LineItem = {
				description: 'Test',
				quantity: '2.5',
				unit_price: '10',
				vat_rate: '0',
				discount_percent: '0'
			};
			const result = calculateLineTotal(line);
			expect(result.toNumber()).toBe(25);
		});

		it('handles decimal prices', () => {
			const line: LineItem = {
				description: 'Test',
				quantity: '1',
				unit_price: '99.99',
				vat_rate: '0',
				discount_percent: '0'
			};
			const result = calculateLineTotal(line);
			expect(result.toNumber()).toBe(99.99);
		});

		it('handles empty/zero values', () => {
			const line: LineItem = {
				description: 'Test',
				quantity: '0',
				unit_price: '100',
				vat_rate: '22',
				discount_percent: '0'
			};
			const result = calculateLineTotal(line);
			expect(result.toNumber()).toBe(0);
		});

		it('handles empty string values', () => {
			const line: LineItem = {
				description: 'Test',
				quantity: '',
				unit_price: '',
				vat_rate: '',
				discount_percent: ''
			};
			const result = calculateLineTotal(line);
			expect(result.toNumber()).toBe(0);
		});
	});

	describe('calculateLinesTotal', () => {
		it('calculates total for empty array', () => {
			const result = calculateLinesTotal([]);
			expect(result.toNumber()).toBe(0);
		});

		it('calculates total for single line', () => {
			const lines: LineItem[] = [
				{
					description: 'Test',
					quantity: '1',
					unit_price: '100',
					vat_rate: '22',
					discount_percent: '0'
				}
			];
			const result = calculateLinesTotal(lines);
			expect(result.toNumber()).toBe(122);
		});

		it('calculates total for multiple lines', () => {
			const lines: LineItem[] = [
				{
					description: 'Item 1',
					quantity: '1',
					unit_price: '100',
					vat_rate: '0',
					discount_percent: '0'
				},
				{
					description: 'Item 2',
					quantity: '2',
					unit_price: '50',
					vat_rate: '0',
					discount_percent: '0'
				}
			];
			const result = calculateLinesTotal(lines);
			expect(result.toNumber()).toBe(200);
		});

		it('calculates total for lines with different VAT rates', () => {
			const lines: LineItem[] = [
				{
					description: 'Item 1',
					quantity: '1',
					unit_price: '100',
					vat_rate: '22',
					discount_percent: '0'
				},
				{
					description: 'Item 2',
					quantity: '1',
					unit_price: '100',
					vat_rate: '9',
					discount_percent: '0'
				}
			];
			const result = calculateLinesTotal(lines);
			// 122 + 109 = 231
			expect(result.toNumber()).toBe(231);
		});
	});

	describe('createEmptyLine', () => {
		it('creates a line with default values', () => {
			const line = createEmptyLine();
			expect(line.description).toBe('');
			expect(line.quantity).toBe('1');
			expect(line.unit_price).toBe('0');
			expect(line.vat_rate).toBe('22');
			expect(line.discount_percent).toBe('0');
		});

		it('allows custom default VAT rate', () => {
			const line = createEmptyLine('9');
			expect(line.vat_rate).toBe('9');
		});

		it('allows zero VAT rate', () => {
			const line = createEmptyLine('0');
			expect(line.vat_rate).toBe('0');
		});
	});
});
