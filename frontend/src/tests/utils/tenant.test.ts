import { describe, it, expect, vi } from 'vitest';
import type { Page } from '@sveltejs/kit';

// Mock paraglide messages
vi.mock('$lib/paraglide/messages.js', () => ({
	errors_noOrganizationSelected: () => 'No organization selected',
	errors_accessDenied: () => 'Access denied',
	errors_unauthorized: () => 'Unauthorized',
	errors_notFound: () => 'Not found',
	errors_networkError: () => 'Network error',
	errors_loadFailed: () => 'Failed to load'
}));

import { getTenantId, requireTenantId, parseApiError, createActionHandler } from '$lib/utils/tenant';

// Helper to create mock page object
function createMockPage(tenantId: string | null): Page {
	const searchParams = new URLSearchParams();
	if (tenantId) {
		searchParams.set('tenant', tenantId);
	}
	return {
		url: {
			searchParams
		}
	} as unknown as Page;
}

describe('Tenant Utilities', () => {
	describe('getTenantId', () => {
		it('returns valid result when tenant is present', () => {
			const page = createMockPage('tenant-123');
			const result = getTenantId(page);

			expect(result.valid).toBe(true);
			expect(result.tenantId).toBe('tenant-123');
			expect(result.error).toBeUndefined();
		});

		it('returns invalid result when tenant is missing', () => {
			const page = createMockPage(null);
			const result = getTenantId(page);

			expect(result.valid).toBe(false);
			expect(result.tenantId).toBe(null);
			expect(result.error).toBe('No organization selected');
		});

		it('handles empty string tenant', () => {
			const page = createMockPage('');
			const result = getTenantId(page);

			// Empty string is falsy, so should be invalid
			expect(result.valid).toBe(false);
			expect(result.tenantId).toBe(null);
		});

		it('handles UUID format tenant ID', () => {
			const uuid = 'b0000000-0000-0000-0001-000000000001';
			const page = createMockPage(uuid);
			const result = getTenantId(page);

			expect(result.valid).toBe(true);
			expect(result.tenantId).toBe(uuid);
		});
	});

	describe('requireTenantId', () => {
		it('returns tenant ID when present', () => {
			const page = createMockPage('tenant-456');
			const result = requireTenantId(page);

			expect(result).toBe('tenant-456');
		});

		it('returns null and calls error callback when missing', () => {
			const page = createMockPage(null);
			const onError = vi.fn();

			const result = requireTenantId(page, onError);

			expect(result).toBe(null);
			expect(onError).toHaveBeenCalledWith('No organization selected');
		});

		it('returns null without callback when missing', () => {
			const page = createMockPage(null);

			const result = requireTenantId(page);

			expect(result).toBe(null);
		});

		it('does not call error callback when tenant is present', () => {
			const page = createMockPage('tenant-789');
			const onError = vi.fn();

			const result = requireTenantId(page, onError);

			expect(result).toBe('tenant-789');
			expect(onError).not.toHaveBeenCalled();
		});
	});

	describe('parseApiError', () => {
		it('returns access denied for forbidden errors', () => {
			const error = new Error('Access denied to resource');
			expect(parseApiError(error)).toBe('Access denied');
		});

		it('returns access denied for forbidden keyword', () => {
			const error = new Error('Forbidden action');
			expect(parseApiError(error)).toBe('Access denied');
		});

		it('returns unauthorized for unauthorized errors', () => {
			const error = new Error('Unauthorized access');
			expect(parseApiError(error)).toBe('Unauthorized');
		});

		it('returns unauthorized for 401 status', () => {
			const error = new Error('Error 401: Not authenticated');
			expect(parseApiError(error)).toBe('Unauthorized');
		});

		it('returns not found for 404 errors', () => {
			const error = new Error('Resource not found');
			expect(parseApiError(error)).toBe('Not found');
		});

		it('returns not found for 404 status', () => {
			const error = new Error('404 page not found');
			expect(parseApiError(error)).toBe('Not found');
		});

		it('returns network error for fetch failures', () => {
			const error = new Error('Failed to fetch');
			expect(parseApiError(error)).toBe('Network error');
		});

		it('returns network error for network keyword', () => {
			const error = new Error('Network connection failed');
			expect(parseApiError(error)).toBe('Network error');
		});

		it('returns original message for unknown errors', () => {
			const error = new Error('Something specific went wrong');
			expect(parseApiError(error)).toBe('Something specific went wrong');
		});

		it('returns generic message for non-Error objects', () => {
			expect(parseApiError('string error')).toBe('Failed to load');
			expect(parseApiError(null)).toBe('Failed to load');
			expect(parseApiError(undefined)).toBe('Failed to load');
			expect(parseApiError(123)).toBe('Failed to load');
			expect(parseApiError({ message: 'object' })).toBe('Failed to load');
		});

		it('handles case insensitivity', () => {
			expect(parseApiError(new Error('ACCESS DENIED'))).toBe('Access denied');
			expect(parseApiError(new Error('UNAUTHORIZED'))).toBe('Unauthorized');
			expect(parseApiError(new Error('NOT FOUND'))).toBe('Not found');
		});
	});

	describe('createActionHandler', () => {
		it('calls action with tenantId when valid', async () => {
			const page = createMockPage('tenant-action');
			const action = vi.fn().mockResolvedValue(undefined);
			const onError = vi.fn();

			const handler = createActionHandler(page, action, { onError });
			await handler('arg1', 'arg2');

			expect(action).toHaveBeenCalledWith('tenant-action', 'arg1', 'arg2');
			expect(onError).not.toHaveBeenCalled();
		});

		it('calls onError when tenant is missing', async () => {
			const page = createMockPage(null);
			const action = vi.fn().mockResolvedValue(undefined);
			const onError = vi.fn();

			const handler = createActionHandler(page, action, { onError });
			await handler();

			expect(action).not.toHaveBeenCalled();
			expect(onError).toHaveBeenCalledWith('No organization selected');
		});

		it('calls onLoading correctly', async () => {
			const page = createMockPage('tenant-loading');
			const action = vi.fn().mockResolvedValue(undefined);
			const onError = vi.fn();
			const onLoading = vi.fn();

			const handler = createActionHandler(page, action, { onError, onLoading });
			await handler();

			expect(onLoading).toHaveBeenCalledWith(true);
			expect(onLoading).toHaveBeenCalledWith(false);
			expect(onLoading.mock.calls[0][0]).toBe(true);
			expect(onLoading.mock.calls[1][0]).toBe(false);
		});

		it('calls onSuccess with message when provided', async () => {
			const page = createMockPage('tenant-success');
			const action = vi.fn().mockResolvedValue(undefined);
			const onError = vi.fn();
			const onSuccess = vi.fn();

			const handler = createActionHandler(page, action, {
				onError,
				onSuccess,
				successMessage: 'Action completed!'
			});
			await handler();

			expect(onSuccess).toHaveBeenCalledWith('Action completed!');
		});

		it('does not call onSuccess when no message provided', async () => {
			const page = createMockPage('tenant-no-message');
			const action = vi.fn().mockResolvedValue(undefined);
			const onError = vi.fn();
			const onSuccess = vi.fn();

			const handler = createActionHandler(page, action, { onError, onSuccess });
			await handler();

			expect(onSuccess).not.toHaveBeenCalled();
		});

		it('calls onError when action throws', async () => {
			const page = createMockPage('tenant-error');
			const action = vi.fn().mockRejectedValue(new Error('Action failed'));
			const onError = vi.fn();

			const handler = createActionHandler(page, action, { onError });
			await handler();

			expect(onError).toHaveBeenCalledWith('Action failed');
		});

		it('sets loading to false even when action throws', async () => {
			const page = createMockPage('tenant-finally');
			const action = vi.fn().mockRejectedValue(new Error('Action failed'));
			const onError = vi.fn();
			const onLoading = vi.fn();

			const handler = createActionHandler(page, action, { onError, onLoading });
			await handler();

			expect(onLoading).toHaveBeenCalledWith(false);
		});

		it('parses API errors correctly', async () => {
			const page = createMockPage('tenant-api-error');
			const action = vi.fn().mockRejectedValue(new Error('Access denied'));
			const onError = vi.fn();

			const handler = createActionHandler(page, action, { onError });
			await handler();

			expect(onError).toHaveBeenCalledWith('Access denied');
		});
	});
});
