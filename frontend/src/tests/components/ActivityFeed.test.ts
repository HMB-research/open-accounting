import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/svelte';
import { baseLocale, setLocale } from '$lib/paraglide/runtime.js';
import ActivityFeed, { type ActivityItem } from '$lib/components/ActivityFeed.svelte';

function activity(overrides: Partial<ActivityItem> = {}): ActivityItem {
	return {
		id: 'activity-1',
		type: 'INVOICE',
		action: 'created',
		description: 'Invoice INV-1001 created',
		created_at: '2026-06-10T11:30:00Z',
		...overrides
	};
}

describe('ActivityFeed', () => {
	beforeEach(() => {
		setLocale(baseLocale, { reload: false });
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2026-06-10T12:00:00Z'));
	});

	afterEach(() => {
		cleanup();
		vi.useRealTimers();
	});

	it('shows the loading state without rendering stale activities', () => {
		const { container } = render(ActivityFeed, {
			items: [activity()],
			loading: true
		});

		expect(screen.getByTestId('activity-feed')).toBeInTheDocument();
		expect(container.querySelector('.spinner')).toBeInTheDocument();
		expect(screen.queryByText('Invoice INV-1001 created')).not.toBeInTheDocument();
	});

	it('shows an empty state when there is no recent activity', () => {
		render(ActivityFeed, { items: [] });

		expect(screen.getByText('Recent Activity')).toBeInTheDocument();
		expect(screen.getByText('No recent activity')).toBeInTheDocument();
		expect(screen.queryAllByTestId('activity-item')).toHaveLength(0);
	});

	it('renders activity descriptions, relative times, and EUR amounts', () => {
		render(ActivityFeed, {
			items: [
				activity({
					id: 'invoice',
					description: 'Invoice INV-1001 sent',
					amount: '1234.56',
					created_at: '2026-06-10T11:30:00Z'
				}),
				activity({
					id: 'contact',
					type: 'CONTACT',
					description: 'Baltic Commerce updated',
					created_at: '2026-06-08T12:00:00Z'
				})
			]
		});

		expect(screen.getAllByTestId('activity-item')).toHaveLength(2);
		expect(screen.getByText('Invoice INV-1001 sent')).toBeInTheDocument();
		expect(screen.getByText('Baltic Commerce updated')).toBeInTheDocument();
		expect(screen.getByText('30m ago')).toBeInTheDocument();
		expect(screen.getByText('2d ago')).toBeInTheDocument();
		expect(screen.getByText(/1[\s\u00a0]?234,56\s?€/)).toBeInTheDocument();
	});
});
