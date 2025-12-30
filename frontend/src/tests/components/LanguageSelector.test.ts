import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setLocale, getLocale, locales, baseLocale } from '$lib/paraglide/runtime.js';

// Test the language switching logic without component rendering
// Svelte 5 components require a browser environment for rendering

describe('Language Switching Logic', () => {
	beforeEach(() => {
		// Reset to base locale before each test
		setLocale(baseLocale, { reload: false });
	});

	it('should have correct available locales', () => {
		expect(locales).toContain('en');
		expect(locales).toContain('et');
		expect(locales).toHaveLength(2);
	});

	it('should start with English as base locale', () => {
		expect(baseLocale).toBe('en');
	});

	it('should get the current locale', () => {
		expect(getLocale()).toBe('en');
	});

	it('should switch to Estonian locale', () => {
		setLocale('et', { reload: false });
		expect(getLocale()).toBe('et');
	});

	it('should switch back to English locale', () => {
		setLocale('et', { reload: false });
		expect(getLocale()).toBe('et');

		setLocale('en', { reload: false });
		expect(getLocale()).toBe('en');
	});

	it('should maintain locale after multiple switches', () => {
		setLocale('et', { reload: false });
		setLocale('en', { reload: false });
		setLocale('et', { reload: false });
		expect(getLocale()).toBe('et');
	});
});

describe('Language Selector Configuration', () => {
	it('should have language names defined for all locales', () => {
		const languageNames: Record<string, string> = {
			en: 'English',
			et: 'Eesti'
		};

		for (const locale of locales) {
			expect(languageNames[locale]).toBeDefined();
			expect(languageNames[locale].length).toBeGreaterThan(0);
		}
	});

	it('should have unique language names', () => {
		const languageNames: Record<string, string> = {
			en: 'English',
			et: 'Eesti'
		};

		const names = Object.values(languageNames);
		const uniqueNames = new Set(names);
		expect(uniqueNames.size).toBe(names.length);
	});
});
