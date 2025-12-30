import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { resolve } from 'path';

describe('Translation Completeness', () => {
	const messagesDir = resolve(process.cwd(), 'messages');
	const enMessages: Record<string, string> = JSON.parse(
		readFileSync(resolve(messagesDir, 'en.json'), 'utf-8')
	);
	const etMessages: Record<string, string> = JSON.parse(
		readFileSync(resolve(messagesDir, 'et.json'), 'utf-8')
	);

	it('should have matching keys in English and Estonian', () => {
		const enKeys = Object.keys(enMessages).sort();
		const etKeys = Object.keys(etMessages).sort();

		const missingInEt = enKeys.filter((key) => !etKeys.includes(key));
		const missingInEn = etKeys.filter((key) => !enKeys.includes(key));

		if (missingInEt.length > 0) {
			console.warn('Keys missing in Estonian:', missingInEt);
		}
		if (missingInEn.length > 0) {
			console.warn('Keys missing in English:', missingInEn);
		}

		expect(missingInEt).toEqual([]);
		expect(missingInEn).toEqual([]);
	});

	it('should not have empty translations in English', () => {
		const emptyKeys = Object.entries(enMessages)
			.filter(([, value]) => typeof value === 'string' && value.trim() === '')
			.map(([key]) => key);

		expect(emptyKeys).toEqual([]);
	});

	it('should not have empty translations in Estonian', () => {
		const emptyKeys = Object.entries(etMessages)
			.filter(([, value]) => typeof value === 'string' && value.trim() === '')
			.map(([key]) => key);

		expect(emptyKeys).toEqual([]);
	});

	it('should have valid Estonian characters', () => {
		const estonianChars = /[äöüõÄÖÜÕ]/;

		// Check that at least some Estonian translations contain Estonian characters
		const hasEstonianChars = Object.values(etMessages).some(
			(value) => typeof value === 'string' && estonianChars.test(value)
		);

		expect(hasEstonianChars).toBe(true);
	});

	it('should preserve placeholders in translations', () => {
		const placeholderPattern = /\{(\w+)\}/g;
		const mismatchedPlaceholders: string[] = [];

		for (const key of Object.keys(enMessages)) {
			const enValue = enMessages[key];
			const etValue = etMessages[key];

			if (typeof enValue === 'string' && typeof etValue === 'string') {
				const enPlaceholders = [...enValue.matchAll(placeholderPattern)].map((m) => m[1]).sort();
				const etPlaceholders = [...etValue.matchAll(placeholderPattern)].map((m) => m[1]).sort();

				if (JSON.stringify(enPlaceholders) !== JSON.stringify(etPlaceholders)) {
					mismatchedPlaceholders.push(
						`${key}: EN has [${enPlaceholders.join(', ')}], ET has [${etPlaceholders.join(', ')}]`
					);
				}
			}
		}

		if (mismatchedPlaceholders.length > 0) {
			console.warn('Mismatched placeholders:', mismatchedPlaceholders);
		}

		expect(mismatchedPlaceholders).toEqual([]);
	});

	describe('Required Translation Categories', () => {
		// Check for keys with specific prefixes (flat structure with underscores)
		function hasKeysWithPrefix(messages: Record<string, string>, prefix: string): boolean {
			return Object.keys(messages).some((key) => key.startsWith(prefix));
		}

		it('should have navigation translations', () => {
			expect(hasKeysWithPrefix(enMessages, 'nav_')).toBe(true);
			expect(hasKeysWithPrefix(etMessages, 'nav_')).toBe(true);
		});

		it('should have common translations', () => {
			expect(hasKeysWithPrefix(enMessages, 'common_')).toBe(true);
			expect(hasKeysWithPrefix(etMessages, 'common_')).toBe(true);
		});

		it('should have auth translations', () => {
			expect(hasKeysWithPrefix(enMessages, 'auth_')).toBe(true);
			expect(hasKeysWithPrefix(etMessages, 'auth_')).toBe(true);
		});

		it('should have dashboard translations', () => {
			expect(hasKeysWithPrefix(enMessages, 'dashboard_')).toBe(true);
			expect(hasKeysWithPrefix(etMessages, 'dashboard_')).toBe(true);
		});

		it('should have invoices translations', () => {
			expect(hasKeysWithPrefix(enMessages, 'invoices_')).toBe(true);
			expect(hasKeysWithPrefix(etMessages, 'invoices_')).toBe(true);
		});

		it('should have contacts translations', () => {
			expect(hasKeysWithPrefix(enMessages, 'contacts_')).toBe(true);
			expect(hasKeysWithPrefix(etMessages, 'contacts_')).toBe(true);
		});

		it('should have accounts translations', () => {
			expect(hasKeysWithPrefix(enMessages, 'accounts_')).toBe(true);
			expect(hasKeysWithPrefix(etMessages, 'accounts_')).toBe(true);
		});

		it('should have settings translations', () => {
			expect(hasKeysWithPrefix(enMessages, 'settings_')).toBe(true);
			expect(hasKeysWithPrefix(etMessages, 'settings_')).toBe(true);
		});

		it('should have error translations', () => {
			expect(hasKeysWithPrefix(enMessages, 'errors_')).toBe(true);
			expect(hasKeysWithPrefix(etMessages, 'errors_')).toBe(true);
		});

		it('should have onboarding translations', () => {
			expect(hasKeysWithPrefix(enMessages, 'onboarding_')).toBe(true);
			expect(hasKeysWithPrefix(etMessages, 'onboarding_')).toBe(true);
		});

		it('should have plugin translations', () => {
			expect(hasKeysWithPrefix(enMessages, 'plugins_')).toBe(true);
			expect(hasKeysWithPrefix(etMessages, 'plugins_')).toBe(true);
		});

		it('should have banking import translations', () => {
			expect(hasKeysWithPrefix(enMessages, 'bankingImport_')).toBe(true);
			expect(hasKeysWithPrefix(etMessages, 'bankingImport_')).toBe(true);
		});
	});
});
