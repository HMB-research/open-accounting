import type { Page } from '@sveltejs/kit';
import * as m from '$lib/paraglide/messages.js';

/**
 * Result of tenant validation
 */
export interface TenantValidationResult {
	tenantId: string | null;
	valid: boolean;
	error?: string;
}

/**
 * Get tenant ID from page URL with validation
 * Returns tenantId if valid, null with error message if invalid
 */
export function getTenantId(page: Page): TenantValidationResult {
	const tenantId = page.url.searchParams.get('tenant');

	if (!tenantId) {
		return {
			tenantId: null,
			valid: false,
			error: m.errors_noOrganizationSelected()
		};
	}

	return {
		tenantId,
		valid: true
	};
}

/**
 * Require tenant ID - throws error if not present
 * Use this in action handlers where tenant is required
 */
export function requireTenantId(
	page: Page,
	onError?: (error: string) => void
): string | null {
	const result = getTenantId(page);

	if (!result.valid) {
		if (onError && result.error) {
			onError(result.error);
		}
		return null;
	}

	return result.tenantId;
}

/**
 * Parse API error response into user-friendly message
 */
export function parseApiError(err: unknown): string {
	if (err instanceof Error) {
		const message = err.message.toLowerCase();

		// Handle specific error types
		if (message.includes('access denied') || message.includes('forbidden')) {
			return m.errors_accessDenied();
		}
		if (message.includes('unauthorized') || message.includes('401')) {
			return m.errors_unauthorized();
		}
		if (message.includes('not found') || message.includes('404')) {
			return m.errors_notFound();
		}
		if (message.includes('network') || message.includes('fetch')) {
			return m.errors_networkError();
		}

		// Return the original message if it's informative
		return err.message;
	}

	return m.errors_loadFailed();
}

/**
 * Create a standard action handler wrapper that handles tenant validation and errors
 */
export function createActionHandler<T extends unknown[]>(
	page: Page,
	action: (tenantId: string, ...args: T) => Promise<void>,
	options: {
		onError: (error: string) => void;
		onLoading?: (loading: boolean) => void;
		onSuccess?: (message: string) => void;
		successMessage?: string;
	}
) {
	return async (...args: T) => {
		const tenantId = requireTenantId(page, options.onError);
		if (!tenantId) return;

		options.onLoading?.(true);
		try {
			await action(tenantId, ...args);
			if (options.successMessage && options.onSuccess) {
				options.onSuccess(options.successMessage);
			}
		} catch (err) {
			options.onError(parseApiError(err));
		} finally {
			options.onLoading?.(false);
		}
	};
}
