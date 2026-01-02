/**
 * Mock for $env/dynamic/public
 *
 * This mock provides a default PUBLIC_API_URL for testing.
 * Tests can override this by mocking the module.
 */
export const env = {
	PUBLIC_API_URL: 'http://localhost:8080'
};
