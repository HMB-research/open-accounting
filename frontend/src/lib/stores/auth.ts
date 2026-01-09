import { writable, derived, get } from 'svelte/store';
import { browser } from '$app/environment';

interface AuthState {
	isAuthenticated: boolean;
	accessToken: string | null;
	refreshToken: string | null;
	rememberMe: boolean;
}

const STORAGE_KEY_ACCESS = 'access_token';
const STORAGE_KEY_REFRESH = 'refresh_token';
const STORAGE_KEY_REMEMBER = 'remember_me';

function createAuthStore() {
	// Initialize state from storage
	const initialState: AuthState = {
		isAuthenticated: false,
		accessToken: null,
		refreshToken: null,
		rememberMe: false
	};

	if (browser) {
		// Check if user selected "remember me" previously
		const rememberMe = localStorage.getItem(STORAGE_KEY_REMEMBER) === 'true';
		initialState.rememberMe = rememberMe;

		// Load tokens from appropriate storage
		const storage = rememberMe ? localStorage : sessionStorage;
		const accessToken = storage.getItem(STORAGE_KEY_ACCESS);
		const refreshToken = storage.getItem(STORAGE_KEY_REFRESH);

		if (accessToken) {
			initialState.accessToken = accessToken;
			initialState.refreshToken = refreshToken;
			initialState.isAuthenticated = true;
		}
	}

	const { subscribe, set, update } = writable<AuthState>(initialState);

	return {
		subscribe,

		/**
		 * Set tokens after successful login
		 */
		setTokens(accessToken: string, refreshToken: string, rememberMe: boolean = false) {
			if (browser) {
				// Store remember me preference
				if (rememberMe) {
					localStorage.setItem(STORAGE_KEY_REMEMBER, 'true');
				} else {
					localStorage.removeItem(STORAGE_KEY_REMEMBER);
				}

				// Use localStorage for remember me, sessionStorage otherwise
				const storage = rememberMe ? localStorage : sessionStorage;

				// Clear tokens from the other storage
				const otherStorage = rememberMe ? sessionStorage : localStorage;
				otherStorage.removeItem(STORAGE_KEY_ACCESS);
				otherStorage.removeItem(STORAGE_KEY_REFRESH);

				// Store tokens in the appropriate storage
				storage.setItem(STORAGE_KEY_ACCESS, accessToken);
				storage.setItem(STORAGE_KEY_REFRESH, refreshToken);
			}

			set({
				isAuthenticated: true,
				accessToken,
				refreshToken,
				rememberMe
			});
		},

		/**
		 * Update access token after refresh
		 */
		updateAccessToken(accessToken: string) {
			update((state) => {
				if (browser) {
					const storage = state.rememberMe ? localStorage : sessionStorage;
					storage.setItem(STORAGE_KEY_ACCESS, accessToken);
				}
				return { ...state, accessToken };
			});
		},

		/**
		 * Clear all tokens and log out
		 */
		clearTokens() {
			if (browser) {
				localStorage.removeItem(STORAGE_KEY_ACCESS);
				localStorage.removeItem(STORAGE_KEY_REFRESH);
				localStorage.removeItem(STORAGE_KEY_REMEMBER);
				sessionStorage.removeItem(STORAGE_KEY_ACCESS);
				sessionStorage.removeItem(STORAGE_KEY_REFRESH);
			}

			set({
				isAuthenticated: false,
				accessToken: null,
				refreshToken: null,
				rememberMe: false
			});
		},

		/**
		 * Get current access token
		 */
		getAccessToken(): string | null {
			return get({ subscribe }).accessToken;
		},

		/**
		 * Get current refresh token
		 */
		getRefreshToken(): string | null {
			return get({ subscribe }).refreshToken;
		},

		/**
		 * Check if remember me is enabled
		 */
		isRememberMe(): boolean {
			return get({ subscribe }).rememberMe;
		}
	};
}

export const authStore = createAuthStore();

// Derived store for just the authentication status
export const isAuthenticated = derived(authStore, ($auth) => $auth.isAuthenticated);
