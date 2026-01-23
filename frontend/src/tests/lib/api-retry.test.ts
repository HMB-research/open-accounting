import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
	isRetryableError,
	calculateBackoffDelay,
	DEFAULT_RETRY_CONFIG,
	TEST_RETRY_CONFIG,
	type RetryConfig
} from '$lib/api';

describe('Retry Utility Functions', () => {
	describe('isRetryableError', () => {
		it('should return true for 500 Internal Server Error', () => {
			expect(isRetryableError(null, 500)).toBe(true);
		});

		it('should return true for 502 Bad Gateway', () => {
			expect(isRetryableError(null, 502)).toBe(true);
		});

		it('should return true for 503 Service Unavailable', () => {
			expect(isRetryableError(null, 503)).toBe(true);
		});

		it('should return true for 504 Gateway Timeout', () => {
			expect(isRetryableError(null, 504)).toBe(true);
		});

		it('should return true for 429 Too Many Requests', () => {
			expect(isRetryableError(null, 429)).toBe(true);
		});

		it('should return true for TypeError with "fetch" in message', () => {
			const error = new TypeError('Failed to fetch');
			expect(isRetryableError(error)).toBe(true);
		});

		it('should return false for 400 Bad Request', () => {
			expect(isRetryableError(null, 400)).toBe(false);
		});

		it('should return false for 401 Unauthorized', () => {
			expect(isRetryableError(null, 401)).toBe(false);
		});

		it('should return false for 403 Forbidden', () => {
			expect(isRetryableError(null, 403)).toBe(false);
		});

		it('should return false for 404 Not Found', () => {
			expect(isRetryableError(null, 404)).toBe(false);
		});

		it('should return false for generic Error', () => {
			const error = new Error('Something went wrong');
			expect(isRetryableError(error)).toBe(false);
		});

		it('should return false for TypeError without "fetch" in message', () => {
			const error = new TypeError('Cannot read property of undefined');
			expect(isRetryableError(error)).toBe(false);
		});
	});

	describe('calculateBackoffDelay', () => {
		const config: RetryConfig = {
			maxRetries: 3,
			baseDelayMs: 1000,
			maxDelayMs: 10000
		};

		it('should calculate base delay for first attempt', () => {
			// Mock Math.random to return 0 for predictable testing
			const mockRandom = vi.spyOn(Math, 'random').mockReturnValue(0);

			const delay = calculateBackoffDelay(0, config);
			expect(delay).toBe(1000); // base * 2^0 = 1000

			mockRandom.mockRestore();
		});

		it('should double delay for each subsequent attempt', () => {
			const mockRandom = vi.spyOn(Math, 'random').mockReturnValue(0);

			const delay0 = calculateBackoffDelay(0, config);
			const delay1 = calculateBackoffDelay(1, config);
			const delay2 = calculateBackoffDelay(2, config);

			expect(delay0).toBe(1000); // base * 2^0 = 1000
			expect(delay1).toBe(2000); // base * 2^1 = 2000
			expect(delay2).toBe(4000); // base * 2^2 = 4000

			mockRandom.mockRestore();
		});

		it('should not exceed maxDelayMs', () => {
			const mockRandom = vi.spyOn(Math, 'random').mockReturnValue(0);

			// Large attempt number that would exceed max
			const delay = calculateBackoffDelay(10, config); // Would be 1000 * 2^10 = 1024000

			expect(delay).toBeLessThanOrEqual(config.maxDelayMs);

			mockRandom.mockRestore();
		});

		it('should add jitter to delay', () => {
			// With random = 0.5, jitter should add 12.5% (half of 25%)
			const mockRandom = vi.spyOn(Math, 'random').mockReturnValue(0.5);

			const delay = calculateBackoffDelay(0, config);
			// base * 2^0 + (base * 0.25 * 0.5) = 1000 + 125 = 1125
			expect(delay).toBe(1125);

			mockRandom.mockRestore();
		});

		it('should add maximum jitter when random is 1', () => {
			const mockRandom = vi.spyOn(Math, 'random').mockReturnValue(1);

			const delay = calculateBackoffDelay(0, config);
			// base * 2^0 + (base * 0.25 * 1) = 1000 + 250 = 1250
			expect(delay).toBe(1250);

			mockRandom.mockRestore();
		});
	});

	describe('Retry Configurations', () => {
		it('should have sensible DEFAULT_RETRY_CONFIG values', () => {
			expect(DEFAULT_RETRY_CONFIG.maxRetries).toBe(3);
			expect(DEFAULT_RETRY_CONFIG.baseDelayMs).toBe(1000);
			expect(DEFAULT_RETRY_CONFIG.maxDelayMs).toBe(10000);
		});

		it('should have fast TEST_RETRY_CONFIG for testing', () => {
			expect(TEST_RETRY_CONFIG.maxRetries).toBe(3);
			expect(TEST_RETRY_CONFIG.baseDelayMs).toBe(10);
			expect(TEST_RETRY_CONFIG.maxDelayMs).toBe(50);
		});

		it('should have maxDelayMs >= baseDelayMs in all configs', () => {
			expect(DEFAULT_RETRY_CONFIG.maxDelayMs).toBeGreaterThanOrEqual(
				DEFAULT_RETRY_CONFIG.baseDelayMs
			);
			expect(TEST_RETRY_CONFIG.maxDelayMs).toBeGreaterThanOrEqual(TEST_RETRY_CONFIG.baseDelayMs);
		});
	});
});

describe('API Client Retry Integration', () => {
	let mockFetch: ReturnType<typeof vi.fn>;
	let originalFetch: typeof global.fetch;

	beforeEach(() => {
		mockFetch = vi.fn();
		originalFetch = global.fetch;
		global.fetch = mockFetch;
	});

	afterEach(() => {
		global.fetch = originalFetch;
		vi.clearAllMocks();
	});

	describe('Successful Retry Scenarios', () => {
		it('should retry on 500 and succeed on second attempt', async () => {
			const { api } = await import('$lib/api');

			// First call fails with 500, second succeeds
			mockFetch
				.mockResolvedValueOnce({
					ok: false,
					status: 500,
					json: async () => ({})
				})
				.mockResolvedValueOnce({
					ok: true,
					status: 200,
					json: async () => ({ data: 'success' })
				});

			api.setTokens('test-token', 'test-refresh');

			const result = await api.getMyTenants();

			expect(result).toEqual({ data: 'success' });
			expect(mockFetch).toHaveBeenCalledTimes(2);
		});

		it('should retry on network error and succeed on second attempt', async () => {
			const { api } = await import('$lib/api');

			mockFetch
				.mockRejectedValueOnce(new TypeError('Failed to fetch'))
				.mockResolvedValueOnce({
					ok: true,
					status: 200,
					json: async () => ({ recovered: true })
				});

			api.setTokens('test-token', 'test-refresh');

			const result = await api.getMyTenants();

			expect(result).toEqual({ recovered: true });
			expect(mockFetch).toHaveBeenCalledTimes(2);
		});

		it('should retry on 429 rate limit and succeed', async () => {
			const { api } = await import('$lib/api');

			mockFetch
				.mockResolvedValueOnce({
					ok: false,
					status: 429,
					json: async () => ({ error: 'Rate limited' })
				})
				.mockResolvedValueOnce({
					ok: true,
					status: 200,
					json: async () => ({ ok: true })
				});

			api.setTokens('test-token', 'test-refresh');

			const result = await api.getMyTenants();

			expect(result).toEqual({ ok: true });
			expect(mockFetch).toHaveBeenCalledTimes(2);
		});
	});

	describe('Non-Retryable Errors', () => {
		it('should NOT retry on 400 Bad Request', async () => {
			const { api } = await import('$lib/api');

			mockFetch.mockResolvedValueOnce({
				ok: false,
				status: 400,
				json: async () => ({ error: 'Bad request' })
			});

			api.setTokens('test-token', 'test-refresh');

			await expect(api.getMyTenants()).rejects.toThrow('Bad request');
			expect(mockFetch).toHaveBeenCalledTimes(1); // Only 1 call, no retry
		});

		it('should NOT retry on 404 Not Found', async () => {
			const { api } = await import('$lib/api');

			mockFetch.mockResolvedValueOnce({
				ok: false,
				status: 404,
				json: async () => ({ error: 'Not found' })
			});

			api.setTokens('test-token', 'test-refresh');

			await expect(api.getMyTenants()).rejects.toThrow('Not found');
			expect(mockFetch).toHaveBeenCalledTimes(1);
		});

		it('should NOT retry on 403 Forbidden', async () => {
			const { api } = await import('$lib/api');

			mockFetch.mockResolvedValueOnce({
				ok: false,
				status: 403,
				json: async () => ({ error: 'Forbidden' })
			});

			api.setTokens('test-token', 'test-refresh');

			await expect(api.getMyTenants()).rejects.toThrow('Forbidden');
			expect(mockFetch).toHaveBeenCalledTimes(1);
		});
	});

	describe('Retry Exhaustion', () => {
		it('should fail after exhausting all retries on 500', async () => {
			const { api } = await import('$lib/api');

			// All calls fail with 500
			const errorResponse = {
				ok: false,
				status: 500,
				json: async () => ({})
			};
			mockFetch
				.mockResolvedValueOnce(errorResponse)
				.mockResolvedValueOnce(errorResponse)
				.mockResolvedValueOnce(errorResponse)
				.mockResolvedValueOnce(errorResponse);

			api.setTokens('test-token', 'test-refresh');

			await expect(api.getMyTenants()).rejects.toThrow('Request failed');

			// 1 initial + 3 retries = 4 total attempts
			expect(mockFetch).toHaveBeenCalledTimes(4);
		}, 15000); // Increase timeout for retry delays

		it('should fail after exhausting all retries on network error', async () => {
			const { api } = await import('$lib/api');

			mockFetch
				.mockRejectedValueOnce(new TypeError('Failed to fetch'))
				.mockRejectedValueOnce(new TypeError('Failed to fetch'))
				.mockRejectedValueOnce(new TypeError('Failed to fetch'))
				.mockRejectedValueOnce(new TypeError('Failed to fetch'));

			api.setTokens('test-token', 'test-refresh');

			await expect(api.getMyTenants()).rejects.toThrow('Failed to fetch');
			expect(mockFetch).toHaveBeenCalledTimes(4);
		}, 15000);

		it('should succeed on the last retry attempt', async () => {
			const { api } = await import('$lib/api');

			const errorResponse = {
				ok: false,
				status: 500,
				json: async () => ({})
			};

			// First 3 fail, 4th (last retry) succeeds
			mockFetch
				.mockResolvedValueOnce(errorResponse)
				.mockResolvedValueOnce(errorResponse)
				.mockResolvedValueOnce(errorResponse)
				.mockResolvedValueOnce({
					ok: true,
					status: 200,
					json: async () => ({ finally: 'success' })
				});

			api.setTokens('test-token', 'test-refresh');

			const result = await api.getMyTenants();

			expect(result).toEqual({ finally: 'success' });
			expect(mockFetch).toHaveBeenCalledTimes(4);
		}, 15000);
	});
});
