import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/svelte';

const { apiMock } = vi.hoisted(() => ({
	apiMock: {
		getMyTenants: vi.fn()
	}
}));

vi.mock('$lib/api', async () => {
	const actual = await vi.importActual<typeof import('$lib/api')>('$lib/api');
	return { ...actual, api: apiMock };
});

import DashboardPage from '../../routes/dashboard/+page.svelte';

describe('dashboard zero-tenant onboarding', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		apiMock.getMyTenants.mockResolvedValue([]);
	});

	afterEach(() => cleanup());

	it('offers the explicit SmartAccounts catalog path beside manual organization creation', async () => {
		render(DashboardPage);

		expect(await screen.findByRole('button', { name: /create organization/i })).toBeInTheDocument();
		const importLink = screen.getByRole('link', { name: 'Import companies from SmartAccounts' });
		expect(importLink).toHaveAttribute('href', '/migration');
	});
});
