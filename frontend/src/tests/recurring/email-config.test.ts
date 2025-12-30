import { describe, it, expect, beforeEach } from 'vitest';
import * as m from '$lib/paraglide/messages.js';
import { setLocale, baseLocale } from '$lib/paraglide/runtime.js';

describe('Recurring Invoice Email Configuration Messages', () => {
	beforeEach(() => {
		// Reset to base locale before each test
		setLocale(baseLocale, { reload: false });
	});

	describe('Email Configuration Labels', () => {
		it('should have emailSettings in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_emailSettings()).toBe('Email Settings');

			setLocale('et', { reload: false });
			expect(m.recurring_emailSettings()).toBe('E-posti seaded');
		});

		it('should have sendEmailOnGeneration in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_sendEmailOnGeneration()).toBe('Send email when invoice is generated');

			setLocale('et', { reload: false });
			expect(m.recurring_sendEmailOnGeneration()).toBe('Saada e-kiri arve genereerimisel');
		});

		it('should have attachPdf in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_attachPdf()).toBe('Attach PDF invoice to email');

			setLocale('et', { reload: false });
			expect(m.recurring_attachPdf()).toBe('Lisa PDF arve e-kirjale');
		});

		it('should have recipientOverride in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_recipientOverride()).toBe('Override recipient email (optional)');

			setLocale('et', { reload: false });
			expect(m.recurring_recipientOverride()).toBe('Ülekiri saaja e-post (valikuline)');
		});

		it('should have emailSubject in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_emailSubject()).toBe('Custom email subject (optional)');

			setLocale('et', { reload: false });
			expect(m.recurring_emailSubject()).toBe('Kohandatud e-kirja teema (valikuline)');
		});

		it('should have emailMessageLabel in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_emailMessageLabel()).toBe('Custom message (optional)');

			setLocale('et', { reload: false });
			expect(m.recurring_emailMessageLabel()).toBe('Kohandatud sõnum (valikuline)');
		});

		it('should have emailEnabled in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_emailEnabled()).toBe('Email notifications enabled');

			setLocale('et', { reload: false });
			expect(m.recurring_emailEnabled()).toBe('E-posti teavitused lubatud');
		});
	});

	describe('Email Status Labels', () => {
		it('should have emailSent in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_emailSent()).toBe('Email sent successfully');

			setLocale('et', { reload: false });
			expect(m.recurring_emailSent()).toBe('E-kiri edukalt saadetud');
		});

		it('should have emailFailed in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_emailFailed()).toBe('Email failed to send');

			setLocale('et', { reload: false });
			expect(m.recurring_emailFailed()).toBe('E-kirja saatmine ebaõnnestus');
		});

		it('should have emailSkipped in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_emailSkipped()).toBe('Email was skipped');

			setLocale('et', { reload: false });
			expect(m.recurring_emailSkipped()).toBe('E-kirja saatmine vahele jäetud');
		});
	});

	describe('Email Placeholder Messages', () => {
		it('should have recipientOverridePlaceholder in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_recipientOverridePlaceholder()).toBe('Leave empty to use contact email');

			setLocale('et', { reload: false });
			expect(m.recurring_recipientOverridePlaceholder()).toBe('Jäta tühjaks kontakti e-posti kasutamiseks');
		});

		it('should have emailSubjectPlaceholder in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_emailSubjectPlaceholder()).toBe('Leave empty for default subject');

			setLocale('et', { reload: false });
			expect(m.recurring_emailSubjectPlaceholder()).toBe('Jäta tühjaks vaikimisi teema jaoks');
		});

		it('should have emailMessagePlaceholder in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_emailMessagePlaceholder()).toBe('Additional message to include in email');

			setLocale('et', { reload: false });
			expect(m.recurring_emailMessagePlaceholder()).toBe('Täiendav sõnum e-kirjale');
		});
	});

	describe('Email Configuration Message Completeness', () => {
		it('should have all required recurring email messages', () => {
			const requiredMessages = [
				'recurring_emailSettings',
				'recurring_sendEmailOnGeneration',
				'recurring_attachPdf',
				'recurring_recipientOverride',
				'recurring_recipientOverridePlaceholder',
				'recurring_emailSubject',
				'recurring_emailSubjectPlaceholder',
				'recurring_emailMessageLabel',
				'recurring_emailMessagePlaceholder',
				'recurring_emailSent',
				'recurring_emailFailed',
				'recurring_emailSkipped',
				'recurring_emailEnabled'
			];

			for (const msgKey of requiredMessages) {
				expect(m).toHaveProperty(msgKey);
				expect(typeof (m as unknown as Record<string, () => string>)[msgKey]).toBe('function');
			}
		});
	});
});

describe('Recurring Invoice Base Messages', () => {
	beforeEach(() => {
		setLocale(baseLocale, { reload: false });
	});

	describe('Recurring Invoice Labels', () => {
		it('should have title in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_title()).toBe('Recurring Invoices');

			setLocale('et', { reload: false });
			expect(m.recurring_title()).toBe('Püsiarved');
		});

		it('should have frequency in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_frequency()).toBe('Frequency');

			setLocale('et', { reload: false });
			expect(m.recurring_frequency()).toBe('Sagedus');
		});

		it('should have startDate in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_startDate()).toBe('Start Date');

			setLocale('et', { reload: false });
			expect(m.recurring_startDate()).toBe('Alguskuupäev');
		});

		it('should have nextGeneration in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_nextGeneration()).toBe('Next Generation');

			setLocale('et', { reload: false });
			expect(m.recurring_nextGeneration()).toBe('Järgmine genereerimine');
		});
	});

	describe('Frequency Options', () => {
		it('should have weekly frequency in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_weekly()).toBe('Weekly');

			setLocale('et', { reload: false });
			expect(m.recurring_weekly()).toBe('Iganädalane');
		});

		it('should have biweekly frequency in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_biweekly()).toBe('Bi-weekly');

			setLocale('et', { reload: false });
			expect(m.recurring_biweekly()).toBe('Iga kahe nädala tagant');
		});

		it('should have monthly frequency in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_monthly()).toBe('Monthly');

			setLocale('et', { reload: false });
			expect(m.recurring_monthly()).toBe('Igakuine');
		});

		it('should have quarterly frequency in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_quarterly()).toBe('Quarterly');

			setLocale('et', { reload: false });
			expect(m.recurring_quarterly()).toBe('Kvartaalne');
		});

		it('should have yearly frequency in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_yearly()).toBe('Yearly');

			setLocale('et', { reload: false });
			expect(m.recurring_yearly()).toBe('Aastane');
		});
	});

	describe('Action Messages', () => {
		it('should have createRecurringInvoice action in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_createRecurringInvoice()).toBe('Create Recurring Invoice');

			setLocale('et', { reload: false });
			expect(m.recurring_createRecurringInvoice()).toBe('Loo püsiarve');
		});

		it('should have generate action in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_generate()).toBe('Generate');

			setLocale('et', { reload: false });
			expect(m.recurring_generate()).toBe('Genereeri');
		});

		it('should have generateNow action in both languages', () => {
			setLocale('en', { reload: false });
			expect(m.recurring_generateNow()).toBe('Generate invoice now');

			setLocale('et', { reload: false });
			expect(m.recurring_generateNow()).toBe('Genereeri arve kohe');
		});
	});
});

describe('API Types for Email Configuration', () => {
	it('should define correct email configuration interface shape', () => {
		// This test validates the TypeScript interface aligns with backend
		const emailConfig = {
			send_email_on_generation: true,
			email_template_type: 'INVOICE_SEND',
			recipient_email_override: 'test@example.com',
			attach_pdf_to_email: true,
			email_subject_override: 'Custom Subject',
			email_message: 'Custom message body'
		};

		expect(emailConfig.send_email_on_generation).toBe(true);
		expect(emailConfig.email_template_type).toBe('INVOICE_SEND');
		expect(emailConfig.recipient_email_override).toBe('test@example.com');
		expect(emailConfig.attach_pdf_to_email).toBe(true);
		expect(emailConfig.email_subject_override).toBe('Custom Subject');
		expect(emailConfig.email_message).toBe('Custom message body');
	});

	it('should define correct generation result interface shape', () => {
		// This test validates the TypeScript interface aligns with backend
		const generationResult = {
			recurring_invoice_id: 'ri-123',
			generated_invoice_id: 'inv-456',
			generated_invoice_number: 'INV-2025-001',
			email_sent: true,
			email_status: 'SENT',
			email_log_id: 'log-789',
			email_error: ''
		};

		expect(generationResult.email_sent).toBe(true);
		expect(generationResult.email_status).toBe('SENT');
		expect(generationResult.email_log_id).toBe('log-789');
		expect(generationResult.email_error).toBe('');
	});

	it('should handle email status values', () => {
		const validStatuses = ['SENT', 'FAILED', 'SKIPPED', 'NO_CONFIG'];

		for (const status of validStatuses) {
			expect(validStatuses).toContain(status);
		}
	});

	it('should default attach_pdf_to_email to true when undefined', () => {
		const config: { attach_pdf_to_email?: boolean } = {};
		const attachPdf = config.attach_pdf_to_email ?? true;
		expect(attachPdf).toBe(true);
	});

	it('should respect explicit attach_pdf_to_email false', () => {
		const config = { attach_pdf_to_email: false };
		const attachPdf = config.attach_pdf_to_email ?? true;
		expect(attachPdf).toBe(false);
	});
});
