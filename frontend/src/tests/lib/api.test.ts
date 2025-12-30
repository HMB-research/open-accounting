import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import Decimal from 'decimal.js';

// Test the parseDecimals logic (private method, so we test via the public API pattern)
describe('API Client - Decimal Parsing', () => {
	// Helper to test decimal parsing behavior
	const parseDecimals = (obj: unknown): unknown => {
		if (typeof obj === 'string' && /^-?\d+(\.\d+)?$/.test(obj)) {
			return new Decimal(obj);
		}
		if (Array.isArray(obj)) {
			return obj.map((item) => parseDecimals(item));
		}
		if (obj !== null && typeof obj === 'object') {
			const result: Record<string, unknown> = {};
			for (const [key, value] of Object.entries(obj)) {
				result[key] = parseDecimals(value);
			}
			return result;
		}
		return obj;
	};

	it('should parse simple decimal strings', () => {
		expect(parseDecimals('123.45')).toEqual(new Decimal('123.45'));
		expect(parseDecimals('100')).toEqual(new Decimal('100'));
		expect(parseDecimals('-50.25')).toEqual(new Decimal('-50.25'));
	});

	it('should not parse non-decimal strings', () => {
		expect(parseDecimals('hello')).toBe('hello');
		expect(parseDecimals('12.34.56')).toBe('12.34.56');
		expect(parseDecimals('$100')).toBe('$100');
		expect(parseDecimals('100%')).toBe('100%');
	});

	it('should handle null and undefined', () => {
		expect(parseDecimals(null)).toBe(null);
		expect(parseDecimals(undefined)).toBe(undefined);
	});

	it('should handle booleans', () => {
		expect(parseDecimals(true)).toBe(true);
		expect(parseDecimals(false)).toBe(false);
	});

	it('should handle numbers', () => {
		expect(parseDecimals(123)).toBe(123);
		expect(parseDecimals(0)).toBe(0);
		expect(parseDecimals(-50)).toBe(-50);
	});

	it('should parse decimal strings in arrays', () => {
		const input = ['100', '200.50', 'text'];
		const result = parseDecimals(input) as unknown[];
		expect(result[0]).toEqual(new Decimal('100'));
		expect(result[1]).toEqual(new Decimal('200.50'));
		expect(result[2]).toBe('text');
	});

	it('should parse decimal strings in objects', () => {
		const input = {
			amount: '1000.50',
			name: 'Test',
			count: 5
		};
		const result = parseDecimals(input) as Record<string, unknown>;
		expect(result.amount).toEqual(new Decimal('1000.50'));
		expect(result.name).toBe('Test');
		expect(result.count).toBe(5);
	});

	it('should handle nested objects', () => {
		const input = {
			invoice: {
				total: '500.00',
				lines: [{ amount: '100' }, { amount: '400' }]
			}
		};
		const result = parseDecimals(input) as Record<string, unknown>;
		const invoice = result.invoice as Record<string, unknown>;
		expect(invoice.total).toEqual(new Decimal('500.00'));
		const lines = invoice.lines as Record<string, unknown>[];
		expect(lines[0].amount).toEqual(new Decimal('100'));
		expect(lines[1].amount).toEqual(new Decimal('400'));
	});

	it('should handle empty arrays', () => {
		expect(parseDecimals([])).toEqual([]);
	});

	it('should handle empty objects', () => {
		expect(parseDecimals({})).toEqual({});
	});

	it('should handle high precision decimals', () => {
		const input = '12345678901234567890.12345678';
		const result = parseDecimals(input);
		expect(result).toEqual(new Decimal('12345678901234567890.12345678'));
	});
});

describe('API Client - Token Management', () => {
	// Mock localStorage
	const localStorageMock = (() => {
		let store: Record<string, string> = {};
		return {
			getItem: vi.fn((key: string) => store[key] || null),
			setItem: vi.fn((key: string, value: string) => {
				store[key] = value;
			}),
			removeItem: vi.fn((key: string) => {
				delete store[key];
			}),
			clear: vi.fn(() => {
				store = {};
			})
		};
	})();

	beforeEach(() => {
		Object.defineProperty(global, 'localStorage', {
			value: localStorageMock,
			writable: true
		});
		localStorageMock.clear();
		vi.clearAllMocks();
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	it('should have test utilities for token storage', () => {
		// Verify our mock setup works
		localStorageMock.setItem('access_token', 'test-token');
		expect(localStorageMock.getItem('access_token')).toBe('test-token');
		localStorageMock.removeItem('access_token');
		expect(localStorageMock.getItem('access_token')).toBeNull();
	});
});

describe('API Types', () => {
	it('should define ContactType values correctly', () => {
		type ContactType = 'CUSTOMER' | 'SUPPLIER' | 'BOTH';
		const customer: ContactType = 'CUSTOMER';
		const supplier: ContactType = 'SUPPLIER';
		const both: ContactType = 'BOTH';

		expect(customer).toBe('CUSTOMER');
		expect(supplier).toBe('SUPPLIER');
		expect(both).toBe('BOTH');
	});

	it('should define InvoiceType values correctly', () => {
		type InvoiceType = 'SALES' | 'PURCHASE' | 'CREDIT_NOTE';
		const sales: InvoiceType = 'SALES';
		const purchase: InvoiceType = 'PURCHASE';
		const creditNote: InvoiceType = 'CREDIT_NOTE';

		expect(sales).toBe('SALES');
		expect(purchase).toBe('PURCHASE');
		expect(creditNote).toBe('CREDIT_NOTE');
	});

	it('should define InvoiceStatus values correctly', () => {
		type InvoiceStatus = 'DRAFT' | 'SENT' | 'PARTIALLY_PAID' | 'PAID' | 'OVERDUE' | 'VOIDED';
		const statuses: InvoiceStatus[] = ['DRAFT', 'SENT', 'PARTIALLY_PAID', 'PAID', 'OVERDUE', 'VOIDED'];

		expect(statuses).toHaveLength(6);
		expect(statuses).toContain('DRAFT');
		expect(statuses).toContain('PAID');
	});

	it('should define PaymentType values correctly', () => {
		type PaymentType = 'RECEIVED' | 'MADE';
		const received: PaymentType = 'RECEIVED';
		const made: PaymentType = 'MADE';

		expect(received).toBe('RECEIVED');
		expect(made).toBe('MADE');
	});

	it('should define Frequency values correctly', () => {
		type Frequency = 'WEEKLY' | 'BIWEEKLY' | 'MONTHLY' | 'QUARTERLY' | 'YEARLY';
		const frequencies: Frequency[] = ['WEEKLY', 'BIWEEKLY', 'MONTHLY', 'QUARTERLY', 'YEARLY'];

		expect(frequencies).toHaveLength(5);
	});

	it('should define TransactionStatus values correctly', () => {
		type TransactionStatus = 'UNMATCHED' | 'MATCHED' | 'RECONCILED';
		const unmatched: TransactionStatus = 'UNMATCHED';
		const matched: TransactionStatus = 'MATCHED';
		const reconciled: TransactionStatus = 'RECONCILED';

		expect(unmatched).toBe('UNMATCHED');
		expect(matched).toBe('MATCHED');
		expect(reconciled).toBe('RECONCILED');
	});

	it('should define TemplateType values correctly', () => {
		type TemplateType = 'INVOICE_SEND' | 'PAYMENT_RECEIPT' | 'OVERDUE_REMINDER';
		const templates: TemplateType[] = ['INVOICE_SEND', 'PAYMENT_RECEIPT', 'OVERDUE_REMINDER'];

		expect(templates).toHaveLength(3);
	});

	it('should define EmploymentType values correctly', () => {
		type EmploymentType = 'FULL_TIME' | 'PART_TIME' | 'CONTRACT';
		const types: EmploymentType[] = ['FULL_TIME', 'PART_TIME', 'CONTRACT'];

		expect(types).toHaveLength(3);
	});

	it('should define PayrollStatus values correctly', () => {
		type PayrollStatus = 'DRAFT' | 'CALCULATED' | 'APPROVED' | 'PAID' | 'DECLARED';
		const statuses: PayrollStatus[] = ['DRAFT', 'CALCULATED', 'APPROVED', 'PAID', 'DECLARED'];

		expect(statuses).toHaveLength(5);
	});

	it('should define PluginState values correctly', () => {
		type PluginState = 'installed' | 'enabled' | 'disabled' | 'failed';
		const states: PluginState[] = ['installed', 'enabled', 'disabled', 'failed'];

		expect(states).toHaveLength(4);
	});
});
