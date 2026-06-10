import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@testing-library/svelte';
import StatusBadge from '$lib/components/StatusBadge.svelte';

const statusConfig = {
	PAID: { class: 'badge-paid', label: 'Paid' },
	OVERDUE: { class: 'badge-overdue', label: 'Overdue' }
};

describe('StatusBadge', () => {
	afterEach(() => {
		cleanup();
	});

	it('renders the configured label and styling class', () => {
		render(StatusBadge, {
			status: 'PAID',
			config: statusConfig
		});

		const badge = screen.getByText('Paid');
		expect(badge).toBeInTheDocument();
		expect(badge).toHaveClass('badge', 'badge-md', 'badge-paid');
	});

	it('falls back to the raw status when no config is available', () => {
		render(StatusBadge, {
			status: 'ARCHIVED',
			config: statusConfig
		});

		const badge = screen.getByText('ARCHIVED');
		expect(badge).toBeInTheDocument();
		expect(badge).toHaveClass('badge-default');
	});

	it('applies the requested badge size', () => {
		render(StatusBadge, {
			status: 'OVERDUE',
			config: statusConfig,
			size: 'lg'
		});

		expect(screen.getByText('Overdue')).toHaveClass('badge-lg', 'badge-overdue');
	});
});
