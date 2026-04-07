import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import WorkflowHero from '$lib/components/WorkflowHero.svelte';

describe('WorkflowHero', () => {
	afterEach(() => {
		cleanup();
	});

	it('renders hero content, stats, and invokes button actions', async () => {
		const onPrimary = vi.fn();

		render(WorkflowHero, {
			eyebrow: 'Operations',
			title: 'Invoices',
			description: 'Review billing activity and send documents from one place.',
			backHref: '/dashboard',
			backLabel: 'Back to dashboard',
			badgeLabel: 'Open period',
			stats: [
				{ label: 'Total invoices', value: '12' },
				{ label: 'Overdue', value: '3', tone: 'warning' }
			],
			actions: [
				{ label: 'Import invoices', variant: 'secondary' },
				{ label: 'New invoice', onclick: onPrimary }
			],
			aside: {
				kicker: 'Best next step',
				title: 'Import your legacy invoices first',
				body: 'That keeps numbering and contact matches consistent before you bill live customers.',
				items: ['Use one line per invoice row', 'Review overdue balances after import'],
				linkLabel: 'Open contacts',
				href: '/contacts'
			}
		});

		expect(screen.getByRole('heading', { name: 'Invoices' })).toBeInTheDocument();
		expect(screen.getByText('Open period')).toBeInTheDocument();
		expect(screen.getByText('Total invoices')).toBeInTheDocument();
		expect(screen.getByText('Import your legacy invoices first')).toBeInTheDocument();
		expect(screen.getByRole('link', { name: /Back to dashboard/i })).toHaveAttribute('href', '/dashboard');
		expect(screen.getByRole('link', { name: 'Open contacts' })).toHaveAttribute('href', '/contacts');

		await fireEvent.click(screen.getByRole('button', { name: 'New invoice' }));
		expect(onPrimary).toHaveBeenCalledTimes(1);
	});
});
