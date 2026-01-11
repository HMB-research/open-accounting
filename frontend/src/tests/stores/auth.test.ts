import { describe, it, expect, beforeEach, vi } from 'vitest';
import { get } from 'svelte/store';

// Mock browser environment
vi.mock('$app/environment', () => ({
	browser: true
}));

// Mock storage
const mockLocalStorage = new Map<string, string>();
const mockSessionStorage = new Map<string, string>();

const createStorageMock = (storage: Map<string, string>) => ({
	getItem: vi.fn((key: string) => storage.get(key) ?? null),
	setItem: vi.fn((key: string, value: string) => storage.set(key, value)),
	removeItem: vi.fn((key: string) => storage.delete(key)),
	clear: vi.fn(() => storage.clear())
});

vi.stubGlobal('localStorage', createStorageMock(mockLocalStorage));
vi.stubGlobal('sessionStorage', createStorageMock(mockSessionStorage));

describe('authStore', () => {
	beforeEach(() => {
		mockLocalStorage.clear();
		mockSessionStorage.clear();
		vi.resetModules();
	});

	describe('setTokens', () => {
		it('stores tokens in sessionStorage when rememberMe is false', async () => {
			const { authStore } = await import('$lib/stores/auth');

			authStore.setTokens('access-token', 'refresh-token', false);

			const state = get(authStore);
			expect(state.isAuthenticated).toBe(true);
			expect(state.accessToken).toBe('access-token');
			expect(state.refreshToken).toBe('refresh-token');
			expect(state.rememberMe).toBe(false);
		});

		it('stores tokens in localStorage when rememberMe is true', async () => {
			const { authStore } = await import('$lib/stores/auth');

			authStore.setTokens('access-token', 'refresh-token', true);

			const state = get(authStore);
			expect(state.isAuthenticated).toBe(true);
			expect(state.rememberMe).toBe(true);
		});

		it('clears tokens from alternate storage when setting new tokens', async () => {
			const { authStore } = await import('$lib/stores/auth');

			// First set with rememberMe false (sessionStorage)
			authStore.setTokens('session-access', 'session-refresh', false);

			// Then set with rememberMe true (localStorage)
			authStore.setTokens('local-access', 'local-refresh', true);

			// sessionStorage should be cleared
			expect(sessionStorage.removeItem).toHaveBeenCalled();
		});
	});

	describe('updateAccessToken', () => {
		it('updates access token while preserving other state', async () => {
			const { authStore } = await import('$lib/stores/auth');

			authStore.setTokens('old-access', 'refresh', false);
			authStore.updateAccessToken('new-access');

			const state = get(authStore);
			expect(state.accessToken).toBe('new-access');
			expect(state.refreshToken).toBe('refresh');
		});

		it('preserves rememberMe setting when updating token', async () => {
			const { authStore } = await import('$lib/stores/auth');

			authStore.setTokens('access', 'refresh', true);
			authStore.updateAccessToken('new-access');

			const state = get(authStore);
			expect(state.rememberMe).toBe(true);
		});
	});

	describe('clearTokens', () => {
		it('clears all tokens and resets authentication state', async () => {
			const { authStore } = await import('$lib/stores/auth');

			authStore.setTokens('access', 'refresh', true);
			authStore.clearTokens();

			const state = get(authStore);
			expect(state.isAuthenticated).toBe(false);
			expect(state.accessToken).toBe(null);
			expect(state.refreshToken).toBe(null);
		});

		it('clears tokens from both localStorage and sessionStorage', async () => {
			const { authStore } = await import('$lib/stores/auth');

			authStore.setTokens('access', 'refresh', true);
			authStore.clearTokens();

			expect(localStorage.removeItem).toHaveBeenCalledWith('access_token');
			expect(localStorage.removeItem).toHaveBeenCalledWith('refresh_token');
			expect(localStorage.removeItem).toHaveBeenCalledWith('remember_me');
			expect(sessionStorage.removeItem).toHaveBeenCalledWith('access_token');
			expect(sessionStorage.removeItem).toHaveBeenCalledWith('refresh_token');
		});

		it('resets rememberMe to false', async () => {
			const { authStore } = await import('$lib/stores/auth');

			authStore.setTokens('access', 'refresh', true);
			expect(get(authStore).rememberMe).toBe(true);

			authStore.clearTokens();
			expect(get(authStore).rememberMe).toBe(false);
		});
	});

	describe('getAccessToken', () => {
		it('returns current access token', async () => {
			const { authStore } = await import('$lib/stores/auth');

			authStore.setTokens('my-access-token', 'refresh', false);

			expect(authStore.getAccessToken()).toBe('my-access-token');
		});

		it('returns null when not authenticated', async () => {
			const { authStore } = await import('$lib/stores/auth');

			expect(authStore.getAccessToken()).toBe(null);
		});

		it('returns updated token after update', async () => {
			const { authStore } = await import('$lib/stores/auth');

			authStore.setTokens('initial', 'refresh', false);
			authStore.updateAccessToken('updated');

			expect(authStore.getAccessToken()).toBe('updated');
		});
	});

	describe('getRefreshToken', () => {
		it('returns current refresh token', async () => {
			const { authStore } = await import('$lib/stores/auth');

			authStore.setTokens('access', 'my-refresh-token', false);

			expect(authStore.getRefreshToken()).toBe('my-refresh-token');
		});

		it('returns null when not authenticated', async () => {
			const { authStore } = await import('$lib/stores/auth');

			expect(authStore.getRefreshToken()).toBe(null);
		});
	});

	describe('isRememberMe', () => {
		it('returns false by default', async () => {
			const { authStore } = await import('$lib/stores/auth');

			expect(authStore.isRememberMe()).toBe(false);
		});

		it('returns true when tokens set with rememberMe', async () => {
			const { authStore } = await import('$lib/stores/auth');

			authStore.setTokens('access', 'refresh', true);

			expect(authStore.isRememberMe()).toBe(true);
		});

		it('returns false after clearTokens', async () => {
			const { authStore } = await import('$lib/stores/auth');

			authStore.setTokens('access', 'refresh', true);
			authStore.clearTokens();

			expect(authStore.isRememberMe()).toBe(false);
		});
	});

	describe('isAuthenticated derived store', () => {
		it('reflects authentication status', async () => {
			const { authStore, isAuthenticated } = await import('$lib/stores/auth');

			expect(get(isAuthenticated)).toBe(false);

			authStore.setTokens('access', 'refresh', false);
			expect(get(isAuthenticated)).toBe(true);

			authStore.clearTokens();
			expect(get(isAuthenticated)).toBe(false);
		});
	});
});
