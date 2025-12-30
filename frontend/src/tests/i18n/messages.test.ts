import { describe, it, expect, beforeEach } from 'vitest';
import * as m from '$lib/paraglide/messages.js';
import { setLocale, getLocale, locales, baseLocale } from '$lib/paraglide/runtime.js';

describe('i18n Messages', () => {
	beforeEach(() => {
		// Reset to base locale before each test
		setLocale(baseLocale, { reload: false });
	});

	describe('Locale Configuration', () => {
		it('should have English as base locale', () => {
			expect(baseLocale).toBe('en');
		});

		it('should support English and Estonian locales', () => {
			expect(locales).toContain('en');
			expect(locales).toContain('et');
			expect(locales).toHaveLength(2);
		});

		it('should default to English locale', () => {
			expect(getLocale()).toBe('en');
		});

		it('should switch to Estonian locale', () => {
			setLocale('et', { reload: false });
			expect(getLocale()).toBe('et');
		});
	});

	describe('Common Messages', () => {
		it('should have save message in English', () => {
			setLocale('en', { reload: false });
			expect(m.common_save()).toBe('Save');
		});

		it('should have save message in Estonian', () => {
			setLocale('et', { reload: false });
			expect(m.common_save()).toBe('Salvesta');
		});

		it('should have cancel message in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.common_cancel()).toBe('Cancel');

			setLocale('et', { reload: false });
			expect(m.common_cancel()).toBe('Tühista');
		});

		it('should have delete message in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.common_delete()).toBe('Delete');

			setLocale('et', { reload: false });
			expect(m.common_delete()).toBe('Kustuta');
		});

		it('should have loading message in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.common_loading()).toBe('Loading...');

			setLocale('et', { reload: false });
			expect(m.common_loading()).toBe('Laadimine...');
		});
	});

	describe('Navigation Messages', () => {
		it('should have dashboard in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.nav_dashboard()).toBe('Dashboard');

			setLocale('et', { reload: false });
			expect(m.nav_dashboard()).toBe('Töölaud');
		});

		it('should have invoices in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.nav_invoices()).toBe('Invoices');

			setLocale('et', { reload: false });
			expect(m.nav_invoices()).toBe('Arved');
		});

		it('should have contacts in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.nav_contacts()).toBe('Contacts');

			setLocale('et', { reload: false });
			expect(m.nav_contacts()).toBe('Kontaktid');
		});

		it('should have settings in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.nav_settings()).toBe('Settings');

			setLocale('et', { reload: false });
			expect(m.nav_settings()).toBe('Seaded');
		});
	});

	describe('Authentication Messages', () => {
		it('should have login in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.auth_login()).toBe('Sign In');

			setLocale('et', { reload: false });
			expect(m.auth_login()).toBe('Logi sisse');
		});

		it('should have welcome back in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.auth_welcomeBack()).toBe('Welcome Back');

			setLocale('et', { reload: false });
			expect(m.auth_welcomeBack()).toBe('Tere tulemast tagasi');
		});
	});

	describe('Onboarding Messages', () => {
		it('should have welcome message in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.onboarding_welcome()).toBe('Welcome to Open Accounting');

			setLocale('et', { reload: false });
			expect(m.onboarding_welcome()).toBe('Tere tulemast Open Accountingusse');
		});

		it('should have step titles in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.onboarding_step1Title()).toBe('Company');
			expect(m.onboarding_step2Title()).toBe('Branding');
			expect(m.onboarding_step3Title()).toBe('Contact');
			expect(m.onboarding_step4Title()).toBe('Done');

			setLocale('et', { reload: false });
			expect(m.onboarding_step1Title()).toBe('Ettevõte');
			expect(m.onboarding_step2Title()).toBe('Kujundus');
			expect(m.onboarding_step3Title()).toBe('Kontakt');
			expect(m.onboarding_step4Title()).toBe('Valmis');
		});

		it('should have navigation buttons in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.onboarding_continue()).toBe('Continue');
			expect(m.onboarding_back()).toBe('Back');
			expect(m.onboarding_skip()).toBe('Skip Setup');

			setLocale('et', { reload: false });
			expect(m.onboarding_continue()).toBe('Jätka');
			expect(m.onboarding_back()).toBe('Tagasi');
			expect(m.onboarding_skip()).toBe('Jäta seadistamine vahele');
		});
	});

	describe('Error Messages', () => {
		it('should have error messages in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.errors_loadFailed()).toBe('Failed to load data');
			expect(m.errors_saveFailed()).toBe('Failed to save changes');

			setLocale('et', { reload: false });
			expect(m.errors_loadFailed()).toBe('Andmete laadimine ebaõnnestus');
			expect(m.errors_saveFailed()).toBe('Muudatuste salvestamine ebaõnnestus');
		});
	});

	describe('Parameterized Messages', () => {
		it('should handle parameterized messages correctly', () => {
			setLocale('en', { reload: false });
			const result = m.bankingImport_importedTransactions({ count: '5' });
			expect(result).toContain('5');

			setLocale('et', { reload: false });
			const resultEt = m.bankingImport_importedTransactions({ count: '5' });
			expect(resultEt).toContain('5');
		});
	});
});

describe('i18n Message Completeness', () => {
	it('should have all required common messages', () => {
		const requiredMessages = [
			'common_save',
			'common_cancel',
			'common_delete',
			'common_edit',
			'common_create',
			'common_loading',
			'common_search',
			'common_actions',
			'common_status',
			'common_date',
			'common_amount',
			'common_name',
			'common_email',
			'common_phone',
			'common_address'
		];

		for (const msgKey of requiredMessages) {
			expect(m).toHaveProperty(msgKey);
			expect(typeof (m as Record<string, () => string>)[msgKey]).toBe('function');
		}
	});

	it('should have all required navigation messages', () => {
		const requiredMessages = [
			'nav_dashboard',
			'nav_accounts',
			'nav_journal',
			'nav_contacts',
			'nav_invoices',
			'nav_payments',
			'nav_reports',
			'nav_payroll',
			'nav_employees',
			'nav_settings',
			'nav_logout'
		];

		for (const msgKey of requiredMessages) {
			expect(m).toHaveProperty(msgKey);
			expect(typeof (m as Record<string, () => string>)[msgKey]).toBe('function');
		}
	});

	it('should have all required onboarding messages', () => {
		const requiredMessages = [
			'onboarding_welcome',
			'onboarding_subtitle',
			'onboarding_step1Title',
			'onboarding_step2Title',
			'onboarding_step3Title',
			'onboarding_step4Title',
			'onboarding_companyInfo',
			'onboarding_companyName',
			'onboarding_continue',
			'onboarding_back',
			'onboarding_skip',
			'onboarding_goToDashboard'
		];

		for (const msgKey of requiredMessages) {
			expect(m).toHaveProperty(msgKey);
			expect(typeof (m as Record<string, () => string>)[msgKey]).toBe('function');
		}
	});
});
